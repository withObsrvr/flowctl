package storage

import (
	"context"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createTestMemoryService(id string) *ServiceInfo {
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

func TestMemoryStorage_Basic(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	
	if err := storage.Open(); err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()
	
	// Test registration
	service := createTestMemoryService("memory-test-1")
	if err := storage.RegisterService(ctx, service); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}
	
	// Test retrieval
	retrieved, err := storage.GetService(ctx, service.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
	if retrieved.Info.Id != service.Info.Id {
		t.Errorf("Retrieved service ID does not match: got %s, want %s", 
			retrieved.Info.Id, service.Info.Id)
	}
	
	// Test update
	err = storage.UpdateService(ctx, service.Info.Id, func(s *ServiceInfo) error {
		s.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to update service: %v", err)
	}
	
	// Verify update
	retrieved, err = storage.GetService(ctx, service.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service after update: %v", err)
	}
	if retrieved.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Errorf("Update to service health did not take effect")
	}
	
	// Test list
	services, err := storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}
	
	// Test delete
	if err := storage.DeleteService(ctx, service.Info.Id); err != nil {
		t.Fatalf("Failed to delete service: %v", err)
	}
	
	// Verify deletion
	_, err = storage.GetService(ctx, service.Info.Id)
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error after deletion, got: %v", err)
	}
	
	// List should be empty now
	services, err = storage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services after deletion: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("Expected 0 services after deletion, got %d", len(services))
	}
}

func TestMemoryStorage_Transaction(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	
	if err := storage.Open(); err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()
	
	// Test transaction
	service1 := createTestMemoryService("memory-txn-1")
	service2 := createTestMemoryService("memory-txn-2")
	
	err := storage.WithTransaction(ctx, func(txn Transaction) error {
		if err := txn.RegisterService(service1); err != nil {
			return err
		}
		return txn.RegisterService(service2)
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
	
	// Test transaction with error
	err = storage.WithTransaction(ctx, func(txn Transaction) error {
		// Update a service
		if err := txn.UpdateService(service1.Info.Id, func(s *ServiceInfo) error {
			s.Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY
			return nil
		}); err != nil {
			return err
		}
		
		// Try to update a non-existent service, which should fail
		return txn.UpdateService("non-existent", func(s *ServiceInfo) error {
			return nil
		})
	})
	
	// The transaction should have failed
	if err == nil {
		t.Errorf("Expected transaction to fail, but it succeeded")
	}
	
	// In memory storage doesn't support true transactions (rollback)
	// so we need to check if the update was applied despite the error
	retrieved, err := storage.GetService(ctx, service1.Info.Id)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
	if retrieved.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Errorf("Memory transaction doesn't support rollback, but this test still verifies behavior")
	}
}