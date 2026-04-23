package api

import (
	"context"
	"testing"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/storage"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
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

func TestControlPlaneServer_StopPipelineRunInvokesStopper(t *testing.T) {
	memStorage, cleanup := createTestStorage()
	defer cleanup()

	server := NewControlPlaneServer(memStorage)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	ctx := context.Background()
	_, err := server.CreatePipelineRun(ctx, &flowctlpb.CreatePipelineRunRequest{
		RunId:        "run-stop-test",
		PipelineName: "test-pipeline",
	})
	if err != nil {
		t.Fatalf("failed to create pipeline run: %v", err)
	}

	stopped := make(chan struct{}, 1)
	server.RegisterRunStopper("run-stop-test", func() {
		stopped <- struct{}{}
	})

	run, err := server.StopPipelineRun(ctx, &flowctlpb.StopPipelineRunRequest{RunId: "run-stop-test"})
	if err != nil {
		t.Fatalf("failed to stop pipeline run: %v", err)
	}
	if run.Status != flowctlpb.RunStatus_RUN_STATUS_STOPPED {
		t.Fatalf("expected stopped status, got %v", run.Status)
	}

	select {
	case <-stopped:
	case <-time.After(1 * time.Second):
		t.Fatal("expected stop callback to be invoked")
	}
}

func TestControlPlaneServer_HeartbeatUnknownPreservesStatusAndMergesMetrics(t *testing.T) {
	server := NewControlPlaneServer(nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	ctx := context.Background()
	_, err := server.RegisterComponent(ctx, &flowctlv1.RegisterRequest{
		ComponentId: "svc-1",
		Component: &flowctlv1.ComponentInfo{
			Id:   "svc-1",
			Type: flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE,
		},
	})
	if err != nil {
		t.Fatalf("failed to register component: %v", err)
	}

	server.services["svc-1"].Status.Status = flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED
	server.services["svc-1"].Status.Metrics = map[string]string{"component_metric": "42"}

	_, err = server.Heartbeat(ctx, &flowctlv1.HeartbeatRequest{
		ServiceId: "svc-1",
		Status:    flowctlv1.HealthStatus_HEALTH_STATUS_UNKNOWN,
		Metrics:   map[string]string{"orchestrator": "process"},
	})
	if err != nil {
		t.Fatalf("failed to send heartbeat: %v", err)
	}

	status, err := server.GetComponentStatus(ctx, &flowctlv1.ComponentStatusRequest{ServiceId: "svc-1"})
	if err != nil {
		t.Fatalf("failed to get component status: %v", err)
	}
	if status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED {
		t.Fatalf("expected degraded status to be preserved, got %v", status.Status)
	}
	if status.Metrics["component_metric"] != "42" {
		t.Fatalf("expected existing metric to be preserved, got metrics=%v", status.Metrics)
	}
	if status.Metrics["orchestrator"] != "process" {
		t.Fatalf("expected synthetic metric to be merged, got metrics=%v", status.Metrics)
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
		InputEventTypes:  []string{"source.input"},
		OutputEventTypes: []string{"test.event"},
		HealthEndpoint:   "127.0.0.1:1234",
		Metadata: map[string]string{
			"source_name":        "My Source",
			"source_version":     "1.2.3",
			"source_description": "test source",
			"network":            "testnet",
		},
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

	stored := server.services["my-service-id"]
	if stored.Info.Id != "my-component-id" {
		t.Fatalf("expected component id to be propagated, got %q", stored.Info.Id)
	}
	if stored.Info.Name != "My Source" || stored.Info.Version != "1.2.3" || stored.Info.Description != "test source" {
		t.Fatalf("expected wrapper to propagate basic metadata, got %+v", stored.Info)
	}
	if stored.Info.Endpoint != "127.0.0.1:1234" {
		t.Fatalf("expected endpoint to be propagated, got %q", stored.Info.Endpoint)
	}
	if len(stored.Info.InputEventTypes) != 1 || stored.Info.InputEventTypes[0] != "source.input" {
		t.Fatalf("expected input event types to be propagated, got %v", stored.Info.InputEventTypes)
	}
	if len(stored.Info.OutputEventTypes) != 1 || stored.Info.OutputEventTypes[0] != "test.event" {
		t.Fatalf("expected output event types to be propagated, got %v", stored.Info.OutputEventTypes)
	}
	if stored.Info.Metadata["network"] != "testnet" {
		t.Fatalf("expected metadata to be propagated, got %v", stored.Info.Metadata)
	}

	// Heartbeat should work with the returned ServiceId
	_, err = wrapper.Heartbeat(ctx, &flowctlpb.ServiceHeartbeat{
		ServiceId: ack.ServiceId,
	})
	if err != nil {
		t.Fatalf("Heartbeat failed with returned ServiceId: %v", err)
	}
}
