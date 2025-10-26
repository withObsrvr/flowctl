package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"github.com/withobsrvr/flowctl/pkg/runner"
	"go.uber.org/zap"
)

// DockerOrchestrator manages components as Docker containers
type DockerOrchestrator struct {
	controlPlaneEndpoint string
	pipelines            map[string]*runner.PipelineRunner
	dockerClient         runner.DockerClient
	controlPlaneClient   runner.ControlPlaneClient
	mu                   sync.RWMutex
	logger               *zap.Logger
}

// NewDockerOrchestrator creates a new Docker orchestrator
func NewDockerOrchestrator(controlPlaneEndpoint string) (*DockerOrchestrator, error) {
	log := logger.GetLogger().Named("docker-orchestrator")
	
	// Create Docker client
	dockerClient, err := runner.NewDockerClient(log)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	
	// Create control plane client
	controlPlaneClient, err := runner.NewControlPlaneClient(controlPlaneEndpoint, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create control plane client: %w", err)
	}
	
	return &DockerOrchestrator{
		controlPlaneEndpoint: controlPlaneEndpoint,
		pipelines:            make(map[string]*runner.PipelineRunner),
		dockerClient:         dockerClient,
		controlPlaneClient:   controlPlaneClient,
		logger:               log,
	}, nil
}

// GetType returns the orchestrator type
func (o *DockerOrchestrator) GetType() string {
	return TypeDocker
}

// StartComponent starts a component as a Docker container
func (o *DockerOrchestrator) StartComponent(ctx context.Context, component *Component) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	// Check if component is already running
	if _, exists := o.pipelines[component.ID]; exists {
		return fmt.Errorf("component %s is already running", component.ID)
	}
	
	o.logger.Info("Starting Docker component",
		zap.String("id", component.ID),
		zap.String("type", component.Type),
		zap.String("image", component.Image))
	
	// Convert orchestrator component to model component
	modelComponent := o.convertToModelComponent(component)
	
	// Add control plane endpoint to environment if it's a pipeline type
	if component.Type == "pipeline" {
		if modelComponent.Env == nil {
			modelComponent.Env = make(map[string]string)
		}
		// Only set FLOWCTL_ENDPOINT if not already set
		if _, hasEndpoint := modelComponent.Env["FLOWCTL_ENDPOINT"]; !hasEndpoint {
			// Check if the pipeline wants flowctl integration
			if _, enableFlowctl := modelComponent.Env["ENABLE_FLOWCTL"]; enableFlowctl {
				modelComponent.Env["FLOWCTL_ENDPOINT"] = o.controlPlaneEndpoint
			}
		}
	}
	
	// Create pipeline runner for the component
	pipelineRunner := runner.NewPipelineRunner(modelComponent, o.dockerClient, o.controlPlaneClient, o.logger)
	
	// Start the component
	if err := pipelineRunner.Start(ctx); err != nil {
		return fmt.Errorf("failed to start component %s: %w", component.ID, err)
	}
	
	// Store the runner
	o.pipelines[component.ID] = pipelineRunner
	
	return nil
}

// StopComponent stops a component by ID
func (o *DockerOrchestrator) StopComponent(ctx context.Context, componentID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	runner, exists := o.pipelines[componentID]
	if !exists {
		return &ComponentNotFoundError{ComponentID: componentID}
	}
	
	// Stop the component
	if err := runner.Stop(ctx); err != nil {
		return &OrchestratorError{
			ComponentID: componentID,
			Operation:   "stop",
			Err:         err,
		}
	}
	
	delete(o.pipelines, componentID)
	return nil
}

// StopAll stops all managed components
func (o *DockerOrchestrator) StopAll(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	var errors []error
	for id, runner := range o.pipelines {
		if err := runner.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop component %s: %w", id, err))
		}
	}
	
	// Clear all pipelines
	o.pipelines = make(map[string]*runner.PipelineRunner)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping components: %v", errors)
	}
	
	return nil
}

// GetStatus returns the status of a specific component
func (o *DockerOrchestrator) GetStatus(ctx context.Context, componentID string) (*ComponentStatus, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	runner, exists := o.pipelines[componentID]
	if !exists {
		return nil, &ComponentNotFoundError{ComponentID: componentID}
	}
	
	// For now, return a basic status
	// TODO: Implement proper status retrieval from Docker
	status := &ComponentStatus{
		ID:        componentID,
		Status:    StatusRunning,
		StartTime: time.Now(), // TODO: Get actual start time
		Metadata: map[string]string{
			"type":  "docker",
			"image": runner.Pipeline.Image,
		},
	}
	
	return status, nil
}

// GetAllStatus returns the status of all managed components
func (o *DockerOrchestrator) GetAllStatus(ctx context.Context) (map[string]*ComponentStatus, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	statuses := make(map[string]*ComponentStatus)
	for id := range o.pipelines {
		status, err := o.GetStatus(ctx, id)
		if err != nil {
			continue
		}
		statuses[id] = status
	}
	
	return statuses, nil
}

// IsHealthy checks if a component is healthy
func (o *DockerOrchestrator) IsHealthy(ctx context.Context, componentID string) (bool, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	_, exists := o.pipelines[componentID]
	if !exists {
		return false, &ComponentNotFoundError{ComponentID: componentID}
	}
	
	// TODO: Implement actual health checking
	return true, nil
}

// WaitForHealthy waits for a component to become healthy
func (o *DockerOrchestrator) WaitForHealthy(ctx context.Context, componentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for component %s to become healthy", componentID)
			}
			
			healthy, err := o.IsHealthy(ctx, componentID)
			if err != nil {
				return err
			}
			
			if healthy {
				return nil
			}
		}
	}
}

// GetLogs retrieves logs for a component
func (o *DockerOrchestrator) GetLogs(ctx context.Context, componentID string, follow bool) (LogStream, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	runner, exists := o.pipelines[componentID]
	if !exists {
		return nil, &ComponentNotFoundError{ComponentID: componentID}
	}
	
	return runner.GetLogs(ctx, follow)
}

// convertToModelComponent converts an orchestrator component to a model component
func (o *DockerOrchestrator) convertToModelComponent(orchComp *Component) model.Component {
	// Convert volumes
	volumes := make([]model.Volume, 0, len(orchComp.Volumes))
	for _, v := range orchComp.Volumes {
		volumes = append(volumes, model.Volume{
			HostPath:      v.HostPath,
			ContainerPath: v.ContainerPath,
			ReadOnly:      v.ReadOnly,
		})
	}
	
	// Convert ports
	ports := make([]model.Port, 0, len(orchComp.Ports))
	for _, p := range orchComp.Ports {
		ports = append(ports, model.Port{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}
	
	// Use restart policy from component, with defaults based on type
	restartPolicy := orchComp.RestartPolicy
	if restartPolicy == "" {
		// Default restart policies
		if orchComp.Type == "pipeline" {
			// Pipelines typically run to completion and should not restart
			restartPolicy = "no"
		} else {
			// Other components (sources, sinks, processors) should restart on failure
			restartPolicy = "unless-stopped"
		}
	}
	
	return model.Component{
		ID:            orchComp.ID,
		Type:          orchComp.Type,
		Image:         orchComp.Image,
		Command:       orchComp.Command,
		Args:          orchComp.Args,
		Env:           orchComp.Environment,
		Volumes:       volumes,
		Ports:         ports,
		RestartPolicy: restartPolicy,
	}
}