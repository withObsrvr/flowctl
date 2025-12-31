package storage

import (
	"context"
	"sync"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"go.uber.org/zap"
)

// MemoryStorage is an in-memory implementation of ServiceStorage for testing
type MemoryStorage struct {
	mu           sync.RWMutex
	services     map[string]*ServiceInfo
	pipelineRuns map[string]*PipelineRunInfo
}

// NewMemoryStorage creates a new in-memory storage for testing
func NewMemoryStorage() ServiceStorage {
	return &MemoryStorage{
		services:     make(map[string]*ServiceInfo),
		pipelineRuns: make(map[string]*PipelineRunInfo),
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

	logger.Debug("Registering service in memory", zap.String("id", service.Info.Id))
	s.services[service.Info.Id] = service
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

// UnregisterService removes a service from the registry (alias for DeleteService)
func (s *MemoryStorage) UnregisterService(ctx context.Context, serviceID string) error {
	return s.DeleteService(ctx, serviceID)
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
	t.storage.services[service.Info.Id] = service
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

// Pipeline run storage methods

// CreatePipelineRun stores a new pipeline run in the registry
func (s *MemoryStorage) CreatePipelineRun(ctx context.Context, run *PipelineRunInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Debug("Creating pipeline run in memory", zap.String("run_id", run.Run.RunId))
	s.pipelineRuns[run.Run.RunId] = run
	return nil
}

// UpdatePipelineRun updates an existing pipeline run in the registry
func (s *MemoryStorage) UpdatePipelineRun(ctx context.Context, runID string, updater func(*PipelineRunInfo) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.pipelineRuns[runID]
	if !ok {
		return ErrPipelineRunNotFound{RunID: runID}
	}

	return updater(run)
}

// GetPipelineRun retrieves a pipeline run by its ID
func (s *MemoryStorage) GetPipelineRun(ctx context.Context, runID string) (*PipelineRunInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.pipelineRuns[runID]
	if !ok {
		return nil, ErrPipelineRunNotFound{RunID: runID}
	}

	return run, nil
}

// ListPipelineRuns retrieves pipeline runs, optionally filtered by pipeline name and status
func (s *MemoryStorage) ListPipelineRuns(ctx context.Context, pipelineName string, status flowctlpb.RunStatus, limit int32) ([]*PipelineRunInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]*PipelineRunInfo, 0)
	count := int32(0)

	for _, run := range s.pipelineRuns {
		// Check if we've reached the limit
		if limit > 0 && count >= limit {
			break
		}

		// Apply filters
		if pipelineName != "" && run.Run.PipelineName != pipelineName {
			continue
		}
		if status != flowctlpb.RunStatus_RUN_STATUS_UNKNOWN && run.Run.Status != status {
			continue
		}

		runs = append(runs, run)
		count++
	}

	return runs, nil
}

// DeletePipelineRun removes a pipeline run from the registry
func (s *MemoryStorage) DeletePipelineRun(ctx context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pipelineRuns[runID]; !ok {
		return ErrPipelineRunNotFound{RunID: runID}
	}

	delete(s.pipelineRuns, runID)
	return nil
}