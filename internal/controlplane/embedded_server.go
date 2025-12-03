package controlplane

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/api"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// EmbeddedControlPlane wraps the existing control plane server
// to provide embedded functionality within the pipeline runner
type EmbeddedControlPlane struct {
	server          *grpc.Server
	controlPlane    *api.ControlPlaneServer
	listener        net.Listener
	address         string
	port            int
	started         bool
	stopped         bool
	mu              sync.RWMutex
	config          Config
}

// Config holds configuration for the embedded control plane
type Config struct {
	Address         string
	Port            int
	HeartbeatTTL    time.Duration
	JanitorInterval time.Duration
	Storage         storage.ServiceStorage
	ServerOptions   []grpc.ServerOption
}

// NewEmbeddedControlPlane creates a new embedded control plane instance
func NewEmbeddedControlPlane(config Config) *EmbeddedControlPlane {
	return &EmbeddedControlPlane{
		address: config.Address,
		port:    config.Port,
		config:  config,
	}
}

// Start begins the embedded control plane server
func (e *EmbeddedControlPlane) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("control plane already started")
	}

	// Create listener
	address := fmt.Sprintf("%s:%d", e.address, e.port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	e.listener = listener

	// Create gRPC server with provided options
	serverOptions := e.config.ServerOptions
	if serverOptions == nil {
		serverOptions = []grpc.ServerOption{}
	}
	e.server = grpc.NewServer(serverOptions...)

	// Create control plane server
	e.controlPlane = api.NewControlPlaneServer(e.config.Storage)

	// Configure heartbeat TTL and janitor interval if provided
	if e.config.HeartbeatTTL > 0 {
		e.controlPlane.SetHeartbeatTTL(e.config.HeartbeatTTL)
	}
	if e.config.JanitorInterval > 0 {
		e.controlPlane.SetJanitorInterval(e.config.JanitorInterval)
	}

	// Start the control plane (initializes storage and starts janitor)
	if err := e.controlPlane.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Register the control plane service
	flowctlv1.RegisterControlPlaneServiceServer(e.server, e.controlPlane)

	// Start server in background
	go func() {
		logger.Info("Embedded control plane listening", zap.String("address", address))
		if err := e.server.Serve(listener); err != nil {
			if !e.stopped {
				logger.Error("Control plane server error", zap.Error(err))
			}
		}
	}()

	e.started = true
	logger.Info("Embedded control plane started", zap.String("address", address))

	return nil
}

// Stop gracefully shuts down the embedded control plane
func (e *EmbeddedControlPlane) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started || e.stopped {
		return nil
	}

	logger.Info("Stopping embedded control plane")

	e.stopped = true

	// Graceful shutdown of gRPC server
	if e.server != nil {
		e.server.GracefulStop()
	}

	// Close control plane
	if e.controlPlane != nil {
		if err := e.controlPlane.Close(); err != nil {
			logger.Error("Error closing control plane", zap.Error(err))
		}
	}

	logger.Info("Embedded control plane stopped")
	return nil
}

// GetEndpoint returns the control plane endpoint URL
func (e *EmbeddedControlPlane) GetEndpoint() string {
	return fmt.Sprintf("%s:%d", e.address, e.port)
}

// GetServiceList retrieves the list of registered services
func (e *EmbeddedControlPlane) GetServiceList() ([]*flowctlv1.ComponentStatusResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.started || e.stopped {
		return nil, fmt.Errorf("control plane not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := e.controlPlane.ListComponents(ctx, &flowctlv1.ListComponentsRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Components, nil
}

// WaitForComponent waits for a specific component to register with the control plane
func (e *EmbeddedControlPlane) WaitForComponent(componentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		services, err := e.GetServiceList()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, service := range services {
			// FIXED: Check component ID from ComponentInfo
			if service.Component != nil && service.Component.Id == componentID {
				if service.Status == flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
					logger.Debug("Found registered component",
						zap.String("component_id", service.Component.Id),
						zap.String("status", service.Status.String()))
					return nil
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	// Better error message: show what we actually found
	registeredComponents := []string{}
	services, _ := e.GetServiceList()
	for _, service := range services {
		if service.Component != nil {
			registeredComponents = append(registeredComponents,
				fmt.Sprintf("%s (status=%v)", service.Component.Id, service.Status.String()))
		}
	}

	return fmt.Errorf("timeout waiting for component %s to register (found: %v)",
		componentID, registeredComponents)
}

// IsStarted returns true if the control plane is started
func (e *EmbeddedControlPlane) IsStarted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.started && !e.stopped
}

// GetPort returns the port the control plane is listening on
func (e *EmbeddedControlPlane) GetPort() int {
	return e.port
}

// GetAddress returns the address the control plane is listening on
func (e *EmbeddedControlPlane) GetAddress() string {
	return e.address
}