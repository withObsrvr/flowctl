package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/google/uuid"
	"github.com/withobsrvr/flowctl/internal/controlplane"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/orchestrator"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PipelineRunner manages the lifecycle of a pipeline with embedded control plane
type PipelineRunner struct {
	pipeline     *model.Pipeline
	orchestrator orchestrator.Orchestrator
	controlPlane *controlplane.EmbeddedControlPlane
	ctx          context.Context
	cancel       context.CancelFunc
	config       Config

	// Run tracking fields
	runID         string       // Current run ID
	runStartTime  time.Time    // When run started
	eventsCount   atomic.Int64 // Thread-safe event counter
	stopRequested atomic.Bool
	stopOnce      sync.Once
}

// Config holds configuration for the pipeline runner
type Config struct {
	OrchestratorType    string
	ControlPlanePort    int
	ControlPlaneAddress string
	UseExternalCP       bool // If true, use external control plane instead of embedded
	ShowStatus          bool
	LogDir              string
	HeartbeatTTL        time.Duration
	JanitorInterval     time.Duration
	DBPath              string
	NoPersistence       bool
}

// NewPipelineRunner creates a new pipeline runner with embedded control plane
func NewPipelineRunner(pipeline *model.Pipeline, config Config) (*PipelineRunner, error) {
	// Set defaults
	if config.ControlPlaneAddress == "" {
		config.ControlPlaneAddress = "127.0.0.1"
	}
	// Preserve port 0 so the embedded control plane can auto-select a free port.
	// CLI callers already default to 8080 unless the user explicitly passes 0.
	if config.OrchestratorType == "" {
		config.OrchestratorType = "process"
	}
	if config.LogDir == "" {
		config.LogDir = "logs"
	}
	if config.HeartbeatTTL == 0 {
		config.HeartbeatTTL = 30 * time.Second
	}
	if config.JanitorInterval == 0 {
		config.JanitorInterval = 10 * time.Second
	}

	// Create embedded control plane (if not using external)
	var embeddedCP *controlplane.EmbeddedControlPlane
	var controlPlaneAddr string

	if !config.UseExternalCP {
		var serviceStorage storage.ServiceStorage
		if config.NoPersistence {
			serviceStorage = storage.NewMemoryStorage()
		} else {
			dbPath, err := resolveEmbeddedDBPath(config.DBPath)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve embedded control plane db path: %w", err)
			}
			serviceStorage = storage.NewBoltDBStorage(&storage.BoltOptions{Path: dbPath})
			logger.Info("Using persistent embedded control plane storage",
				zap.String("path", dbPath))
		}

		// Create embedded control plane
		controlPlaneConfig := controlplane.Config{
			Address:         config.ControlPlaneAddress,
			Port:            config.ControlPlanePort,
			HeartbeatTTL:    config.HeartbeatTTL,
			JanitorInterval: config.JanitorInterval,
			Storage:         serviceStorage,
		}

		embeddedCP = controlplane.NewEmbeddedControlPlane(controlPlaneConfig)
		controlPlaneAddr = embeddedCP.GetEndpoint()
	} else {
		// Use external control plane
		controlPlaneAddr = fmt.Sprintf("%s:%d", config.ControlPlaneAddress, config.ControlPlanePort)
		logger.Info("Using external control plane",
			zap.String("endpoint", controlPlaneAddr))
	}

	// Create orchestrator with control plane endpoint
	var orch orchestrator.Orchestrator

	switch config.OrchestratorType {
	case "process":
		processOrch, err := orchestrator.NewProcessOrchestrator(controlPlaneAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to create process orchestrator: %w", err)
		}
		orch = processOrch
	case "container", "docker":
		// Create Docker orchestrator
		dockerOrch, err := orchestrator.NewDockerOrchestrator(controlPlaneAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker orchestrator: %w", err)
		}
		orch = dockerOrch
	default:
		return nil, fmt.Errorf("unknown orchestrator type: %s", config.OrchestratorType)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PipelineRunner{
		pipeline:     pipeline,
		orchestrator: orch,
		controlPlane: embeddedCP,
		ctx:          ctx,
		cancel:       cancel,
		config:       config,
	}, nil
}

