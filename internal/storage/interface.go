package storage

import (
	"context"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
)

// ServiceInfo represents the complete information about a registered service
type ServiceInfo struct {
	// Service registration information
	Info *flowctlv1.ComponentInfo
	// Current service status
	Status *flowctlv1.ComponentStatusResponse
	// Last time the service was seen (sent a heartbeat)
	LastSeen time.Time
}

// PipelineRunInfo represents the complete information about a pipeline run
type PipelineRunInfo struct {
	Run *flowctlpb.PipelineRun
}

// ServiceStorage defines the interface for persistent storage of service registry
type ServiceStorage interface {
	// Open initializes the storage and makes it ready for use
	Open() error
	
	// Close closes the storage and releases any resources
	Close() error
	
	// RegisterService stores a new service in the registry
	RegisterService(ctx context.Context, service *ServiceInfo) error
	
	// UpdateService updates an existing service in the registry
	UpdateService(ctx context.Context, serviceID string, updater func(*ServiceInfo) error) error
	
	// GetService retrieves a service by its ID
	GetService(ctx context.Context, serviceID string) (*ServiceInfo, error)
	
	// ListServices retrieves all services in the registry
	ListServices(ctx context.Context) ([]*ServiceInfo, error)
	
	// DeleteService removes a service from the registry
	DeleteService(ctx context.Context, serviceID string) error

	// UnregisterService removes a service from the registry (alias for DeleteService)
	UnregisterService(ctx context.Context, serviceID string) error

	// WithTransaction executes the given function within a transaction
	// The transaction is committed if the function returns nil, or rolled back if it returns an error
	WithTransaction(ctx context.Context, fn func(txn Transaction) error) error

	// Pipeline run management methods

	// CreatePipelineRun stores a new pipeline run in the registry
	CreatePipelineRun(ctx context.Context, run *PipelineRunInfo) error

	// UpdatePipelineRun updates an existing pipeline run in the registry
	UpdatePipelineRun(ctx context.Context, runID string, updater func(*PipelineRunInfo) error) error

	// GetPipelineRun retrieves a pipeline run by its ID
	GetPipelineRun(ctx context.Context, runID string) (*PipelineRunInfo, error)

	// ListPipelineRuns retrieves pipeline runs, optionally filtered by pipeline name and status
	ListPipelineRuns(ctx context.Context, pipelineName string, status flowctlpb.RunStatus, limit int32) ([]*PipelineRunInfo, error)

	// DeletePipelineRun removes a pipeline run from the registry
	DeletePipelineRun(ctx context.Context, runID string) error
}

// Transaction represents a storage transaction
type Transaction interface {
	// RegisterService stores a new service in the registry within a transaction
	RegisterService(service *ServiceInfo) error
	
	// UpdateService updates an existing service in the registry within a transaction
	UpdateService(serviceID string, updater func(*ServiceInfo) error) error
	
	// GetService retrieves a service by its ID within a transaction
	GetService(serviceID string) (*ServiceInfo, error)
	
	// ListServices retrieves all services in the registry within a transaction
	ListServices() ([]*ServiceInfo, error)
	
	// DeleteService removes a service from the registry within a transaction
	DeleteService(serviceID string) error
}

// ErrServiceNotFound is returned when a service with the specified ID is not found
type ErrServiceNotFound struct {
	ServiceID string
}

// Error implements the error interface
func (e ErrServiceNotFound) Error() string {
	return "service not found: " + e.ServiceID
}

// ErrPipelineRunNotFound is returned when a pipeline run with the specified ID is not found
type ErrPipelineRunNotFound struct {
	RunID string
}

// Error implements the error interface
func (e ErrPipelineRunNotFound) Error() string {
	return "pipeline run not found: " + e.RunID
}

// IsNotFound returns true if the error is ErrServiceNotFound or ErrPipelineRunNotFound
func IsNotFound(err error) bool {
	_, okService := err.(ErrServiceNotFound)
	_, okRun := err.(ErrPipelineRunNotFound)
	return okService || okRun
}