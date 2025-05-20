package storage

import (
	"context"
	"sync"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// MemoryStorage is an in-memory implementation of ServiceStorage for testing
type MemoryStorage struct {
	mu       sync.RWMutex
	services map[string]*ServiceInfo
}

// NewMemoryStorage creates a new in-memory storage for testing
func NewMemoryStorage() ServiceStorage {
	return &MemoryStorage{
		services: make(map[string]*ServiceInfo),
	}
}

// Open initializes the storage
func (s *MemoryStorage) Open() error {
	logger.Debug("Opening memory storage")
	return nil
}

// Close closes the storage
func (s *MemoryStorage) Close() error {
	logger.Debug("Closing memory storage")
	return nil
}

// RegisterService stores a new service in the registry
func (s *MemoryStorage) RegisterService(ctx context.Context, service *ServiceInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("Registering service in memory", zap.String("id", service.Info.ServiceId))
	s.services[service.Info.ServiceId] = service
	return nil
}

// UpdateService updates an existing service in the registry
func (s *MemoryStorage) UpdateService(ctx context.Context, serviceID string, updater func(*ServiceInfo) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[serviceID]
	if !ok {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	return updater(service)
}

// GetService retrieves a service by its ID
func (s *MemoryStorage) GetService(ctx context.Context, serviceID string) (*ServiceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[serviceID]
	if !ok {
		return nil, ErrServiceNotFound{ServiceID: serviceID}
	}

	return service, nil
}

// ListServices retrieves all services in the registry
func (s *MemoryStorage) ListServices(ctx context.Context) ([]*ServiceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]*ServiceInfo, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}

	return services, nil
}

// DeleteService removes a service from the registry
func (s *MemoryStorage) DeleteService(ctx context.Context, serviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.services[serviceID]; !ok {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	delete(s.services, serviceID)
	return nil
}

// WithTransaction executes the given function within a transaction
func (s *MemoryStorage) WithTransaction(ctx context.Context, fn func(txn Transaction) error) error {
	return fn(&memoryTransaction{storage: s})
}

// memoryTransaction implements the Transaction interface for memory storage
type memoryTransaction struct {
	storage *MemoryStorage
}

// RegisterService stores a new service in the registry within a transaction
func (t *memoryTransaction) RegisterService(service *ServiceInfo) error {
	t.storage.services[service.Info.ServiceId] = service
	return nil
}

// UpdateService updates an existing service in the registry within a transaction
func (t *memoryTransaction) UpdateService(serviceID string, updater func(*ServiceInfo) error) error {
	service, ok := t.storage.services[serviceID]
	if !ok {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	return updater(service)
}

// GetService retrieves a service by its ID within a transaction
func (t *memoryTransaction) GetService(serviceID string) (*ServiceInfo, error) {
	service, ok := t.storage.services[serviceID]
	if !ok {
		return nil, ErrServiceNotFound{ServiceID: serviceID}
	}

	return service, nil
}

// ListServices retrieves all services in the registry within a transaction
func (t *memoryTransaction) ListServices() ([]*ServiceInfo, error) {
	services := make([]*ServiceInfo, 0, len(t.storage.services))
	for _, service := range t.storage.services {
		services = append(services, service)
	}

	return services, nil
}

// DeleteService removes a service from the registry within a transaction
func (t *memoryTransaction) DeleteService(serviceID string) error {
	if _, ok := t.storage.services[serviceID]; !ok {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	delete(t.storage.services, serviceID)
	return nil
}