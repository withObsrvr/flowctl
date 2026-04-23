package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
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

	cleanup := func() {
		storage.Close()
		os.RemoveAll(dir)
	}

	return storage, cleanup
}

func createTestService(id string) *ServiceInfo {
	now := time.Now()
	return &ServiceInfo{
		Info: &flowctlv1.ComponentInfo{
			Id:               id,
			Type:             flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE,
			InputEventTypes:  []string{},
			OutputEventTypes: []string{"event1", "event2"},
			Endpoint:         "localhost:8080",
			Metadata: map[string]string{
				"version": "1.0",
				"owner":   "test",
			},
		},
		Status: &flowctlv1.ComponentStatusResponse{
			Component: &flowctlv1.ComponentInfo{
				Id:   id,
				Type: flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE,
			},
			Status:        flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
			LastHeartbeat: timestamppb.New(now),
			RegisteredAt:  timestamppb.New(now),
			Metrics: map[string]string{
				"requests": "100",
				"errors":   "0",
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

	if err := storage.RegisterService(ctx, service); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	retrieved, err := storage.GetService(ctx, service.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if retrieved.Info.Id != service.Info.Id {
		t.Errorf("Retrieved service ID does not match: got %s, want %s", retrieved.Info.Id, service.Info.Id)
	}
	if retrieved.Status.Status != service.Status.Status {
		t.Errorf("Retrieved service health does not match: got %v, want %v", retrieved.Status.Status, service.Status.Status)
	}
}

func TestBoltDBStorage_UpdateService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service := createTestService("test-service-2")

	if err := storage.RegisterService(ctx, service); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	err := storage.UpdateService(ctx, service.Info.Id, func(s *ServiceInfo) error {
		s.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
		s.Status.Metrics["errors"] = "10"
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to update service: %v", err)
	}

	retrieved, err := storage.GetService(ctx, service.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if retrieved.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Errorf("Retrieved service health does not match: got %v, want %v", retrieved.Status.Status, flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY)
	}
	if retrieved.Status.Metrics["errors"] != "10" {
		t.Errorf("Retrieved service metrics do not match: got %v, want %v", retrieved.Status.Metrics["errors"], "10")
	}
}

func TestBoltDBStorage_ListServices(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	services := []*ServiceInfo{
		createTestService("test-service-3"),
		createTestService("test-service-4"),
		createTestService("test-service-5"),
	}

	for _, service := range services {
		if err := storage.RegisterService(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}
	}

	retrieved, err := storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}

	if len(retrieved) != len(services) {
		t.Errorf("Retrieved services count does not match: got %d, want %d", len(retrieved), len(services))
	}

	serviceMap := make(map[string]*ServiceInfo)
	for _, s := range retrieved {
		serviceMap[s.Info.Id] = s
	}

	for _, service := range services {
		if _, exists := serviceMap[service.Info.Id]; !exists {
			t.Errorf("Service %s not found in retrieved list", service.Info.Id)
		}
	}
}

func TestBoltDBStorage_DeleteService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	service := createTestService("test-service-6")

	if err := storage.RegisterService(ctx, service); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	if err := storage.DeleteService(ctx, service.Info.Id); err != nil {
		t.Fatalf("Failed to delete service: %v", err)
	}

	_, err := storage.GetService(ctx, service.Info.Id)
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error after deletion, got: %v", err)
	}
}

func TestBoltDBStorage_GetNonExistentService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	_, err := storage.GetService(ctx, "non-existent-service")
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error for non-existent service, got: %v", err)
	}
}

func TestBoltDBStorage_UpdateNonExistentService(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
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

	services, err := storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}
	if len(services) != 2 {
		t.Errorf("Expected 2 services after transaction, got %d", len(services))
	}

	err = storage.WithTransaction(ctx, func(txn Transaction) error {
		if err := txn.UpdateService(service1.Info.Id, func(s *ServiceInfo) error {
			s.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
			return nil
		}); err != nil {
			return err
		}

		return txn.DeleteService("non-existent-service")
	})
	if err == nil {
		t.Fatalf("Expected transaction to fail, but it succeeded")
	}

	retrieved, err := storage.GetService(ctx, service1.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
	if retrieved.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Service was updated despite transaction rollback")
	}
}
