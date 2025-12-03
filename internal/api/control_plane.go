package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Service represents a registered service
type Service struct {
	Info     *flowctlv1.ComponentInfo
	Status   *flowctlv1.ComponentStatusResponse
	LastSeen time.Time
	Conn     *grpc.ClientConn
}

// ControlPlaneServer implements the control plane gRPC service
type ControlPlaneServer struct {
	flowctlv1.UnimplementedControlPlaneServiceServer
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
		s.services[service.Info.Id] = service
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

// RegisterComponent implements the RegisterComponent RPC
func (s *ControlPlaneServer) RegisterComponent(ctx context.Context, req *flowctlv1.RegisterRequest) (*flowctlv1.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := req.Component

	// Use component_id from request if provided, otherwise use component.id
	serviceId := req.ComponentId
	if serviceId == "" {
		serviceId = info.Id
	}
	if serviceId == "" {
		serviceId = fmt.Sprintf("%s-%d", info.Type, time.Now().UnixNano())
	}

	// Create service record
	now := time.Now()
	service := &Service{
		Info: info,
		Status: &flowctlv1.ComponentStatusResponse{
			Component:     info,
			Status:        flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
			LastHeartbeat: timestamppb.New(now),
			Metrics:       make(map[string]string),
			RegisteredAt:  timestamppb.New(now),
		},
		LastSeen: now,
	}

	// Store service in memory
	s.services[serviceId] = service

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
				zap.String("service_id", serviceId),
				zap.Error(err))
			// We continue anyway as the service is registered in memory
		}
	}

	// Log service registration
	logger.Info("Service registered",
		zap.String("service_id", serviceId),
		zap.String("component_id", info.Id),
		zap.String("component_type", info.Type.String()),
		zap.String("endpoint", info.Endpoint),
		zap.Strings("input_event_types", info.InputEventTypes),
		zap.Strings("output_event_types", info.OutputEventTypes),
	)

	// Generate topic names based on component type and event types
	var topicNames []string
	switch info.Type {
	case flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	case flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	}

	// Return registration response
	return &flowctlv1.RegisterResponse{
		ServiceId:      serviceId,
		AssignedTopics: topicNames,
		ConnectionInfo: map[string]string{
			"control_plane_endpoint": "localhost:8080",
		},
	}, nil
}

// Heartbeat implements the Heartbeat RPC
func (s *ControlPlaneServer) Heartbeat(ctx context.Context, req *flowctlv1.HeartbeatRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[req.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", req.ServiceId)
	}

	now := time.Now()
	service.LastSeen = now
	service.Status.Metrics = req.Metrics
	service.Status.Status = req.Status
	service.Status.LastHeartbeat = timestamppb.New(now)

	// Update in persistent storage if available
	if s.storage != nil {
		err := s.storage.UpdateService(ctx, req.ServiceId, func(storedService *storage.ServiceInfo) error {
			storedService.LastSeen = now
			storedService.Status.Metrics = req.Metrics
			storedService.Status.Status = req.Status
			storedService.Status.LastHeartbeat = timestamppb.New(now)
			return nil
		})

		if err != nil && !storage.IsNotFound(err) {
			logger.Error("Failed to update service heartbeat in storage",
				zap.String("service_id", req.ServiceId),
				zap.Error(err))
			// Continue anyway as the heartbeat is processed in memory
		}
	}

	// Log heartbeat receipt
	logger.Debug("Received heartbeat",
		zap.String("service_id", req.ServiceId),
		zap.String("component_type", service.Info.Type.String()),
		zap.Any("metrics", req.Metrics),
	)

	return &emptypb.Empty{}, nil
}

// GetComponentStatus implements the GetComponentStatus RPC
func (s *ControlPlaneServer) GetComponentStatus(ctx context.Context, req *flowctlv1.ComponentStatusRequest) (*flowctlv1.ComponentStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[req.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", req.ServiceId)
	}

	return service.Status, nil
}

// ListComponents implements the ListComponents RPC
func (s *ControlPlaneServer) ListComponents(ctx context.Context, req *flowctlv1.ListComponentsRequest) (*flowctlv1.ListComponentsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	components := make([]*flowctlv1.ComponentStatusResponse, 0, len(s.services))
	for _, service := range s.services {
		// Filter by type if requested
		if req.TypeFilter != flowctlv1.ComponentType_COMPONENT_TYPE_UNSPECIFIED && service.Info.Type != req.TypeFilter {
			continue
		}

		// Filter unhealthy if not requested
		if !req.IncludeUnhealthy && service.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
			continue
		}

		components = append(components, service.Status)
	}

	return &flowctlv1.ListComponentsResponse{Components: components}, nil
}

// UnregisterComponent implements the UnregisterComponent RPC
func (s *ControlPlaneServer) UnregisterComponent(ctx context.Context, req *flowctlv1.UnregisterRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.services, req.ServiceId)

	if s.storage != nil {
		// Remove from storage
		if err := s.storage.UnregisterService(ctx, req.ServiceId); err != nil {
			logger.Error("Failed to unregister service from storage",
				zap.String("service_id", req.ServiceId),
				zap.Error(err))
		}
	}

	logger.Info("Service unregistered", zap.String("service_id", req.ServiceId))
	return &emptypb.Empty{}, nil
}

// DiscoverComponents implements the DiscoverComponents RPC
func (s *ControlPlaneServer) DiscoverComponents(ctx context.Context, req *flowctlv1.DiscoveryRequest) (*flowctlv1.DiscoveryResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchingComponents []*flowctlv1.ComponentInfo

	for _, service := range s.services {
		// Filter by type if specified
		if req.Type != flowctlv1.ComponentType_COMPONENT_TYPE_UNSPECIFIED && service.Info.Type != req.Type {
			continue
		}

		// Filter by event types if specified (OR logic)
		if len(req.EventTypes) > 0 {
			found := false
			allEventTypes := append(service.Info.InputEventTypes, service.Info.OutputEventTypes...)
			for _, requestedType := range req.EventTypes {
				for _, componentEventType := range allEventTypes {
					if componentEventType == requestedType {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by metadata (AND logic)
		if len(req.MetadataFilters) > 0 {
			matches := true
			for key, value := range req.MetadataFilters {
				if service.Info.Metadata == nil || service.Info.Metadata[key] != value {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
		}

		matchingComponents = append(matchingComponents, service.Info)
	}

	return &flowctlv1.DiscoveryResponse{Components: matchingComponents}, nil
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
			if service.Status.Status == flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
				// Mark it as unhealthy and log the event
				service.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
				logger.Warn("Service marked unhealthy due to stale heartbeat",
					zap.String("service_id", id),
					zap.String("component_type", service.Info.Type.String()),
					zap.Time("last_seen", service.LastSeen),
					zap.Duration("ttl", s.heartbeatTTL))

				// Update in persistent storage if available
				if s.storage != nil {
					err := s.storage.UpdateService(ctx, id, func(storedService *storage.ServiceInfo) error {
						storedService.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
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