func resolveEmbeddedDBPath(dbPath string) (string, error) {
	if dbPath != "" {
		return dbPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".flowctl", storage.DefaultBoltFilePath), nil
}

// Run starts the pipeline with embedded or external control plane
func (r *PipelineRunner) Run() error {
	if r.config.UseExternalCP {
		logger.Info("Starting pipeline with external control plane",
			zap.String("pipeline", r.pipeline.Metadata.Name),
			zap.String("orchestrator", r.config.OrchestratorType),
			zap.String("control_plane_endpoint", fmt.Sprintf("%s:%d", r.config.ControlPlaneAddress, r.config.ControlPlanePort)))
	} else {
		logger.Info("Starting pipeline with embedded control plane",
			zap.String("pipeline", r.pipeline.Metadata.Name),
			zap.String("orchestrator", r.config.OrchestratorType),
			zap.Int("control_plane_port", r.config.ControlPlanePort))
	}

	// Start embedded control plane if we have one
	if r.controlPlane != nil {
		if err := r.controlPlane.Start(r.ctx); err != nil {
			return fmt.Errorf("failed to start control plane: %w", err)
		}

		if err := r.syncEmbeddedControlPlaneEndpoint(); err != nil {
			r.controlPlane.Stop()
			return fmt.Errorf("failed to sync embedded control plane endpoint: %w", err)
		}

		// Wait for control plane to be ready
		if err := r.waitForControlPlaneReady(); err != nil {
			r.controlPlane.Stop()
			return fmt.Errorf("control plane not ready: %w", err)
		}
	} else {
		// Verify external control plane is accessible
		if err := r.waitForControlPlaneReady(); err != nil {
			return fmt.Errorf("external control plane not accessible at %s:%d: %w",
				r.config.ControlPlaneAddress, r.config.ControlPlanePort, err)
		}
		logger.Info("External control plane is accessible")
	}

	// Create pipeline run record
	r.createPipelineRun()

	// Start components in dependency order
	if err := r.startComponents(); err != nil {
		r.updateRunStatus(flowctlpb.RunStatus_RUN_STATUS_FAILED)
		if r.controlPlane != nil {
			r.controlPlane.Stop()
		}
		return err
	}

	// Wait for all components to register
	if err := r.waitForComponentRegistration(); err != nil {
		r.updateRunStatus(flowctlpb.RunStatus_RUN_STATUS_FAILED)
		if r.controlPlane != nil {
			r.controlPlane.Stop()
		}
		return err
	}

	if err := r.enrichRegisteredComponents(); err != nil {
		logger.Warn("Failed to enrich registered component metadata", zap.Error(err))
	}

	// Wire components together for data flow
	streamOrch := NewStreamOrchestrator(r.ctx, r.controlPlane, r.pipeline)
	if err := streamOrch.WireAll(); err != nil {
		r.updateRunStatus(flowctlpb.RunStatus_RUN_STATUS_FAILED)
		if r.controlPlane != nil {
			r.controlPlane.Stop()
		}
		return fmt.Errorf("failed to wire components: %w", err)
	}

	// Update status to RUNNING now that pipeline is fully wired
	r.updateRunStatus(flowctlpb.RunStatus_RUN_STATUS_RUNNING)

	// Start monitoring if enabled
	if r.config.ShowStatus {
		go r.monitorPipeline()

		// Start log aggregation
		logAggregator := r.StartAggregatedLogging()
		defer logAggregator.Stop()

		// Print aggregated logs
		go func() {
			for logEntry := range logAggregator.GetLogChannel() {
				logAggregator.PrintLog(logEntry)
			}
		}()
	}

	logger.Info("Pipeline started successfully with control plane integration")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown
	var shutdownStatus flowctlpb.RunStatus
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal")
		shutdownStatus = flowctlpb.RunStatus_RUN_STATUS_STOPPED
	case <-r.ctx.Done():
		logger.Info("Context cancelled")
		if r.stopRequested.Load() {
			shutdownStatus = flowctlpb.RunStatus_RUN_STATUS_STOPPED
		} else {
			shutdownStatus = flowctlpb.RunStatus_RUN_STATUS_COMPLETED
		}
	}

	// Update run status before shutdown
	r.updateRunStatus(shutdownStatus)

	return r.shutdown()
}

