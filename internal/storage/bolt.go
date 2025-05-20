package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	// DefaultBoltFilePath is the default path for the BoltDB file
	DefaultBoltFilePath = "flowctl-service-registry.db"

	// DefaultBoltFileMode is the default file mode for the BoltDB file
	DefaultBoltFileMode = 0600

	// DefaultBoltTimeout is the default timeout for BoltDB operations
	DefaultBoltTimeout = 1 * time.Second
)

// BucketName is the name of the bucket where service information is stored
var serviceBucket = []byte("services")

// BoltDBStorage implements the ServiceStorage interface using BoltDB
type BoltDBStorage struct {
	db      *bolt.DB
	path    string
	options *BoltOptions
}

// BoltOptions configures the BoltDB storage
type BoltOptions struct {
	// Path to the BoltDB file
	Path string
	// File mode for the BoltDB file
	FileMode os.FileMode
	// Timeout for BoltDB operations
	Timeout time.Duration
}

// NewBoltDBStorage creates a new BoltDBStorage with the given options
func NewBoltDBStorage(opts *BoltOptions) *BoltDBStorage {
	if opts == nil {
		opts = &BoltOptions{}
	}

	// Set default options if not provided
	if opts.Path == "" {
		opts.Path = DefaultBoltFilePath
	}
	if opts.FileMode == 0 {
		opts.FileMode = DefaultBoltFileMode
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultBoltTimeout
	}

	return &BoltDBStorage{
		path: opts.Path,
		options: opts,
	}
}

// Open initializes the BoltDB database
func (s *BoltDBStorage) Open() error {
	logger.Info("Opening BoltDB database", zap.String("path", s.path))

	// Make sure the directory exists
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("failed to create directory for database: %w", err)
	}

	// Open the database
	opts := &bolt.Options{Timeout: DefaultBoltTimeout}
	if s.options != nil && s.options.Timeout > 0 {
		opts.Timeout = s.options.Timeout
	}
	
	db, err := bolt.Open(s.path, 0600, opts)
	if err != nil {
		return fmt.Errorf("failed to open BoltDB: %w", err)
	}
	s.db = db

	// Initialize the buckets
	err = s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(serviceBucket)
		if err != nil {
			return fmt.Errorf("failed to create services bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		// Close the database if initialization fails
		s.db.Close()
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	logger.Info("BoltDB database opened successfully")
	return nil
}

// Close closes the BoltDB database
func (s *BoltDBStorage) Close() error {
	if s.db != nil {
		logger.Info("Closing BoltDB database")
		return s.db.Close()
	}
	return nil
}

// RegisterService stores a new service in the registry
func (s *BoltDBStorage) RegisterService(ctx context.Context, service *ServiceInfo) error {
	logger.Debug("Registering service", zap.String("id", service.Info.ServiceId))
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.registerServiceTx(tx, service)
	})
}

// registerServiceTx is the transaction version of RegisterService
func (s *BoltDBStorage) registerServiceTx(tx *bolt.Tx, service *ServiceInfo) error {
	b := tx.Bucket(serviceBucket)
	if b == nil {
		return fmt.Errorf("services bucket not found")
	}

	// Serialize the service to JSON
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	// Store the service
	key := []byte(service.Info.ServiceId)
	if err := b.Put(key, data); err != nil {
		return fmt.Errorf("failed to store service: %w", err)
	}

	return nil
}

// UpdateService updates an existing service in the registry
func (s *BoltDBStorage) UpdateService(ctx context.Context, serviceID string, updater func(*ServiceInfo) error) error {
	logger.Debug("Updating service", zap.String("id", serviceID))
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.updateServiceTx(tx, serviceID, updater)
	})
}

// updateServiceTx is the transaction version of UpdateService
func (s *BoltDBStorage) updateServiceTx(tx *bolt.Tx, serviceID string, updater func(*ServiceInfo) error) error {
	b := tx.Bucket(serviceBucket)
	if b == nil {
		return fmt.Errorf("services bucket not found")
	}

	// Get the service
	key := []byte(serviceID)
	data := b.Get(key)
	if data == nil {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	// Deserialize the service
	var service ServiceInfo
	if err := json.Unmarshal(data, &service); err != nil {
		return fmt.Errorf("failed to unmarshal service: %w", err)
	}

	// Apply the updater function
	if err := updater(&service); err != nil {
		return err
	}

	// Serialize the updated service
	updatedData, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal updated service: %w", err)
	}

	// Store the updated service
	if err := b.Put(key, updatedData); err != nil {
		return fmt.Errorf("failed to store updated service: %w", err)
	}

	return nil
}

