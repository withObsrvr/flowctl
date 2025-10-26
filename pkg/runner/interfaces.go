package runner

import (
	"context"
	"io"
)

// DockerClient interface for Docker operations
type DockerClient interface {
	// CreateContainer creates a new container
	CreateContainer(ctx context.Context, config *ContainerConfig) (containerID string, err error)
	
	// StartContainer starts a created container
	StartContainer(ctx context.Context, containerID string) error
	
	// StopContainer stops a running container
	StopContainer(ctx context.Context, containerID string) error
	
	// WaitContainer waits for a container to exit
	WaitContainer(ctx context.Context, containerID string) (exitCode int, err error)
	
	// InspectContainer gets container information
	InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error)
	
	// GetLogs retrieves container logs
	GetLogs(ctx context.Context, containerID string, follow bool) (LogStream, error)
}

// ControlPlaneClient interface for control plane operations
type ControlPlaneClient interface {
	// RegisterPipeline registers a pipeline with the control plane
	RegisterPipeline(ctx context.Context, pipelineID string, metadata map[string]string) error
	
	// IsServiceRegistered checks if a service is registered
	IsServiceRegistered(ctx context.Context, serviceID string) (bool, error)
	
	// SendHeartbeat sends a heartbeat for a service
	SendHeartbeat(ctx context.Context, serviceID string, metrics map[string]float64) error
}

// HealthChecker interface for health monitoring
type HealthChecker interface {
	// Start begins health monitoring
	Start(ctx context.Context)
	
	// IsHealthy checks current health status
	IsHealthy() bool
}

// LogStream interface for reading logs
type LogStream interface {
	io.ReadCloser
}

// ContainerConfig represents container configuration
type ContainerConfig struct {
	Name          string
	Image         string
	Command       []string
	Environment   []string
	Volumes       []VolumeMount
	Ports         []PortMapping
	NetworkMode   string
	RestartPolicy string
	Labels        map[string]string
}

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// PortMapping represents a port mapping configuration
type PortMapping struct {
	ContainerPort int
	HostPort      int
	Protocol      string
}

// ContainerInfo represents container information
type ContainerInfo struct {
	ID       string
	State    ContainerState
	Config   ContainerConfig
	Created  string
}

// ContainerState represents container state
type ContainerState struct {
	Status     string
	Running    bool
	ExitCode   int
	StartedAt  string
	FinishedAt string
	Error      string
}