// startComponents starts all pipeline components in dependency order
func (r *PipelineRunner) startComponents() error {
	logger.Info("Starting pipeline components")

	// Start sources first (they have no dependencies)
	for _, source := range r.pipeline.Spec.Sources {
		component := r.convertToOrchestratorComponent(source, "source")
		if err := r.orchestrator.StartComponent(r.ctx, component); err != nil {
			return fmt.Errorf("failed to start source %s: %w", source.ID, err)
		}
		logger.Info("Started source component", zap.String("id", source.ID))
	}

	// Start processors (they depend on sources)
	for _, processor := range r.pipeline.Spec.Processors {
		component := r.convertToOrchestratorComponent(processor, "processor")
		if err := r.orchestrator.StartComponent(r.ctx, component); err != nil {
			return fmt.Errorf("failed to start processor %s: %w", processor.ID, err)
		}
		logger.Info("Started processor component", zap.String("id", processor.ID))
	}

	// Start sinks (they depend on processors)
	for _, sink := range r.pipeline.Spec.Sinks {
		component := r.convertToOrchestratorComponent(sink, "sink")
		if err := r.orchestrator.StartComponent(r.ctx, component); err != nil {
			return fmt.Errorf("failed to start sink %s: %w", sink.ID, err)
		}
		logger.Info("Started sink component", zap.String("id", sink.ID))
	}

	// Start pipeline components (new)
	if len(r.pipeline.Spec.Pipelines) > 0 {
		logger.Info("Starting pipeline components", zap.Int("count", len(r.pipeline.Spec.Pipelines)))

		// Handle dependencies between pipelines
		started := make(map[string]bool)

		// Helper function to start a pipeline component
		var startPipeline func(pipeline model.Component) error
		startPipeline = func(pipeline model.Component) error {
			// Skip if already started
			if started[pipeline.ID] {
				return nil
			}

			// Start dependencies first
			for _, dep := range pipeline.DependsOn {
				// Find the dependency
				for _, p := range r.pipeline.Spec.Pipelines {
					if p.ID == dep && !started[p.ID] {
						if err := startPipeline(p); err != nil {
							return err
						}
					}
				}
			}

			// Start the pipeline component
			component := r.convertToOrchestratorComponent(pipeline, "pipeline")
			if err := r.orchestrator.StartComponent(r.ctx, component); err != nil {
				return fmt.Errorf("failed to start pipeline %s: %w", pipeline.ID, err)
			}

			logger.Info("Started pipeline component",
				zap.String("id", pipeline.ID),
				zap.String("image", pipeline.Image))

			started[pipeline.ID] = true
			return nil
		}

		// Start all pipelines respecting dependencies
		for _, pipeline := range r.pipeline.Spec.Pipelines {
			if err := startPipeline(pipeline); err != nil {
				return err
			}
		}
	}

	return nil
}

// convertToOrchestratorComponent converts a model.Component to orchestrator.Component
func (r *PipelineRunner) convertToOrchestratorComponent(modelComp model.Component, componentType string) *orchestrator.Component {
	orchComp := &orchestrator.Component{
		ID:            modelComp.ID,
		Type:          modelComp.Type,
		Name:          modelComp.ID,
		Image:         modelComp.Image,
		Command:       modelComp.Command,
		Args:          modelComp.Args,
		Environment:   modelComp.Env,
		Dependencies:  modelComp.Inputs,
		RestartPolicy: modelComp.RestartPolicy,
	}

	// Convert ports
	for _, port := range modelComp.Ports {
		orchComp.Ports = append(orchComp.Ports, orchestrator.Port{
			Name:     port.Name,
			Port:     port.ContainerPort,
			Protocol: port.Protocol,
		})
	}

	// Convert volumes
	for _, vol := range modelComp.Volumes {
		orchComp.Volumes = append(orchComp.Volumes, orchestrator.VolumeMount{
			HostPath:      vol.HostPath,
			ContainerPath: vol.ContainerPath,
			ReadOnly:      vol.ReadOnly,
		})
	}

	// Convert secrets to volume mounts (secrets are read-only by nature)
	for _, secret := range modelComp.Secrets {
		// Only process file/dir type secrets as volume mounts
		// env type secrets would be handled separately (but not yet implemented)
		if secret.Type == "file" || secret.Type == "dir" {
			orchComp.Volumes = append(orchComp.Volumes, orchestrator.VolumeMount{
				HostPath:      secret.HostPath,
				ContainerPath: secret.ContainerPath,
				ReadOnly:      true, // Secrets are always read-only
			})
		}
	}

	// If no type is specified, use the component type from pipeline structure
	if orchComp.Type == "" {
		orchComp.Type = componentType
	}

	return orchComp
}

