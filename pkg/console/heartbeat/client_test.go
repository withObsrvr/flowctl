package heartbeat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://console.obsrvr.com", "123", "session-456", "secret")

	if client.consoleURL != "https://console.obsrvr.com" {
		t.Errorf("Expected consoleURL to be https://console.obsrvr.com, got %s", client.consoleURL)
	}

	if client.pipelineID != "123" {
		t.Errorf("Expected pipelineID to be 123, got %s", client.pipelineID)
	}

	if client.sessionID != "session-456" {
		t.Errorf("Expected sessionID to be session-456, got %s", client.sessionID)
	}

	if client.webhookSecret != "secret" {
		t.Errorf("Expected webhookSecret to be secret, got %s", client.webhookSecret)
	}

	if client.GetLedgerCount() != 0 {
		t.Errorf("Expected initial ledger count to be 0, got %d", client.GetLedgerCount())
	}
}

func TestSetGetLedgerCount(t *testing.T) {
	client := NewClient("https://test.com", "1", "2", "secret")

	client.SetLedgerCount(1000)
	count := client.GetLedgerCount()

	if count != 1000 {
		t.Errorf("Expected ledger count to be 1000, got %d", count)
	}

	// Test atomic updates
	client.SetLedgerCount(5000)
	count = client.GetLedgerCount()

	if count != 5000 {
		t.Errorf("Expected ledger count to be 5000, got %d", count)
	}
}

func TestSendHeartbeat(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Verify path
		expectedPath := "/flow/api/pipelines/123/heartbeat/"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json")
		}

		signature := r.Header.Get("X-Pipeline-Signature")
		if signature == "" {
			t.Errorf("Expected X-Pipeline-Signature header")
		}

		// Verify signature length (64 hex characters for SHA256)
		if len(signature) != 64 {
			t.Errorf("Expected signature length 64, got %d", len(signature))
		}

		// Verify payload
		body, _ := io.ReadAll(r.Body)
		var payload HeartbeatPayload
		json.Unmarshal(body, &payload)

		if payload.SessionID != "test-session" {
			t.Errorf("Expected session_id test-session, got %s", payload.SessionID)
		}

		if payload.LedgersProcessed != 5000 {
			t.Errorf("Expected ledgers_processed 5000, got %d", payload.LedgersProcessed)
		}

		if payload.Timestamp == "" {
			t.Errorf("Expected timestamp to be set")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "test-session", "secret")
	client.SetLedgerCount(5000)

	ctx := context.Background()
	err := client.SendHeartbeat(ctx, nil)

	if err != nil {
		t.Errorf("SendHeartbeat failed: %v", err)
	}
}

func TestSendHeartbeatWithCheckpointData(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify payload includes checkpoint data
		body, _ := io.ReadAll(r.Body)
		var payload HeartbeatPayload
		json.Unmarshal(body, &payload)

		if payload.CheckpointData == nil {
			t.Errorf("Expected checkpoint_data to be present")
		}

		ledger, ok := payload.CheckpointData["ledger"].(float64)
		if !ok || int(ledger) != 12345 {
			t.Errorf("Expected checkpoint_data.ledger to be 12345")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "test-session", "secret")
	client.SetLedgerCount(5000)

	ctx := context.Background()
	checkpointData := map[string]interface{}{
		"ledger": 12345,
	}
	err := client.SendHeartbeat(ctx, checkpointData)

	if err != nil {
		t.Errorf("SendHeartbeat with checkpoint failed: %v", err)
	}
}

func TestSendHeartbeatFailure(t *testing.T) {
	// Mock server that returns 403
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"Invalid signature"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "test-session", "secret")

	ctx := context.Background()
	err := client.SendHeartbeat(ctx, nil)

	if err == nil {
		t.Errorf("Expected error for 403 response")
	}

	if err.Error() != "heartbeat failed with status: 403" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestSendHeartbeatContextCanceled(t *testing.T) {
	// Mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "test-session", "secret")

	// Create context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.SendHeartbeat(ctx, nil)

	if err == nil {
		t.Errorf("Expected error when context is canceled")
	}
}