// GetService retrieves a service by its ID
func (s *BoltDBStorage) GetService(ctx context.Context, serviceID string) (*ServiceInfo, error) {
	logger.Debug("Getting service", zap.String("id", serviceID))
	var service *ServiceInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		service, err = s.getServiceTx(tx, serviceID)
		return err
	})
	return service, err
}

// getServiceTx is the transaction version of GetService
func (s *BoltDBStorage) getServiceTx(tx *bolt.Tx, serviceID string) (*ServiceInfo, error) {
	b := tx.Bucket(serviceBucket)
	if b == nil {
		return nil, fmt.Errorf("services bucket not found")
	}

	// Get the service
	key := []byte(serviceID)
	data := b.Get(key)
	if data == nil {
		return nil, ErrServiceNotFound{ServiceID: serviceID}
	}

	// Deserialize the service
	var service ServiceInfo
	if err := json.Unmarshal(data, &service); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service: %w", err)
	}

	return &service, nil
}

// ListServices retrieves all services in the registry
func (s *BoltDBStorage) ListServices(ctx context.Context) ([]*ServiceInfo, error) {
	logger.Debug("Listing services")
	var services []*ServiceInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		services, err = s.listServicesTx(tx)
		return err
	})
	return services, err
}

// listServicesTx is the transaction version of ListServices
func (s *BoltDBStorage) listServicesTx(tx *bolt.Tx) ([]*ServiceInfo, error) {
	b := tx.Bucket(serviceBucket)
	if b == nil {
		return nil, fmt.Errorf("services bucket not found")
	}

	// Collect all services
	var services []*ServiceInfo
	err := b.ForEach(func(k, v []byte) error {
		var service ServiceInfo
		if err := json.Unmarshal(v, &service); err != nil {
			return fmt.Errorf("failed to unmarshal service: %w", err)
		}
		services = append(services, &service)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return services, nil
}

// DeleteService removes a service from the registry
func (s *BoltDBStorage) DeleteService(ctx context.Context, serviceID string) error {
	logger.Debug("Deleting service", zap.String("id", serviceID))
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.deleteServiceTx(tx, serviceID)
	})
}

// deleteServiceTx is the transaction version of DeleteService
func (s *BoltDBStorage) deleteServiceTx(tx *bolt.Tx, serviceID string) error {
	b := tx.Bucket(serviceBucket)
	if b == nil {
		return fmt.Errorf("services bucket not found")
	}

	// Check if the service exists
	key := []byte(serviceID)
	if b.Get(key) == nil {
		return ErrServiceNotFound{ServiceID: serviceID}
	}

	// Delete the service
	if err := b.Delete(key); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}

// WithTransaction executes the given function within a transaction
func (s *BoltDBStorage) WithTransaction(ctx context.Context, fn func(txn Transaction) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		boltTxn := &boltTransaction{tx: tx, storage: s}
		return fn(boltTxn)
	})
}

// boltTransaction implements the Transaction interface
type boltTransaction struct {
	tx      *bolt.Tx
	storage *BoltDBStorage
}

// RegisterService stores a new service in the registry within a transaction
func (t *boltTransaction) RegisterService(service *ServiceInfo) error {
	return t.storage.registerServiceTx(t.tx, service)
}

// UpdateService updates an existing service in the registry within a transaction
func (t *boltTransaction) UpdateService(serviceID string, updater func(*ServiceInfo) error) error {
	return t.storage.updateServiceTx(t.tx, serviceID, updater)
}

// GetService retrieves a service by its ID within a transaction
func (t *boltTransaction) GetService(serviceID string) (*ServiceInfo, error) {
	return t.storage.getServiceTx(t.tx, serviceID)
}

// ListServices retrieves all services in the registry within a transaction
func (t *boltTransaction) ListServices() ([]*ServiceInfo, error) {
	return t.storage.listServicesTx(t.tx)
}

// DeleteService removes a service from the registry within a transaction
func (t *boltTransaction) DeleteService(serviceID string) error {
	return t.storage.deleteServiceTx(t.tx, serviceID)
}