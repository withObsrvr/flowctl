package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupTestStorage(t *testing.T) (*BoltDBStorage, func()) {
	// Create a temporary directory for the test DB
	dir, err := os.MkdirTemp("", "flowctl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create the storage instance
	dbPath := filepath.Join(dir, "test.db")
	storage := NewBoltDBStorage(&BoltOptions{
		Path: dbPath,
	})

	// Open the storage
	if err := storage.Open(); err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}

	// Return the storage and a cleanup function
	cleanup := func() {
		storage.Close()
		os.RemoveAll(dir)
	}

	return storage, cleanup
}

func createTestService(id string) *ServiceInfo {
	now := time.Now()
	return &ServiceInfo{
		Info: &pb.ServiceInfo{
			ServiceId:       id,
			ServiceType:     pb.ServiceType_SERVICE_TYPE_SOURCE,
			InputEventTypes: []string{},
			OutputEventTypes: []string{"event1", "event2"},
			HealthEndpoint:  "localhost:8080/health",
			MaxInflight:     100,
			Metadata: map[string]string{
				"version": "1.0",
				"owner":   "test",
			},
		},
		Status: &pb.ServiceStatus{
			ServiceId:     id,
			ServiceType:   pb.ServiceType_SERVICE_TYPE_SOURCE,
			IsHealthy:     true,
			LastHeartbeat: timestamppb.New(now),
			Metrics: map[string]float64{
				"requests": 100,
				"errors":   0,
			},
		},
		LastSeen: now,
	}
}

func TestBoltDBStorage_RegisterService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service := createTestService("test-service-1")

	// Register a service
	err := storage.RegisterService(ctx, service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Retrieve the service
	retrieved, err := storage.GetService(ctx, service.Info.ServiceId)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	// Verify service data
	if retrieved.Info.ServiceId != service.Info.ServiceId {
		t.Errorf("Retrieved service ID does not match: got %s, want %s", 
			retrieved.Info.ServiceId, service.Info.ServiceId)
	}
	if retrieved.Status.IsHealthy != service.Status.IsHealthy {
		t.Errorf("Retrieved service health does not match: got %v, want %v", 
			retrieved.Status.IsHealthy, service.Status.IsHealthy)
	}
}

func TestBoltDBStorage_UpdateService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service := createTestService("test-service-2")

	// Register a service
	err := storage.RegisterService(ctx, service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Update the service
	err = storage.UpdateService(ctx, service.Info.ServiceId, func(s *ServiceInfo) error {
		s.Status.IsHealthy = false
		s.Status.Metrics["errors"] = 10
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to update service: %v", err)
	}

	// Retrieve the service
	retrieved, err := storage.GetService(ctx, service.Info.ServiceId)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	// Verify updated data
	if retrieved.Status.IsHealthy != false {
		t.Errorf("Retrieved service health does not match: got %v, want %v", 
			retrieved.Status.IsHealthy, false)
	}
	if retrieved.Status.Metrics["errors"] != 10 {
		t.Errorf("Retrieved service metrics do not match: got %v, want %v", 
			retrieved.Status.Metrics["errors"], 10)
	}
}

func TestBoltDBStorage_ListServices(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Register multiple services
	services := []*ServiceInfo{
		createTestService("test-service-3"),
		createTestService("test-service-4"),
		createTestService("test-service-5"),
	}

	for _, service := range services {
		err := storage.RegisterService(ctx, service)
		if err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}
	}

	// List all services
	retrieved, err := storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}

	// Verify list length
	if len(retrieved) != len(services) {
		t.Errorf("Retrieved services count does not match: got %d, want %d", 
			len(retrieved), len(services))
	}

	// Verify each service is in the list
	serviceMap := make(map[string]*ServiceInfo)
	for _, s := range retrieved {
		serviceMap[s.Info.ServiceId] = s
	}

	for _, service := range services {
		if _, exists := serviceMap[service.Info.ServiceId]; !exists {
			t.Errorf("Service %s not found in retrieved list", service.Info.ServiceId)
		}
	}
}

func TestBoltDBStorage_DeleteService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service := createTestService("test-service-6")

	// Register a service
	err := storage.RegisterService(ctx, service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Delete the service
	err = storage.DeleteService(ctx, service.Info.ServiceId)
	if err != nil {
		t.Fatalf("Failed to delete service: %v", err)
	}

	// Try to retrieve the deleted service
	_, err = storage.GetService(ctx, service.Info.ServiceId)
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error after deletion, got: %v", err)
	}
}

func TestBoltDBStorage_GetNonExistentService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Try to retrieve a non-existent service
	_, err := storage.GetService(ctx, "non-existent-service")
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error for non-existent service, got: %v", err)
	}
}

func TestBoltDBStorage_UpdateNonExistentService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Try to update a non-existent service
	err := storage.UpdateService(ctx, "non-existent-service", func(s *ServiceInfo) error {
		return nil
	})
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error when updating non-existent service, got: %v", err)
	}
}

func TestBoltDBStorage_DeleteNonExistentService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Try to delete a non-existent service
	err := storage.DeleteService(ctx, "non-existent-service")
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error when deleting non-existent service, got: %v", err)
	}
}

func TestBoltDBStorage_WithTransaction(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service1 := createTestService("txn-service-1")
	service2 := createTestService("txn-service-2")

	// Use a transaction to register multiple services
	err := storage.WithTransaction(ctx, func(txn Transaction) error {
		if err := txn.RegisterService(service1); err != nil {
			return err
		}
		if err := txn.RegisterService(service2); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify both services were registered
	services, err := storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}

	if len(services) != 2 {
		t.Errorf("Expected 2 services after transaction, got %d", len(services))
	}

	// Verify rollback on error
	err = storage.WithTransaction(ctx, func(txn Transaction) error {
		// This should succeed
		if err := txn.UpdateService(service1.Info.ServiceId, func(s *ServiceInfo) error {
			s.Status.IsHealthy = false
			return nil
		}); err != nil {
			return err
		}

		// This should fail and cause rollback
		return txn.DeleteService("non-existent-service")
	})
	if err == nil {
		t.Fatalf("Expected transaction to fail, but it succeeded")
	}

	// Verify service1 was not updated due to rollback
	retrieved, err := storage.GetService(ctx, service1.Info.ServiceId)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if !retrieved.Status.IsHealthy {
		t.Errorf("Service was updated despite transaction rollback")
	}
}