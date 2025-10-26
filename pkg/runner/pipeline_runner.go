package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/withobsrvr/flowctl/internal/model"
	"go.uber.org/zap"
)

// PipelineRunner manages the lifecycle of pipeline components
type PipelineRunner struct {
	// Pipeline configuration
	Pipeline model.Component
	
	// Container management
	containerID   string
	dockerClient  DockerClient
	
	// Control plane integration
	controlPlane  ControlPlaneClient
	
	// Lifecycle management
	ctx           context.Context
	cancel        context.CancelFunc
	logger        *zap.Logger
	healthChecker HealthChecker
}

// NewPipelineRunner creates a new pipeline runner instance
func NewPipelineRunner(pipeline model.Component, dockerClient DockerClient, controlPlane ControlPlaneClient, logger *zap.Logger) *PipelineRunner {
	return &PipelineRunner{
		Pipeline:     pipeline,
		dockerClient: dockerClient,
		controlPlane: controlPlane,
		logger:       logger,
	}
}

// Start starts the pipeline container
func (r *PipelineRunner) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)
	
	r.logger.Info("Starting pipeline component",
		zap.String("id", r.Pipeline.ID),
		zap.String("image", r.Pipeline.Image),
		zap.String("type", r.Pipeline.Type))
	
	// Build container configuration
	config := r.buildContainerConfig()
	if config == nil {
		return fmt.Errorf("failed to build container configuration")
	}
	
	// Create and start container
	containerID, err := r.dockerClient.CreateContainer(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	r.containerID = containerID
	
	if err := r.dockerClient.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	
	r.logger.Info("Pipeline container started",
		zap.String("id", r.Pipeline.ID),
		zap.String("container_id", containerID))
	
	// Handle pipeline registration
	if err := r.handleRegistration(ctx); err != nil {
		return fmt.Errorf("failed to handle registration: %w", err)
	}
	
	// Start health monitoring
	r.healthChecker = NewContainerHealthMonitor(r.containerID, r.dockerClient, r.controlPlane, r.Pipeline.ID, r.logger)
	go r.healthChecker.Start(r.ctx)
	
	return nil
}

// Stop stops the pipeline container
func (r *PipelineRunner) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	
	if r.containerID == "" {
		return nil
	}
	
	r.logger.Info("Stopping pipeline container",
		zap.String("id", r.Pipeline.ID),
		zap.String("container_id", r.containerID))
	
	// Stop container with timeout
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	if err := r.dockerClient.StopContainer(stopCtx, r.containerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	
	return nil
}

// Wait waits for the pipeline to complete or context cancellation
func (r *PipelineRunner) Wait(ctx context.Context) error {
	if r.containerID == "" {
		return fmt.Errorf("container not started")
	}
	
	// Wait for container to exit
	exitCode, err := r.dockerClient.WaitContainer(ctx, r.containerID)
	if err != nil {
		return fmt.Errorf("error waiting for container: %w", err)
	}
	
	if exitCode != 0 {
		return fmt.Errorf("container exited with non-zero code: %d", exitCode)
	}
	
	return nil
}

// GetLogs retrieves logs from the pipeline container
func (r *PipelineRunner) GetLogs(ctx context.Context, follow bool) (LogStream, error) {
	if r.containerID == "" {
		return nil, fmt.Errorf("container not started")
	}
	
	return r.dockerClient.GetLogs(ctx, r.containerID, follow)
}

