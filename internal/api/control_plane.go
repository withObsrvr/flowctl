package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/withobsrvr/flowctl/proto"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Service represents a registered service
type Service struct {
	Info     *pb.ServiceInfo
	Status   *pb.ServiceStatus
	LastSeen time.Time
	Conn     *grpc.ClientConn
}

// ControlPlaneServer implements the control plane gRPC service
type ControlPlaneServer struct {
	pb.UnimplementedControlPlaneServer
	mu               sync.RWMutex
	services         map[string]*Service // In-memory cache of services
	storage          storage.ServiceStorage // Persistent storage
	heartbeatTTL     time.Duration
	janitorInterval  time.Duration
	done             chan struct{}
	janitorStarted   bool
	janitorWaitGroup sync.WaitGroup
}

// NewControlPlaneServer creates a new control plane server with default settings.
// The server will use default values for heartbeat TTL (30s) and janitor interval (10s).
// To customize these values, use SetHeartbeatTTL and SetJanitorInterval before
// calling Start().
//
// If a nil storage is provided, the service registry will be in-memory only and not
// persist data between restarts.
func NewControlPlaneServer(storage storage.ServiceStorage) *ControlPlaneServer {
	return &ControlPlaneServer{
		services:        make(map[string]*Service),
		storage:         storage,
		heartbeatTTL:    30 * time.Second, // Default TTL is 30 seconds
		janitorInterval: 10 * time.Second, // Default janitor interval is 10 seconds
		done:            make(chan struct{}),
		janitorStarted:  false,
	}
}

// NewInMemoryControlPlaneServer creates a new control plane server with in-memory storage.
// This is equivalent to NewControlPlaneServer(nil) and is provided for backward compatibility.
func NewInMemoryControlPlaneServer() *ControlPlaneServer {
	return NewControlPlaneServer(nil)
}

// SetHeartbeatTTL sets the TTL for service heartbeats.
// This is the duration after which a service without heartbeats will be marked as unhealthy.
// This method should be called before Start() to take effect for the initial janitor.
func (s *ControlPlaneServer) SetHeartbeatTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeatTTL = ttl
	logger.Info("Heartbeat TTL updated", zap.Duration("ttl", ttl))
}

// SetJanitorInterval sets the interval at which the janitor checks for unhealthy services.
// This method should be called before Start() to take effect for the initial janitor.
func (s *ControlPlaneServer) SetJanitorInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.janitorInterval = interval
	logger.Info("Janitor interval updated", zap.Duration("interval", interval))
}

// Start begins monitoring service health by launching the janitor goroutine.
// This method also initializes the storage (if provided) and loads any persisted
// service information.
//
// This should be called after configuring the server with SetHeartbeatTTL and
// SetJanitorInterval if non-default values are desired.
func (s *ControlPlaneServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Initialize storage if provided
	if s.storage != nil {
		logger.Info("Initializing persistent storage")
		if err := s.storage.Open(); err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		
		// Load persisted services
		if err := s.loadServices(); err != nil {
			return fmt.Errorf("failed to load services from storage: %w", err)
		}
	} else {
		logger.Info("Using in-memory service registry (no persistence)")
	}
	
	if !s.janitorStarted {
		s.janitorStarted = true
		s.janitorWaitGroup.Add(1)
		go s.runJanitor()
		logger.Info("Health check janitor started", 
			zap.Duration("heartbeat_ttl", s.heartbeatTTL), 
			zap.Duration("janitor_interval", s.janitorInterval))
	}
	
	return nil
}

// loadServices loads persisted services from storage
func (s *ControlPlaneServer) loadServices() error {
	if s.storage == nil {
		return nil
	}
	
	ctx := context.Background()
	services, err := s.storage.ListServices(ctx)
	if err != nil {
		return err
	}
	
	count := 0
	for _, storedService := range services {
		service := &Service{
			Info:     storedService.Info,
			Status:   storedService.Status,
			LastSeen: storedService.LastSeen,
		}
		s.services[service.Info.ServiceId] = service
		count++
	}
	
	logger.Info("Loaded services from persistent storage", zap.Int("count", count))
	return nil
}

// Close stops the control plane server and cleans up resources.
// This method will stop the janitor goroutine, close the storage, and wait for
// all background tasks to complete.
func (s *ControlPlaneServer) Close() error {
	s.mu.Lock()
	if s.janitorStarted {
		close(s.done)
		s.janitorStarted = false
	}
	s.mu.Unlock()
	
	// Wait for janitor to complete
	s.janitorWaitGroup.Wait()
	
	// Close storage if available
	if s.storage != nil {
		logger.Info("Closing persistent storage")
		if err := s.storage.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}
	
	logger.Info("Control plane server closed")
	return nil
}