// waitForComponentRegistration waits for all components to register with control plane
func (r *PipelineRunner) waitForComponentRegistration() error {
	logger.Info("Waiting for components to register with control plane")

	// Collect all component IDs
	var componentIDs []string
	for _, source := range r.pipeline.Spec.Sources {
		componentIDs = append(componentIDs, source.ID)
	}
	for _, processor := range r.pipeline.Spec.Processors {
		componentIDs = append(componentIDs, processor.ID)
	}
	for _, sink := range r.pipeline.Spec.Sinks {
		componentIDs = append(componentIDs, sink.ID)
	}

	// Wait for each component to register
	for _, componentID := range componentIDs {
		logger.Info("Waiting for component registration", zap.String("component", componentID))

		// Use different waiting logic based on control plane type
		if r.controlPlane != nil {
			// Embedded control plane
			if err := r.controlPlane.WaitForComponent(componentID, 30*time.Second); err != nil {
				return fmt.Errorf("component %s failed to register: %w", componentID, err)
			}
		} else {
			// External control plane - use gRPC client
			if err := r.waitForExternalComponent(componentID, 30*time.Second); err != nil {
				return fmt.Errorf("component %s failed to register: %w", componentID, err)
			}
		}

		logger.Info("Component registered successfully", zap.String("component", componentID))
	}

	return nil
}

// waitForExternalComponent waits for a component to register with external control plane
func (r *PipelineRunner) waitForExternalComponent(componentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Connect to external control plane
		client, conn, err := r.getControlPlaneClient()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// List services to check if component is registered
		resp, err := client.ListServices(r.ctx, &emptypb.Empty{})
		conn.Close()

		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if our component is in the list
		for _, service := range resp.Services {
			if service.ComponentId == componentID {
				logger.Debug("Found registered component via external control plane",
					zap.String("component_id", componentID))
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for component %s to register with external control plane", componentID)
}

// monitorPipeline monitors the pipeline status and logs periodic updates
func (r *PipelineRunner) monitorPipeline() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.logPipelineStatus()
		}
	}
}

// logPipelineStatus logs the current status of all components
func (r *PipelineRunner) logPipelineStatus() {
	services, err := r.controlPlane.GetServiceList()
	if err != nil {
		logger.Error("Failed to get service list", zap.Error(err))
		return
	}

	healthyCount := 0
	unhealthyComponents := []string{}

	for _, service := range services {
		if service.Status == 2 { // HEALTH_STATUS_HEALTHY (correct value from proto)
			healthyCount++
		} else {
			if service.Component != nil {
				unhealthyComponents = append(unhealthyComponents, service.Component.Id)
			}
		}
	}

	logger.Info("Pipeline status",
		zap.Int("total_components", len(services)),
		zap.Int("healthy_components", healthyCount),
		zap.Strings("unhealthy_components", unhealthyComponents))

	// Log component metrics if available
	for _, service := range services {
		if len(service.Metrics) > 0 {
			componentId := "unknown"
			if service.Component != nil {
				componentId = service.Component.Id
			}
			logger.Debug("Component metrics",
				zap.String("component", componentId),
				zap.Any("metrics", service.Metrics))
		}
	}
}

// shutdown gracefully shuts down the pipeline and control plane
func (r *PipelineRunner) shutdown() error {
	logger.Info("Shutting down pipeline and control plane")

	if r.controlPlane != nil && r.runID != "" {
		defer r.controlPlane.UnregisterRunStopper(r.runID)
	}

	// Create a new context for shutdown operations with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop orchestrator (this will stop all components)
	if err := r.orchestrator.StopAll(shutdownCtx); err != nil {
		logger.Error("Error stopping orchestrator", zap.Error(err))
	}

	// Cancel the main context after stopping containers
	r.cancel()

	// Give components time to deregister
	time.Sleep(2 * time.Second)

	// Stop embedded control plane (if we have one)
	if r.controlPlane != nil {
		if err := r.controlPlane.Stop(); err != nil {
			logger.Error("Error stopping control plane", zap.Error(err))
		}
	}

	logger.Info("Pipeline and control plane stopped successfully")
	return nil
}

