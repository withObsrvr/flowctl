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
	flowctlpb "github.com/withobsrvr/flowctl/proto"
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

// Pipeline run management RPC methods

// CreatePipelineRun implements the CreatePipelineRun RPC
func (s *ControlPlaneServer) CreatePipelineRun(ctx context.Context, req *flowctlpb.CreatePipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	now := time.Now()

	run := &flowctlpb.PipelineRun{
		RunId:        req.RunId,
		PipelineId:   req.PipelineName,  // Use pipeline name as ID for now
		PipelineName: req.PipelineName,
		Status:       flowctlpb.RunStatus_RUN_STATUS_STARTING,
		StartTime:    timestamppb.New(now),
		ConfigYaml:   req.ConfigYaml,
		ComponentIds: req.ComponentIds,
		Metrics: &flowctlpb.RunMetrics{
			EventsProcessed: 0,
			EventsPerSecond: 0.0,
			BytesProcessed:  0,
		},
	}

	// Store in persistent storage if available
	if s.storage != nil {
		runInfo := &storage.PipelineRunInfo{
			Run: run,
		}
		if err := s.storage.CreatePipelineRun(ctx, runInfo); err != nil {
			logger.Error("Failed to create pipeline run in storage",
				zap.String("run_id", req.RunId),
				zap.Error(err))
			return nil, fmt.Errorf("failed to create pipeline run: %w", err)
		}
	}

	logger.Info("Pipeline run created",
		zap.String("run_id", req.RunId),
		zap.String("pipeline_name", req.PipelineName),
		zap.Strings("component_ids", req.ComponentIds))

	return run, nil
}

// UpdatePipelineRun implements the UpdatePipelineRun RPC
func (s *ControlPlaneServer) UpdatePipelineRun(ctx context.Context, req *flowctlpb.UpdatePipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	// Update the run in storage
	var updatedRun *flowctlpb.PipelineRun
	err := s.storage.UpdatePipelineRun(ctx, req.RunId, func(runInfo *storage.PipelineRunInfo) error {
		if req.Status != flowctlpb.RunStatus_RUN_STATUS_UNKNOWN {
			runInfo.Run.Status = req.Status

			// Set end time if completed, failed, or stopped
			if req.Status == flowctlpb.RunStatus_RUN_STATUS_COMPLETED ||
			   req.Status == flowctlpb.RunStatus_RUN_STATUS_FAILED ||
			   req.Status == flowctlpb.RunStatus_RUN_STATUS_STOPPED {
				runInfo.Run.EndTime = timestamppb.New(time.Now())
			}
		}
		if req.Metrics != nil {
			runInfo.Run.Metrics = req.Metrics
		}
		if req.Error != "" {
			runInfo.Run.Error = req.Error
		}
		updatedRun = runInfo.Run
		return nil
	})

	if err != nil {
		logger.Error("Failed to update pipeline run",
			zap.String("run_id", req.RunId),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update pipeline run: %w", err)
	}

	logger.Debug("Pipeline run updated",
		zap.String("run_id", req.RunId),
		zap.String("status", req.Status.String()))

	return updatedRun, nil
}

// GetPipelineRun implements the GetPipelineRun RPC
func (s *ControlPlaneServer) GetPipelineRun(ctx context.Context, req *flowctlpb.GetPipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	runInfo, err := s.storage.GetPipelineRun(ctx, req.RunId)
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, fmt.Errorf("pipeline run not found: %s", req.RunId)
		}
		logger.Error("Failed to get pipeline run",
			zap.String("run_id", req.RunId),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get pipeline run: %w", err)
	}

	return runInfo.Run, nil
}

