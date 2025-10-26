package runner

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ContainerHealthMonitor monitors the health of a pipeline container
type ContainerHealthMonitor struct {
	containerID  string
	dockerClient DockerClient
	controlPlane ControlPlaneClient
	serviceID    string
	logger       *zap.Logger
	healthy      bool
}

// NewContainerHealthMonitor creates a new health monitor
func NewContainerHealthMonitor(containerID string, dockerClient DockerClient, controlPlane ControlPlaneClient, serviceID string, logger *zap.Logger) *ContainerHealthMonitor {
	return &ContainerHealthMonitor{
		containerID:  containerID,
		dockerClient: dockerClient,
		controlPlane: controlPlane,
		serviceID:    serviceID,
		logger:       logger,
		healthy:      true,
	}
}

// Start begins monitoring container health
func (m *ContainerHealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	m.logger.Info("Starting health monitoring for pipeline",
		zap.String("service_id", m.serviceID),
		zap.String("container_id", m.containerID))
	
	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Stopping health monitoring",
				zap.String("service_id", m.serviceID))
			return
		case <-ticker.C:
			if err := m.checkHealth(ctx); err != nil {
				m.logger.Error("Health check failed",
					zap.String("service_id", m.serviceID),
					zap.Error(err))
				m.healthy = false
			}
		}
	}
}

// IsHealthy returns the current health status
func (m *ContainerHealthMonitor) IsHealthy() bool {
	return m.healthy
}

// checkHealth performs a health check
func (m *ContainerHealthMonitor) checkHealth(ctx context.Context) error {
	// Check container status
	info, err := m.dockerClient.InspectContainer(ctx, m.containerID)
	if err != nil {
		return err
	}
	
	// Build metrics
	metrics := map[string]float64{
		"container_running": boolToFloat64(info.State.Running),
		"exit_code":        float64(info.State.ExitCode),
	}
	
	// Update health status
	m.healthy = info.State.Running && info.State.ExitCode == 0
	
	// Send heartbeat to control plane
	if err := m.controlPlane.SendHeartbeat(ctx, m.serviceID, metrics); err != nil {
		m.logger.Warn("Failed to send heartbeat",
			zap.String("service_id", m.serviceID),
			zap.Error(err))
	}
	
	return nil
}

// boolToFloat64 converts a boolean to float64 (1.0 for true, 0.0 for false)
func boolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}