// GetControlPlaneEndpoint returns the control plane endpoint
func (r *PipelineRunner) GetControlPlaneEndpoint() string {
	if r.controlPlane != nil {
		return r.controlPlane.GetEndpoint()
	}
	return fmt.Sprintf("%s:%d", r.config.ControlPlaneAddress, r.config.ControlPlanePort)
}

// GetPipelineStatus returns the current status of the pipeline
func (r *PipelineRunner) GetPipelineStatus() (map[string]*orchestrator.ComponentStatus, error) {
	return r.orchestrator.GetAllStatus(r.ctx)
}

// IsHealthy returns true if all components are healthy
func (r *PipelineRunner) IsHealthy() bool {
	services, err := r.controlPlane.GetServiceList()
	if err != nil {
		return false
	}

	for _, service := range services {
		if service.Status != 1 { // Not HEALTH_STATUS_HEALTHY
			return false
		}
	}

	return true
}

func (r *PipelineRunner) syncEmbeddedControlPlaneEndpoint() error {
	endpoint := r.controlPlane.GetEndpoint()
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid control plane endpoint %q: %w", endpoint, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid control plane port %q: %w", portStr, err)
	}

	r.config.ControlPlaneAddress = host
	r.config.ControlPlanePort = port
	r.updatePipelineControlPlaneEndpoint(endpoint)

	// Recreate the orchestrator so components receive the real endpoint,
	// especially when the requested port was 0 and the OS picked a free port.
	switch r.config.OrchestratorType {
	case "process":
		processOrch, err := orchestrator.NewProcessOrchestrator(endpoint)
		if err != nil {
			return fmt.Errorf("failed to recreate process orchestrator: %w", err)
		}
		r.orchestrator = processOrch
	case "container", "docker":
		dockerOrch, err := orchestrator.NewDockerOrchestrator(endpoint)
		if err != nil {
			return fmt.Errorf("failed to recreate docker orchestrator: %w", err)
		}
		r.orchestrator = dockerOrch
	default:
		return fmt.Errorf("unknown orchestrator type: %s", r.config.OrchestratorType)
	}

	return nil
}

// waitForControlPlaneReady waits for the control plane to be ready
func (r *PipelineRunner) updatePipelineControlPlaneEndpoint(endpoint string) {
	updateEnv := func(comp *model.Component) {
		if comp.Env == nil {
			comp.Env = make(map[string]string)
		}
		if _, enabled := comp.Env["ENABLE_FLOWCTL"]; enabled || len(comp.Command) > 0 || comp.Type != "" || comp.Image != "" {
			comp.Env["FLOWCTL_ENDPOINT"] = endpoint
		}
	}

	for i := range r.pipeline.Spec.Sources {
		updateEnv(&r.pipeline.Spec.Sources[i])
	}
	for i := range r.pipeline.Spec.Processors {
		updateEnv(&r.pipeline.Spec.Processors[i])
	}
	for i := range r.pipeline.Spec.Sinks {
		updateEnv(&r.pipeline.Spec.Sinks[i])
	}
	for i := range r.pipeline.Spec.Pipelines {
		updateEnv(&r.pipeline.Spec.Pipelines[i])
	}
}

func (r *PipelineRunner) waitForControlPlaneReady() error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	logger.Debug("Waiting for control plane to be ready...")

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for control plane to be ready")
		case <-ticker.C:
			// For embedded control plane
			if r.controlPlane != nil {
				if r.controlPlane.IsStarted() {
					// Try a simple health check by getting service list
					_, err := r.controlPlane.GetServiceList()
					if err == nil {
						logger.Debug("Control plane is ready")
						return nil
					}
					logger.Debug("Control plane not ready yet", zap.Error(err))
				}
			} else {
				// For external control plane, try to connect
				client, conn, err := r.getControlPlaneClient()
				if err == nil {
					// Try a simple RPC call to verify it's working
					_, listErr := client.ListServices(r.ctx, &emptypb.Empty{})
					conn.Close()
					if listErr == nil {
						logger.Debug("External control plane is ready")
						return nil
					}
					logger.Debug("External control plane not ready yet", zap.Error(listErr))
				} else {
					logger.Debug("Cannot connect to external control plane yet", zap.Error(err))
				}
			}
		}
	}
}