// ListPipelineRuns implements the ListPipelineRuns RPC
func (s *ControlPlaneServer) ListPipelineRuns(ctx context.Context, req *flowctlpb.ListPipelineRunsRequest) (*flowctlpb.ListPipelineRunsResponse, error) {
	if s.storage == nil {
		return &flowctlpb.ListPipelineRunsResponse{Runs: []*flowctlpb.PipelineRun{}}, nil
	}

	limit := req.Limit
	if limit == 0 {
		limit = 10 // Default limit
	}

	runInfos, err := s.storage.ListPipelineRuns(ctx, req.PipelineName, req.Status, limit)
	if err != nil {
		logger.Error("Failed to list pipeline runs",
			zap.String("pipeline_name", req.PipelineName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to list pipeline runs: %w", err)
	}

	runs := make([]*flowctlpb.PipelineRun, len(runInfos))
	for i, runInfo := range runInfos {
		runs[i] = runInfo.Run
	}

	return &flowctlpb.ListPipelineRunsResponse{Runs: runs}, nil
}

// StopPipelineRun implements the StopPipelineRun RPC
func (s *ControlPlaneServer) StopPipelineRun(ctx context.Context, req *flowctlpb.StopPipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	// Update the run status to stopped
	var stoppedRun *flowctlpb.PipelineRun
	err := s.storage.UpdatePipelineRun(ctx, req.RunId, func(runInfo *storage.PipelineRunInfo) error {
		runInfo.Run.Status = flowctlpb.RunStatus_RUN_STATUS_STOPPED
		runInfo.Run.EndTime = timestamppb.New(time.Now())
		stoppedRun = runInfo.Run
		return nil
	})

	if err != nil {
		if storage.IsNotFound(err) {
			return nil, fmt.Errorf("pipeline run not found: %s", req.RunId)
		}
		logger.Error("Failed to stop pipeline run",
			zap.String("run_id", req.RunId),
			zap.Error(err))
		return nil, fmt.Errorf("failed to stop pipeline run: %w", err)
	}

	logger.Info("Pipeline run stopped",
		zap.String("run_id", req.RunId))

	return stoppedRun, nil
}

// ControlPlaneWrapper wraps ControlPlaneServer to implement the flowctlpb.ControlPlane interface
// This is needed because flowctlpb and v1 have methods with the same names but different signatures
type ControlPlaneWrapper struct {
	flowctlpb.UnimplementedControlPlaneServer
	server *ControlPlaneServer
}

// NewControlPlaneWrapper creates a wrapper that implements flowctlpb.ControlPlane
func NewControlPlaneWrapper(server *ControlPlaneServer) *ControlPlaneWrapper {
	return &ControlPlaneWrapper{server: server}
}

// Register implements flowctlpb.ControlPlane.Register
func (w *ControlPlaneWrapper) Register(ctx context.Context, req *flowctlpb.ServiceInfo) (*flowctlpb.RegistrationAck, error) {
	v1Req := &flowctlv1.RegisterRequest{
		Component: &flowctlv1.ComponentInfo{
			Id:   req.ComponentId,
			Type: convertToV1ComponentType(req.ServiceType),
		},
	}

	_, err := w.server.RegisterComponent(ctx, v1Req)
	if err != nil {
		return nil, err
	}

	return &flowctlpb.RegistrationAck{
		ServiceId: req.ServiceId,
	}, nil
}

// Heartbeat implements flowctlpb.ControlPlane.Heartbeat
func (w *ControlPlaneWrapper) Heartbeat(ctx context.Context, req *flowctlpb.ServiceHeartbeat) (*emptypb.Empty, error) {
	v1Req := &flowctlv1.HeartbeatRequest{
		ServiceId: req.ServiceId,
	}

	return w.server.Heartbeat(ctx, v1Req)
}

// GetServiceStatus implements flowctlpb.ControlPlane.GetServiceStatus
func (w *ControlPlaneWrapper) GetServiceStatus(ctx context.Context, req *flowctlpb.ServiceInfo) (*flowctlpb.ServiceStatus, error) {
	return nil, fmt.Errorf("GetServiceStatus not implemented")
}

// ListServices implements flowctlpb.ControlPlane.ListServices
func (w *ControlPlaneWrapper) ListServices(ctx context.Context, req *emptypb.Empty) (*flowctlpb.ServiceList, error) {
	v1Req := &flowctlv1.ListComponentsRequest{}
	v1Resp, err := w.server.ListComponents(ctx, v1Req)
	if err != nil {
		return nil, err
	}

	services := make([]*flowctlpb.ServiceStatus, len(v1Resp.Components))
	for i, comp := range v1Resp.Components {
		services[i] = &flowctlpb.ServiceStatus{
			ServiceId:   comp.Component.Id,
			ComponentId: comp.Component.Id,
			ServiceType: convertFromV1ServiceType(comp.Component.Type),
		}
	}

	return &flowctlpb.ServiceList{Services: services}, nil
}

// Pipeline run tracking methods - delegate directly to the server
func (w *ControlPlaneWrapper) CreatePipelineRun(ctx context.Context, req *flowctlpb.CreatePipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	return w.server.CreatePipelineRun(ctx, req)
}

func (w *ControlPlaneWrapper) UpdatePipelineRun(ctx context.Context, req *flowctlpb.UpdatePipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	return w.server.UpdatePipelineRun(ctx, req)
}

func (w *ControlPlaneWrapper) GetPipelineRun(ctx context.Context, req *flowctlpb.GetPipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	return w.server.GetPipelineRun(ctx, req)
}

func (w *ControlPlaneWrapper) ListPipelineRuns(ctx context.Context, req *flowctlpb.ListPipelineRunsRequest) (*flowctlpb.ListPipelineRunsResponse, error) {
	return w.server.ListPipelineRuns(ctx, req)
}

func (w *ControlPlaneWrapper) StopPipelineRun(ctx context.Context, req *flowctlpb.StopPipelineRunRequest) (*flowctlpb.PipelineRun, error) {
	return w.server.StopPipelineRun(ctx, req)
}

// Helper functions to convert between v1 and flowctlpb types
func convertToV1ComponentType(t flowctlpb.ServiceType) flowctlv1.ComponentType {
	switch t {
	case flowctlpb.ServiceType_SERVICE_TYPE_SOURCE:
		return flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE
	case flowctlpb.ServiceType_SERVICE_TYPE_PROCESSOR:
		return flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR
	case flowctlpb.ServiceType_SERVICE_TYPE_SINK:
		return flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER
	default:
		return flowctlv1.ComponentType_COMPONENT_TYPE_UNSPECIFIED
	}
}

func convertFromV1ServiceType(t flowctlv1.ComponentType) flowctlpb.ServiceType {
	switch t {
	case flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE:
		return flowctlpb.ServiceType_SERVICE_TYPE_SOURCE
	case flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR:
		return flowctlpb.ServiceType_SERVICE_TYPE_PROCESSOR
	case flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER:
		return flowctlpb.ServiceType_SERVICE_TYPE_SINK
	default:
		return flowctlpb.ServiceType_SERVICE_TYPE_UNSPECIFIED
	}
}
