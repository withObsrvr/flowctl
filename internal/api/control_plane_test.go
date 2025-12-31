package api

import (
	"context"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/storage"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// createTestStorage creates a memory storage for testing
func createTestStorage() (storage.ServiceStorage, func()) {
	// Create the memory storage instance
	memStorage := storage.NewMemoryStorage()

	// Open the storage
	_ = memStorage.Open()

	// Return the storage and a cleanup function
	cleanup := func() {
		_ = memStorage.Close()
	}

	return memStorage, cleanup
}

// createTestServiceInfo creates a test service for registration (legacy API)
func createTestServiceInfo(id string, serviceType flowctlpb.ServiceType) *flowctlpb.ServiceInfo {
	return &flowctlpb.ServiceInfo{
		ServiceId:        id,
		ServiceType:      serviceType,
		InputEventTypes:  []string{},
		OutputEventTypes: []string{"test.event.type"},
		HealthEndpoint:   "localhost:8080/health",
		MaxInflight:      100,
		Metadata: map[string]string{
			"version": "1.0",
			"env":     "test",
		},
		ComponentId: id,
	}
}

// createTestHeartbeat creates a test heartbeat message (legacy API)
func createTestHeartbeat(id string) *flowctlpb.ServiceHeartbeat {
	return &flowctlpb.ServiceHeartbeat{
		ServiceId: id,
	}
}

// createV1Heartbeat creates a test heartbeat message (v1 API)
func createV1Heartbeat(id string) *flowctlv1.HeartbeatRequest {
	return &flowctlv1.HeartbeatRequest{
		ServiceId: id,
		Status:    flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
	}
}

func TestControlPlaneServer_WithPersistence(t *testing.T) {
	// Create test storage
	memStorage, cleanup := createTestStorage()
	defer cleanup()

	// Create control plane server with storage
	server := NewControlPlaneServer(memStorage)
	server.SetHeartbeatTTL(5 * time.Second)
	server.SetJanitorInterval(1 * time.Second)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	// Create wrapper for legacy API
	wrapper := NewControlPlaneWrapper(server)

	ctx := context.Background()

	// Test service registration using wrapper (legacy API)
	sourceInfo := createTestServiceInfo("test-source-1", flowctlpb.ServiceType_SERVICE_TYPE_SOURCE)
	ack, err := wrapper.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register source: %v", err)
	}
	if ack.ServiceId != "test-source-1" {
		t.Errorf("Service ID in acknowledgment does not match: got %s, want %s",
			ack.ServiceId, "test-source-1")
	}

	// Verify service is stored in memory (use the returned ServiceId)
	if _, ok := server.services[ack.ServiceId]; !ok {
		t.Errorf("Service not found in server's in-memory map")
	}

	// Send heartbeat using wrapper (legacy API)
	_, err = wrapper.Heartbeat(ctx, createTestHeartbeat(ack.ServiceId))
	if err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Register a second service
	processorInfo := createTestServiceInfo("test-processor-1", flowctlpb.ServiceType_SERVICE_TYPE_PROCESSOR)
	processorAck, err := wrapper.Register(ctx, processorInfo)
	if err != nil {
		t.Fatalf("Failed to register processor: %v", err)
	}

	// List services using wrapper (legacy API)
	serviceList, err := wrapper.ListServices(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}
	if len(serviceList.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(serviceList.Services))
	}

	// Verify data in persistent storage
	storedServices, err := memStorage.ListServices(ctx)
	if err != nil {
		t.Fatalf("Failed to list services from storage: %v", err)
	}
	if len(storedServices) != 2 {
		t.Errorf("Expected 2 services in storage, got %d", len(storedServices))
	}

	// Create a new server instance with the same storage to test recovery
	server.Close()

	// Create new server instance with same storage
	newServer := NewControlPlaneServer(memStorage)
	if err := newServer.Start(); err != nil {
		t.Fatalf("Failed to start new server: %v", err)
	}
	defer newServer.Close()

	// Verify services were recovered
	if len(newServer.services) != 2 {
		t.Errorf("Expected 2 services after recovery, got %d", len(newServer.services))
	}

	// Verify the specific services were recovered (use the ack ServiceIds)
	for _, id := range []string{ack.ServiceId, processorAck.ServiceId} {
		if _, ok := newServer.services[id]; !ok {
			t.Errorf("Service %s not recovered in new server instance", id)
		}
	}

	// Verify we can get component status using v1 API
	statusReq := &flowctlv1.ComponentStatusRequest{
		ServiceId: ack.ServiceId,
	}
	status, err := newServer.GetComponentStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("Failed to get component status: %v", err)
	}
	if status == nil {
		t.Fatal("Status should not be nil")
	}
}