// getControlPlaneClient returns a gRPC client for the control plane
func (r *PipelineRunner) controlPlaneEndpoint() string {
	return fmt.Sprintf("%s:%d", r.config.ControlPlaneAddress, r.config.ControlPlanePort)
}

func (r *PipelineRunner) getControlPlaneClient() (flowctlpb.ControlPlaneClient, *grpc.ClientConn, error) {
	endpoint := r.controlPlaneEndpoint()

	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)
	}

	client := flowctlpb.NewControlPlaneClient(conn)
	return client, conn, nil
}

func (r *PipelineRunner) getV1ControlPlaneClient() (flowctlv1.ControlPlaneServiceClient, *grpc.ClientConn, error) {
	endpoint := r.controlPlaneEndpoint()

	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)
	}

	client := flowctlv1.NewControlPlaneServiceClient(conn)
	return client, conn, nil
}

// getComponentIDs returns all component IDs in the pipeline
func (r *PipelineRunner) getComponentIDs() []string {
	var componentIDs []string

	for _, source := range r.pipeline.Spec.Sources {
		componentIDs = append(componentIDs, source.ID)
	}
	for _, processor := range r.pipeline.Spec.Processors {
		componentIDs = append(componentIDs, processor.ID)
	}
	for _, sink := range r.pipeline.Spec.Sinks {
		componentIDs = append(componentIDs, sink.ID)
	}
	for _, pipeline := range r.pipeline.Spec.Pipelines {
		componentIDs = append(componentIDs, pipeline.ID)
	}

	return componentIDs
}

// createPipelineRun creates a run record in the control plane
func componentTypeForRole(role string) flowctlv1.ComponentType {
	switch role {
	case "source":
		return flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE
	case "processor":
		return flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR
	case "sink":
		return flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER
	default:
		return flowctlv1.ComponentType_COMPONENT_TYPE_UNSPECIFIED
	}
}

func (r *PipelineRunner) enrichRegisteredComponents() error {
	client, conn, err := r.getV1ControlPlaneClient()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	components := []struct {
		role string
		comp model.Component
	}{ }
	for _, source := range r.pipeline.Spec.Sources {
		components = append(components, struct {
			role string
			comp model.Component
		}{role: "source", comp: source})
	}
	for _, processor := range r.pipeline.Spec.Processors {
		components = append(components, struct {
			role string
			comp model.Component
		}{role: "processor", comp: processor})
	}
	for _, sink := range r.pipeline.Spec.Sinks {
		components = append(components, struct {
			role string
			comp model.Component
		}{role: "sink", comp: sink})
	}

	for _, item := range components {
		status, err := client.GetComponentStatus(ctx, &flowctlv1.ComponentStatusRequest{ServiceId: item.comp.ID})
		if err != nil || status.GetComponent() == nil {
			continue
		}

		info := status.Component
		needsEnrichment := len(info.InputEventTypes) == 0 && len(item.comp.InputEventTypes) > 0 ||
			len(info.OutputEventTypes) == 0 && len(item.comp.OutputEventTypes) > 0
		if !needsEnrichment {
			continue
		}

		enriched := &flowctlv1.ComponentInfo{
			Id:               info.Id,
			Type:             info.Type,
			Name:             info.Name,
			Version:          info.Version,
			Description:      info.Description,
			Endpoint:         info.Endpoint,
			InputEventTypes:  append([]string(nil), info.InputEventTypes...),
			OutputEventTypes: append([]string(nil), info.OutputEventTypes...),
			Metadata:         make(map[string]string, len(info.Metadata)),
		}
		for k, v := range info.Metadata {
			enriched.Metadata[k] = v
		}
		if len(enriched.InputEventTypes) == 0 && len(item.comp.InputEventTypes) > 0 {
			enriched.InputEventTypes = append([]string(nil), item.comp.InputEventTypes...)
		}
		if len(enriched.OutputEventTypes) == 0 && len(item.comp.OutputEventTypes) > 0 {
			enriched.OutputEventTypes = append([]string(nil), item.comp.OutputEventTypes...)
		}
		if enriched.Type == flowctlv1.ComponentType_COMPONENT_TYPE_UNSPECIFIED {
			enriched.Type = componentTypeForRole(item.role)
		}

		_, err = client.RegisterComponent(ctx, &flowctlv1.RegisterRequest{
			ComponentId: item.comp.ID,
			Component:   enriched,
		})
		if err != nil {
			return fmt.Errorf("failed to enrich component %s: %w", item.comp.ID, err)
		}
	}

	return nil
}

