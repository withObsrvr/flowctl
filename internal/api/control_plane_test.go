package api

import (
	"context"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/storage"
	pb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// createTestService creates a test service for registration
func createTestServiceInfo(id string, serviceType pb.ServiceType) *pb.ServiceInfo {
	return &pb.ServiceInfo{
		ServiceId:   id,
		ServiceType: serviceType,
		InputEventTypes: []string{},
		OutputEventTypes: []string{"test.event.type"},
		HealthEndpoint: "localhost:8080/health",
		MaxInflight: 100,
		Metadata: map[string]string{
			"version": "1.0",
			"env":     "test",
		},
	}
}

// createTestHeartbeat creates a test heartbeat message
func createTestHeartbeat(id string) *pb.ServiceHeartbeat {
	return &pb.ServiceHeartbeat{
		ServiceId: id,
		Timestamp: timestamppb.Now(),
		Metrics: map[string]float64{
			"requests": 100,
			"latency_ms": 50,
		},
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

	ctx := context.Background()

	// Test service registration
	sourceInfo := createTestServiceInfo("test-source-1", pb.ServiceType_SERVICE_TYPE_SOURCE)
	ack, err := server.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register source: %v", err)
	}
	if ack.ServiceId != "test-source-1" {
		t.Errorf("Service ID in acknowledgment does not match: got %s, want %s", 
			ack.ServiceId, "test-source-1")
	}

	// Verify service is stored in memory
	if _, ok := server.services[sourceInfo.ServiceId]; !ok {
		t.Errorf("Service not found in server's in-memory map")
	}

	// Send heartbeat
	_, err = server.Heartbeat(ctx, createTestHeartbeat(sourceInfo.ServiceId))
	if err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Register a second service
	processorInfo := createTestServiceInfo("test-processor-1", pb.ServiceType_SERVICE_TYPE_PROCESSOR)
	_, err = server.Register(ctx, processorInfo)
	if err != nil {
		t.Fatalf("Failed to register processor: %v", err)
	}

	// List services
	serviceList, err := server.ListServices(ctx, &emptypb.Empty{})
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

	// Verify the specific services were recovered
	for _, id := range []string{sourceInfo.ServiceId, processorInfo.ServiceId} {
		if _, ok := newServer.services[id]; !ok {
			t.Errorf("Service %s not recovered in new server instance", id)
		}
	}

	// Verify we can get service status 
	status, err := newServer.GetServiceStatus(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to get service status: %v", err)
	}
	if status.ServiceId != sourceInfo.ServiceId {
		t.Errorf("Retrieved status has wrong service ID: got %s, want %s", 
			status.ServiceId, sourceInfo.ServiceId)
	}
}

func TestControlPlaneServer_InMemoryMode(t *testing.T) {
	// Create control plane server without persistent storage
	server := NewControlPlaneServer(nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	ctx := context.Background()

	// Register a service
	sourceInfo := createTestServiceInfo("in-memory-source", pb.ServiceType_SERVICE_TYPE_SOURCE)
	_, err := server.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Verify service is stored in memory
	if _, ok := server.services[sourceInfo.ServiceId]; !ok {
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

	ctx := context.Background()

	// Register a service
	sourceInfo := createTestServiceInfo("janitor-test-source", pb.ServiceType_SERVICE_TYPE_SOURCE)
	_, err := server.Register(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Immediately after registration, the service should be healthy
	status, err := server.GetServiceStatus(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to get service status: %v", err)
	}
	if !status.IsHealthy {
		t.Errorf("Service should be healthy immediately after registration")
	}

	// Wait for the janitor to mark the service as unhealthy
	time.Sleep(200 * time.Millisecond) // > TTL + janitor interval

	// Check service health again - should be unhealthy now
	status, err = server.GetServiceStatus(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to get service status: %v", err)
	}
	if status.IsHealthy {
		t.Errorf("Service should be unhealthy after TTL has passed")
	}

	// Send a heartbeat to make it healthy again
	_, err = server.Heartbeat(ctx, createTestHeartbeat(sourceInfo.ServiceId))
	if err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Check service health again - should be healthy now
	status, err = server.GetServiceStatus(ctx, sourceInfo)
	if err != nil {
		t.Fatalf("Failed to get service status: %v", err)
	}
	if !status.IsHealthy {
		t.Errorf("Service should be healthy after receiving heartbeat")
	}
}