// buildContainerConfig builds the container configuration
func (r *PipelineRunner) buildContainerConfig() *ContainerConfig {
	// Combine command and args
	var cmd []string
	if len(r.Pipeline.Command) > 0 {
		cmd = append(cmd, r.Pipeline.Command...)
	}
	if len(r.Pipeline.Args) > 0 {
		cmd = append(cmd, r.Pipeline.Args...)
	}
	
	// Build environment variables with expansion
	envHandler := NewEnvHandler(r.logger)
	
	// Validate environment first
	if err := envHandler.ValidateEnvironment(r.Pipeline.Env); err != nil {
		r.logger.Warn("Environment validation warning",
			zap.String("pipeline", r.Pipeline.ID),
			zap.Error(err))
	}
	
	// Process and expand environment variables
	env, err := envHandler.ProcessEnvironment(r.Pipeline.Env)
	if err != nil {
		r.logger.Error("Failed to process environment",
			zap.String("pipeline", r.Pipeline.ID),
			zap.Error(err))
		// Continue with empty environment
		env = []string{}
	}
	
	// Build volume mounts with proper path resolution
	volumeHandler := NewVolumeHandler(r.logger)
	
	// Convert model volumes to handler volumes
	handlerVolumes := make([]Volume, 0, len(r.Pipeline.Volumes))
	for _, v := range r.Pipeline.Volumes {
		handlerVolumes = append(handlerVolumes, Volume{
			Name:          v.Name,
			HostPath:      v.HostPath,
			ContainerPath: v.ContainerPath,
			ReadOnly:      v.ReadOnly,
		})
	}
	
	// Process volumes with validation and path resolution
	r.logger.Debug("Processing volumes",
		zap.String("pipeline", r.Pipeline.ID),
		zap.Int("count", len(handlerVolumes)))
	
	volumes, err := volumeHandler.ProcessVolumes(handlerVolumes)
	if err != nil {
		r.logger.Error("Failed to process volumes",
			zap.String("pipeline", r.Pipeline.ID),
			zap.Error(err))
		return nil
	}
	
	r.logger.Debug("Processed volumes",
		zap.String("pipeline", r.Pipeline.ID),
		zap.Int("count", len(volumes)))
	
	// Process secrets if any
	if len(r.Pipeline.Secrets) > 0 {
		secretHandler := NewSecretHandler(r.logger)
		
		// Convert model secrets to handler secrets
		handlerSecrets := make([]SecretMount, 0, len(r.Pipeline.Secrets))
		for _, s := range r.Pipeline.Secrets {
			handlerSecrets = append(handlerSecrets, SecretMount{
				Name:          s.Name,
				HostPath:      s.HostPath,
				ContainerPath: s.ContainerPath,
				EnvVar:        s.EnvVar,
				Mode:          s.Mode,
				Type:          s.Type,
				Required:      s.Required,
				Labels:        s.Labels,
			})
		}
		
		// Process secrets to get volumes and env vars
		secretVolumes, secretEnvs, err := secretHandler.ProcessSecrets(handlerSecrets)
		if err != nil {
			r.logger.Error("Failed to process secrets",
				zap.String("pipeline", r.Pipeline.ID),
				zap.Error(err))
			// Fail if secrets are required
			return nil
		}
		
		// Add secret volumes
		volumes = append(volumes, secretVolumes...)
		
		// Add secret environment variables
		for k, v := range secretEnvs {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	
	// Build port mappings
	ports := make([]PortMapping, 0, len(r.Pipeline.Ports))
	for _, p := range r.Pipeline.Ports {
		ports = append(ports, PortMapping{
			ContainerPort: p.ContainerPort,
			HostPort:      p.HostPort,
			Protocol:      p.Protocol,
		})
	}
	
	return &ContainerConfig{
		Name:          fmt.Sprintf("flowctl-pipeline-%s", r.Pipeline.ID),
		Image:         r.Pipeline.Image,
		Command:       cmd,
		Environment:   env,
		Volumes:       volumes,
		Ports:         ports,
		NetworkMode:   "host", // Use host network for control plane connectivity
		RestartPolicy: r.Pipeline.RestartPolicy,
		Labels: map[string]string{
			"flowctl.component":    "pipeline",
			"flowctl.pipeline.id":  r.Pipeline.ID,
			"flowctl.pipeline.type": r.Pipeline.Type,
		},
	}
}

// handleRegistration handles pipeline registration with the control plane
func (r *PipelineRunner) handleRegistration(ctx context.Context) error {
	// Check if pipeline will self-register
	if _, hasFlowctlEndpoint := r.Pipeline.Env["FLOWCTL_ENDPOINT"]; hasFlowctlEndpoint {
		// Pipeline will self-register, wait for it
		r.logger.Info("Pipeline will self-register with control plane",
			zap.String("id", r.Pipeline.ID))
		return r.waitForPipelineRegistration(ctx, 30*time.Second)
	}
	
	// Pipeline won't self-register, so we register it
	r.logger.Info("Registering pipeline with control plane",
		zap.String("id", r.Pipeline.ID))
	return r.registerPipelineWithControlPlane(ctx)
}

// waitForPipelineRegistration waits for the pipeline to self-register
func (r *PipelineRunner) waitForPipelineRegistration(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pipeline %s registration", r.Pipeline.ID)
		case <-ticker.C:
			// Check if pipeline has registered
			if registered, err := r.controlPlane.IsServiceRegistered(ctx, r.Pipeline.ID); err == nil && registered {
				r.logger.Info("Pipeline successfully self-registered",
					zap.String("id", r.Pipeline.ID))
				return nil
			}
		}
	}
}

// registerPipelineWithControlPlane registers the pipeline on its behalf
func (r *PipelineRunner) registerPipelineWithControlPlane(ctx context.Context) error {
	metadata := map[string]string{
		"pipeline_name":    r.Pipeline.ID,
		"pipeline_image":   r.Pipeline.Image,
		"pipeline_command": strings.Join(r.Pipeline.Command, " "),
		"pipeline_type":    "complete",
		"managed_by":       "flowctl",
	}
	
	// Add container ID to metadata
	if r.containerID != "" {
		metadata["container_id"] = r.containerID
	}
	
	return r.controlPlane.RegisterPipeline(ctx, r.Pipeline.ID, metadata)
}