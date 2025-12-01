package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/withobsrvr/flowctl/internal/controlplane"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/orchestrator"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// PipelineRunner manages the lifecycle of a pipeline with embedded control plane
type PipelineRunner struct {
	pipeline      *model.Pipeline
	orchestrator  orchestrator.Orchestrator
	controlPlane  *controlplane.EmbeddedControlPlane
	ctx           context.Context
	cancel        context.CancelFunc
	config        Config
}

// Config holds configuration for the pipeline runner
type Config struct {
	OrchestratorType     string
	ControlPlanePort     int
	ControlPlaneAddress  string
	ShowStatus           bool
	LogDir               string
	HeartbeatTTL         time.Duration
	JanitorInterval      time.Duration
}

// NewPipelineRunner creates a new pipeline runner with embedded control plane
func NewPipelineRunner(pipeline *model.Pipeline, config Config) (*PipelineRunner, error) {
	// Set defaults
	if config.ControlPlaneAddress == "" {
		config.ControlPlaneAddress = "127.0.0.1"
	}
	if config.ControlPlanePort == 0 {
		config.ControlPlanePort = 8080
	}
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

	// Create embedded control plane
	controlPlaneConfig := controlplane.Config{
		Address:         config.ControlPlaneAddress,
		Port:            config.ControlPlanePort,
		HeartbeatTTL:    config.HeartbeatTTL,
		JanitorInterval: config.JanitorInterval,
		Storage:         nil, // In-memory for now
	}

	embeddedCP := controlplane.NewEmbeddedControlPlane(controlPlaneConfig)

	// Create orchestrator with control plane endpoint
	var orch orchestrator.Orchestrator
	controlPlaneAddr := embeddedCP.GetEndpoint()

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

// Run starts the pipeline with embedded control plane
func (r *PipelineRunner) Run() error {
	logger.Info("Starting pipeline with embedded control plane",
		zap.String("pipeline", r.pipeline.Metadata.Name),
		zap.String("orchestrator", r.config.OrchestratorType),
		zap.Int("control_plane_port", r.config.ControlPlanePort))

	// Start embedded control plane first
	if err := r.controlPlane.Start(r.ctx); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Wait for control plane to be ready
	if err := r.waitForControlPlaneReady(); err != nil {
		r.controlPlane.Stop()
		return fmt.Errorf("control plane not ready: %w", err)
	}

	// Start components in dependency order
	if err := r.startComponents(); err != nil {
		r.controlPlane.Stop()
		return err
	}

	// Wait for all components to register
	if err := r.waitForComponentRegistration(); err != nil {
		r.controlPlane.Stop()
		return err
	}

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
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal")
	case <-r.ctx.Done():
		logger.Info("Context cancelled")
	}

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
		if err := r.controlPlane.WaitForComponent(componentID, 30*time.Second); err != nil {
			return fmt.Errorf("component %s failed to register: %w", componentID, err)
		}
		logger.Info("Component registered successfully", zap.String("component", componentID))
	}

	return nil
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
		if service.IsHealthy {
			healthyCount++
		} else {
			unhealthyComponents = append(unhealthyComponents, service.ServiceId)
		}
	}

	logger.Info("Pipeline status",
		zap.Int("total_components", len(services)),
		zap.Int("healthy_components", healthyCount),
		zap.Strings("unhealthy_components", unhealthyComponents))

	// Log component metrics if available
	for _, service := range services {
		if len(service.Metrics) > 0 {
			logger.Debug("Component metrics",
				zap.String("component", service.ServiceId),
				zap.Any("metrics", service.Metrics))
		}
	}
}

// shutdown gracefully shuts down the pipeline and control plane
func (r *PipelineRunner) shutdown() error {
	logger.Info("Shutting down pipeline and control plane")

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

	// Stop control plane
	if err := r.controlPlane.Stop(); err != nil {
		logger.Error("Error stopping control plane", zap.Error(err))
	}

	logger.Info("Pipeline and control plane stopped successfully")
	return nil
}

// GetControlPlaneEndpoint returns the control plane endpoint
func (r *PipelineRunner) GetControlPlaneEndpoint() string {
	return r.controlPlane.GetEndpoint()
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
		if !service.IsHealthy {
			return false
		}
	}

	return true
}

// waitForControlPlaneReady waits for the control plane to be ready
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
			if r.controlPlane.IsStarted() {
				// Try a simple health check by getting service list
				_, err := r.controlPlane.GetServiceList()
				if err == nil {
					logger.Debug("Control plane is ready")
					return nil
				}
				logger.Debug("Control plane not ready yet", zap.Error(err))
			}
		}
	}
}