func TestControlPlaneServer_InMemoryMode(t *testing.T) {
	// Create control plane server without persistent storage
	server := NewControlPlaneServer(nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	// Create wrapper for legacy API
	wrapper := NewControlPlaneWrapper(server)

	ctx := context.Background()

	// Register a service using wrapper
	sourceInfo := createTestServiceInfo("in-memory-source", flowctlpb.ServiceType_SERVICE_TYPE_SOURCE)
	ack, err := wrapper.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Verify service is stored in memory (use the returned ServiceId)
	if _, ok := server.services[ack.ServiceId]; !ok {
		t.Errorf("Service not found in server's in-memory map")
	}
}

func TestControlPlaneServer_Janitor(t *testing.T) {
	// Create control plane server without persistent storage
	server := NewControlPlaneServer(nil)

	// Set short TTL for testing
	server.SetHeartbeatTTL(100 * time.Millisecond)
	server.SetJanitorInterval(50 * time.Millisecond)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	// Create wrapper for legacy API
	wrapper := NewControlPlaneWrapper(server)

	ctx := context.Background()

	// Register a service using wrapper
	sourceInfo := createTestServiceInfo("janitor-test-source", flowctlpb.ServiceType_SERVICE_TYPE_SOURCE)
	ack, err := wrapper.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Immediately after registration, the service should be healthy
	statusReq := &flowctlv1.ComponentStatusRequest{
		ServiceId: ack.ServiceId,
	}
	status, err := server.GetComponentStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("Failed to get component status: %v", err)
	}
	if status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Service should be healthy immediately after registration")
	}

	// Wait for the janitor to mark the service as unhealthy
	time.Sleep(200 * time.Millisecond) // > TTL + janitor interval

	// Check service health again - should be unhealthy now
	status, err = server.GetComponentStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("Failed to get component status: %v", err)
	}
	if status.Status == flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Service should be unhealthy after TTL has passed")
	}

	// Send a heartbeat using v1 API to make it healthy again
	_, err = server.Heartbeat(ctx, createV1Heartbeat(ack.ServiceId))
	if err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Check service health again - should be healthy now
	status, err = server.GetComponentStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("Failed to get component status: %v", err)
	}
	if status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Service should be healthy after receiving heartbeat")
	}
}

func TestControlPlaneWrapper_Register(t *testing.T) {
	// Create control plane server
	server := NewControlPlaneServer(nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	wrapper := NewControlPlaneWrapper(server)
	ctx := context.Background()

	// Test that the wrapper correctly propagates ServiceId
	serviceInfo := &flowctlpb.ServiceInfo{
		ServiceId:        "my-service-id",
		ComponentId:      "my-component-id",
		ServiceType:      flowctlpb.ServiceType_SERVICE_TYPE_SOURCE,
		OutputEventTypes: []string{"test.event"},
	}

	ack, err := wrapper.Register(ctx, serviceInfo)
	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	// The returned ServiceId should match what was requested
	if ack.ServiceId != "my-service-id" {
		t.Errorf("Expected ServiceId 'my-service-id', got '%s'", ack.ServiceId)
	}

	// The service should be stored under the correct key
	if _, ok := server.services["my-service-id"]; !ok {
		t.Error("Service not stored under the correct ServiceId key")
	}

	// Heartbeat should work with the returned ServiceId
	_, err = wrapper.Heartbeat(ctx, &flowctlpb.ServiceHeartbeat{
		ServiceId: ack.ServiceId,
	})
	if err != nil {
		t.Fatalf("Heartbeat failed with returned ServiceId: %v", err)
	}
}
