package storage

import (
	"context"
	"time"

	pb "github.com/withobsrvr/flowctl/proto"
)

// ServiceInfo represents the complete information about a registered service
type ServiceInfo struct {
	// Service registration information
	Info *pb.ServiceInfo
	// Current service status
	Status *pb.ServiceStatus
	// Last time the service was seen (sent a heartbeat)
	LastSeen time.Time
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
	
	// WithTransaction executes the given function within a transaction
	// The transaction is committed if the function returns nil, or rolled back if it returns an error
	WithTransaction(ctx context.Context, fn func(txn Transaction) error) error
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

// IsNotFound returns true if the error is ErrServiceNotFound
func IsNotFound(err error) bool {
	_, ok := err.(ErrServiceNotFound)
	return ok
}