func TestGenerateSignature(t *testing.T) {
	client := NewClient("https://test.com", "1", "2", "my-secret")

	payload := []byte(`{"test":"data"}`)
	signature := client.generateSignature(payload)

	// Should be 64-character hex string
	if len(signature) != 64 {
		t.Errorf("Expected signature length 64, got %d", len(signature))
	}

	// Same payload should generate same signature
	signature2 := client.generateSignature(payload)
	if signature != signature2 {
		t.Errorf("Signatures should be deterministic")
	}

	// Different payload should generate different signature
	differentPayload := []byte(`{"test":"other"}`)
	signature3 := client.generateSignature(differentPayload)
	if signature == signature3 {
		t.Errorf("Different payloads should generate different signatures")
	}

	// Different secret should generate different signature
	client2 := NewClient("https://test.com", "1", "2", "different-secret")
	signature4 := client2.generateSignature(payload)
	if signature == signature4 {
		t.Errorf("Different secrets should generate different signatures")
	}
}

func TestHeartbeatLoop(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "session", "secret")
	client.SetLedgerCount(1000)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	// Start heartbeat loop with 50ms interval (fast for testing)
	go client.StartHeartbeatLoop(ctx, 50*time.Millisecond)

	// Wait for loop to run
	<-ctx.Done()

	// Should have sent at least 2 heartbeats
	if callCount < 2 {
		t.Errorf("Expected at least 2 heartbeats, got %d", callCount)
	}
}

func TestHeartbeatLoopContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "session", "secret")

	ctx, cancel := context.WithCancel(context.Background())

	// Start heartbeat loop
	go client.StartHeartbeatLoop(ctx, 100*time.Millisecond)

	// Cancel after 50ms
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Give it time to exit
	time.Sleep(50 * time.Millisecond)

	// If we reach here without hanging, the loop exited properly
}

func TestHeartbeatLoopContinuesOnError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// First call fails, subsequent succeed
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "123", "session", "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	// Start heartbeat loop
	go client.StartHeartbeatLoop(ctx, 50*time.Millisecond)

	// Wait for loop to run
	<-ctx.Done()

	// Should have attempted multiple calls despite first failure
	if callCount < 2 {
		t.Errorf("Expected at least 2 heartbeat attempts, got %d", callCount)
	}
}

func TestHeartbeatPayloadStructure(t *testing.T) {
	payload := HeartbeatPayload{
		SessionID:        "test-session-id",
		LedgersProcessed: 12345,
		CheckpointData: map[string]interface{}{
			"ledger": 54321,
			"hash":   "abc123",
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("Failed to marshal payload: %v", err)
	}

	// Unmarshal back
	var decoded HeartbeatPayload
	err = json.Unmarshal(jsonBytes, &decoded)
	if err != nil {
		t.Errorf("Failed to unmarshal payload: %v", err)
	}

	// Verify fields
	if decoded.SessionID != payload.SessionID {
		t.Errorf("SessionID mismatch")
	}

	if decoded.LedgersProcessed != payload.LedgersProcessed {
		t.Errorf("LedgersProcessed mismatch")
	}

	if decoded.Timestamp != payload.Timestamp {
		t.Errorf("Timestamp mismatch")
	}

	// Verify checkpoint data
	if ledger, ok := decoded.CheckpointData["ledger"].(float64); !ok || int(ledger) != 54321 {
		t.Errorf("CheckpointData ledger mismatch")
	}
}

func TestConcurrentLedgerUpdates(t *testing.T) {
	client := NewClient("https://test.com", "1", "2", "secret")

	// Concurrently update ledger count
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(val int64) {
			client.SetLedgerCount(val * 100)
			done <- true
		}(int64(i))
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have a value (race detector would catch issues)
	count := client.GetLedgerCount()
	if count < 0 {
		t.Errorf("Invalid ledger count after concurrent updates: %d", count)
	}
}
