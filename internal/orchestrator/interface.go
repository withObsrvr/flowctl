package orchestrator

import (
	"context"
	"fmt"
	"time"
)

// Component represents a pipeline component that can be orchestrated
type Component struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`
	Name          string            `json:"name"`
	Image         string            `json:"image,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	Ports         []Port            `json:"ports,omitempty"`
	Volumes       []VolumeMount     `json:"volumes,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
}

// VolumeMount represents a volume mount for a component
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"readonly"`
}

// Port represents a port mapping for a component
type Port struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// HealthCheck represents health check configuration
type HealthCheck struct {
	Command     []string      `json:"command,omitempty"`
	HTTP        *HTTPCheck    `json:"http,omitempty"`
	Interval    time.Duration `json:"interval,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	Retries     int           `json:"retries,omitempty"`
	StartPeriod time.Duration `json:"start_period,omitempty"`
}

// HTTPCheck represents HTTP health check configuration
type HTTPCheck struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

// ComponentStatus represents the status of a component
type ComponentStatus struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	PID        int               `json:"pid,omitempty"`
	ContainerID string            `json:"container_id,omitempty"`
	Endpoint   string            `json:"endpoint,omitempty"`
	StartTime  time.Time         `json:"start_time,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Orchestrator defines the interface for component orchestration
type Orchestrator interface {
	// StartComponent starts a single component
	StartComponent(ctx context.Context, component *Component) error

	// StopComponent stops a single component by ID
	StopComponent(ctx context.Context, componentID string) error

	// StopAll stops all managed components
	StopAll(ctx context.Context) error

	// GetStatus returns the status of a specific component
	GetStatus(ctx context.Context, componentID string) (*ComponentStatus, error)

	// GetAllStatus returns the status of all managed components
	GetAllStatus(ctx context.Context) (map[string]*ComponentStatus, error)

	// IsHealthy checks if a component is healthy
	IsHealthy(ctx context.Context, componentID string) (bool, error)

	// WaitForHealthy waits for a component to become healthy
	WaitForHealthy(ctx context.Context, componentID string, timeout time.Duration) error

	// GetLogs retrieves logs for a component
	GetLogs(ctx context.Context, componentID string, follow bool) (LogStream, error)

	// GetType returns the orchestrator type (e.g., "process", "docker")
	GetType() string
}

// LogStream represents a log stream from a component
type LogStream interface {
	// Read reads log data
	Read(p []byte) (n int, err error)
	
	// Close closes the log stream
	Close() error
}

// OrchestratorError represents errors from the orchestrator
type OrchestratorError struct {
	ComponentID string
	Operation   string
	Err         error
}

func (e *OrchestratorError) Error() string {
	return fmt.Sprintf("orchestrator error for component %s during %s: %v", e.ComponentID, e.Operation, e.Err)
}

func (e *OrchestratorError) Unwrap() error {
	return e.Err
}

// ComponentNotFoundError indicates a component was not found
type ComponentNotFoundError struct {
	ComponentID string
}

func (e *ComponentNotFoundError) Error() string {
	return fmt.Sprintf("component not found: %s", e.ComponentID)
}

// ComponentStatus constants
const (
	StatusUnknown  = "unknown"
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopping = "stopping"
	StatusStopped  = "stopped"
	StatusFailed   = "failed"
	StatusHealthy  = "healthy"
	StatusUnhealthy = "unhealthy"
)

// Orchestrator type constants
const (
	TypeProcess   = "process"
	TypeDocker    = "docker"
	TypeKubernetes = "kubernetes"
)