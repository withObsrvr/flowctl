package controlplane

import (
	"context"
	"testing"
	"time"
)

func TestEmbeddedControlPlane_Lifecycle(t *testing.T) {
	// Create embedded control plane
	config := Config{
		Address:         "127.0.0.1",
		Port:            8081, // Use different port to avoid conflicts
		HeartbeatTTL:    30 * time.Second,
		JanitorInterval: 10 * time.Second,
	}

	cp := NewEmbeddedControlPlane(config)

	// Test initial state
	if cp.IsStarted() {
		t.Error("Control plane should not be started initially")
	}

	// Test start
	ctx := context.Background()
	if err := cp.Start(ctx); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}

	// Test started state
	if !cp.IsStarted() {
		t.Error("Control plane should be started")
	}

	// Test endpoint
	expectedEndpoint := "http://127.0.0.1:8081"
	if cp.GetEndpoint() != expectedEndpoint {
		t.Errorf("Expected endpoint %s, got %s", expectedEndpoint, cp.GetEndpoint())
	}

	// Test port and address
	if cp.GetPort() != 8081 {
		t.Errorf("Expected port 8081, got %d", cp.GetPort())
	}

	if cp.GetAddress() != "127.0.0.1" {
		t.Errorf("Expected address 127.0.0.1, got %s", cp.GetAddress())
	}

	// Test service list (should be empty initially)
	services, err := cp.GetServiceList()
	if err != nil {
		t.Errorf("Failed to get service list: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(services))
	}

	// Test stop
	if err := cp.Stop(); err != nil {
		t.Errorf("Failed to stop control plane: %v", err)
	}

	// Test stopped state
	if cp.IsStarted() {
		t.Error("Control plane should not be started after stop")
	}
}

func TestEmbeddedControlPlane_DoubleStart(t *testing.T) {
	config := Config{
		Address:         "127.0.0.1",
		Port:            8082, // Use different port
		HeartbeatTTL:    30 * time.Second,
		JanitorInterval: 10 * time.Second,
	}

	cp := NewEmbeddedControlPlane(config)
	ctx := context.Background()

	// First start should succeed
	if err := cp.Start(ctx); err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// Second start should fail
	if err := cp.Start(ctx); err == nil {
		t.Error("Second start should fail")
	}

	// Cleanup
	cp.Stop()
}

func TestEmbeddedControlPlane_WaitForComponent(t *testing.T) {
	config := Config{
		Address:         "127.0.0.1",
		Port:            8083, // Use different port
		HeartbeatTTL:    30 * time.Second,
		JanitorInterval: 10 * time.Second,
	}

	cp := NewEmbeddedControlPlane(config)
	ctx := context.Background()

	if err := cp.Start(ctx); err != nil {
		t.Fatalf("Failed to start control plane: %v", err)
	}
	defer cp.Stop()

	// Test waiting for non-existent component should timeout
	err := cp.WaitForComponent("non-existent", 1*time.Second)
	if err == nil {
		t.Error("Expected timeout error when waiting for non-existent component")
	}

	if err.Error() != "timeout waiting for component non-existent to register" {
		t.Errorf("Expected timeout error message, got: %v", err)
	}
}