func (r *PipelineRunner) createPipelineRun() {
	// Generate run ID
	r.runID = uuid.New().String()
	r.runStartTime = time.Now()

	logger.Info("Creating pipeline run record",
		zap.String("run_id", r.runID),
		zap.String("pipeline_name", r.pipeline.Metadata.Name))

	// Get component IDs
	componentIDs := r.getComponentIDs()

	// Call CreatePipelineRun RPC
	client, conn, err := r.getControlPlaneClient()
	if err != nil {
		logger.Warn("Failed to connect to control plane for run tracking",
			zap.String("run_id", r.runID),
			zap.Error(err))
		return
	}
	defer conn.Close()

	_, err = client.CreatePipelineRun(r.ctx, &flowctlpb.CreatePipelineRunRequest{
		RunId:        r.runID,
		PipelineName: r.pipeline.Metadata.Name,
		ComponentIds: componentIDs,
	})

	if err != nil {
		logger.Warn("Failed to create pipeline run record",
			zap.String("run_id", r.runID),
			zap.Error(err))
		// Don't fail the pipeline, just log the error
		return
	}

	if r.controlPlane != nil {
		r.controlPlane.RegisterRunStopper(r.runID, r.Stop)
	}

	logger.Info("Pipeline run record created successfully",
		zap.String("run_id", r.runID),
		zap.String("pipeline_name", r.pipeline.Metadata.Name))
}

// Stop requests graceful shutdown of the running pipeline.
func (r *PipelineRunner) Stop() {
	r.stopOnce.Do(func() {
		r.stopRequested.Store(true)
		r.cancel()
	})
}

// updateRunStatus updates the run status in the control plane
func (r *PipelineRunner) updateRunStatus(status flowctlpb.RunStatus) {
	if r.runID == "" {
		// No run ID means run tracking failed to initialize
		return
	}

	logger.Debug("Updating pipeline run status",
		zap.String("run_id", r.runID),
		zap.String("status", status.String()))

	// Get control plane client
	client, conn, err := r.getControlPlaneClient()
	if err != nil {
		logger.Warn("Failed to connect to control plane for status update",
			zap.String("run_id", r.runID),
			zap.Error(err))
		return
	}
	defer conn.Close()

	// Calculate metrics
	eventsProcessed := r.eventsCount.Load()
	duration := time.Since(r.runStartTime).Seconds()
	eventsPerSecond := 0.0
	if duration > 0 {
		eventsPerSecond = float64(eventsProcessed) / duration
	}

	metrics := &flowctlpb.RunMetrics{
		EventsProcessed: eventsProcessed,
		EventsPerSecond: eventsPerSecond,
	}

	// Build update request (end time is automatically set by server for terminal states)
	req := &flowctlpb.UpdatePipelineRunRequest{
		RunId:   r.runID,
		Status:  status,
		Metrics: metrics,
	}

	_, err = client.UpdatePipelineRun(r.ctx, req)
	if err != nil {
		logger.Warn("Failed to update pipeline run status",
			zap.String("run_id", r.runID),
			zap.String("status", status.String()),
			zap.Error(err))
		return
	}

	logger.Info("Pipeline run status updated",
		zap.String("run_id", r.runID),
		zap.String("status", status.String()),
		zap.Int64("events_processed", eventsProcessed),
		zap.Float64("events_per_second", eventsPerSecond))
}