// Register implements the Register RPC
func (s *ControlPlaneServer) Register(ctx context.Context, info *pb.ServiceInfo) (*pb.RegistrationAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a unique service ID if not provided
	if info.ServiceId == "" {
		info.ServiceId = fmt.Sprintf("%s-%d", info.ServiceType, time.Now().UnixNano())
	}

	// Create service record
	now := time.Now()
	service := &Service{
		Info: info,
		Status: &pb.ServiceStatus{
			ServiceId:     info.ServiceId,
			ServiceType:   info.ServiceType,
			IsHealthy:     true,
			LastHeartbeat: timestamppb.New(now),
			ComponentId:   info.ComponentId, // Copy component_id to status
		},
		LastSeen: now,
	}

	// Store service in memory
	s.services[info.ServiceId] = service

	// Persist to storage if available
	if s.storage != nil {
		// Convert to storage model
		serviceInfo := &storage.ServiceInfo{
			Info:     info,
			Status:   service.Status,
			LastSeen: now,
		}
		
		if err := s.storage.RegisterService(ctx, serviceInfo); err != nil {
			logger.Error("Failed to persist service registration",
				zap.String("service_id", info.ServiceId),
				zap.Error(err))
			// We continue anyway as the service is registered in memory
		}
	}

	// Log warning if component_id is missing (backward compatibility)
	if info.ComponentId == "" {
		logger.Warn("Service registered without component_id (legacy mode)",
			zap.String("service_id", info.ServiceId))
	}

	// Log service registration
	logger.Info("Service registered",
		zap.String("service_id", info.ServiceId),
		zap.String("component_id", info.ComponentId),
		zap.String("service_type", info.ServiceType.String()),
		zap.String("health_endpoint", info.HealthEndpoint),
		zap.Strings("input_event_types", info.InputEventTypes),
		zap.Strings("output_event_types", info.OutputEventTypes),
		zap.Int32("max_inflight", info.MaxInflight),
	)

	// Generate topic names based on service type and event types
	var topicNames []string
	switch info.ServiceType {
	case pb.ServiceType_SERVICE_TYPE_SOURCE:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	case pb.ServiceType_SERVICE_TYPE_PROCESSOR:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	case pb.ServiceType_SERVICE_TYPE_PIPELINE:
		// Pipelines are composite services that manage their own internal communication
		// They don't need external topics since all components run within the pipeline
		logger.Info("Registered pipeline service",
			zap.String("pipeline_id", info.ServiceId),
			zap.Any("metadata", info.Metadata),
		)
	}

	// Return registration acknowledgment
	return &pb.RegistrationAck{
		ServiceId:  info.ServiceId,
		TopicNames: topicNames,
		ConnectionInfo: map[string]string{
			"control_plane_endpoint": "localhost:8080",
		},
	}, nil
}

// Heartbeat implements the Heartbeat RPC
func (s *ControlPlaneServer) Heartbeat(ctx context.Context, hb *pb.ServiceHeartbeat) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[hb.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", hb.ServiceId)
	}

	now := time.Now()
	service.LastSeen = now
	service.Status.Metrics = hb.Metrics
	service.Status.IsHealthy = true
	service.Status.LastHeartbeat = timestamppb.New(now)

	// Update in persistent storage if available
	if s.storage != nil {
		err := s.storage.UpdateService(ctx, hb.ServiceId, func(storedService *storage.ServiceInfo) error {
			storedService.LastSeen = now
			storedService.Status.Metrics = hb.Metrics
			storedService.Status.IsHealthy = true
			storedService.Status.LastHeartbeat = timestamppb.New(now)
			return nil
		})
		
		if err != nil && !storage.IsNotFound(err) {
			logger.Error("Failed to update service heartbeat in storage",
				zap.String("service_id", hb.ServiceId),
				zap.Error(err))
			// Continue anyway as the heartbeat is processed in memory
		}
	}

	// Log heartbeat receipt
	logger.Debug("Received heartbeat",
		zap.String("service_id", hb.ServiceId),
		zap.String("service_type", service.Info.ServiceType.String()),
		zap.Any("metrics", hb.Metrics),
	)

	return &emptypb.Empty{}, nil
}

// GetServiceStatus implements the GetServiceStatus RPC
func (s *ControlPlaneServer) GetServiceStatus(ctx context.Context, info *pb.ServiceInfo) (*pb.ServiceStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[info.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", info.ServiceId)
	}

	return service.Status, nil
}

// ListServices implements the ListServices RPC
func (s *ControlPlaneServer) ListServices(ctx context.Context, _ *emptypb.Empty) (*pb.ServiceList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]*pb.ServiceStatus, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service.Status)
	}

	return &pb.ServiceList{Services: services}, nil
}

// runJanitor periodically checks for services that haven't sent heartbeats
// and marks them as unhealthy
func (s *ControlPlaneServer) runJanitor() {
	defer s.janitorWaitGroup.Done()
	
	// Make a copy of the settings to avoid holding a lock
	s.mu.RLock()
	interval := s.janitorInterval
	s.mu.RUnlock()
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			logger.Info("Stopping health check janitor")
			return
		case <-ticker.C:
			s.checkServiceHealth()
		}
	}
}

// checkServiceHealth checks all services for stale heartbeats
func (s *ControlPlaneServer) checkServiceHealth() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	staleThreshold := now.Add(-s.heartbeatTTL)
	ctx := context.Background()

	for id, service := range s.services {
		// If the service hasn't sent a heartbeat within the TTL
		if service.LastSeen.Before(staleThreshold) {
			if service.Status.IsHealthy {
				// Mark it as unhealthy and log the event
				service.Status.IsHealthy = false
				logger.Warn("Service marked unhealthy due to stale heartbeat",
					zap.String("service_id", id),
					zap.String("service_type", service.Info.ServiceType.String()),
					zap.Time("last_seen", service.LastSeen),
					zap.Duration("ttl", s.heartbeatTTL))
				
				// Update in persistent storage if available
				if s.storage != nil {
					err := s.storage.UpdateService(ctx, id, func(storedService *storage.ServiceInfo) error {
						storedService.Status.IsHealthy = false
						return nil
					})
					
					if err != nil && !storage.IsNotFound(err) {
						logger.Error("Failed to update service health status in storage",
							zap.String("service_id", id),
							zap.Error(err))
					}
				}
			}
		}
	}
}
