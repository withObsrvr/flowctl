//go:build test

package testfixtures_test

import (
	"context"
	"testing"

	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/source"
	"github.com/withobsrvr/flowctl/internal/testfixtures"
)

func TestMockSource(t *testing.T) {
	// Create a mock source
	src := testfixtures.CreateMockSource()
	
	// Check if we can convert it back to a MockSource
	mockSrc, ok := testfixtures.AsMockSource(src)
	if !ok {
		t.Fatal("Failed to convert Source to MockSource")
	}
	
	// Create an event envelope
	event := source.EventEnvelope{
		LedgerSeq: 123,
		Payload:   []byte("test payload"),
		Cursor:    "test-cursor",
	}
	
	// Create a channel and context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	eventChan := make(chan source.EventEnvelope, 1)
	
	// Start the source in a goroutine
	go func() {
		if err := src.Open(ctx); err != nil {
			t.Errorf("Failed to open source: %v", err)
		}
		
		if err := src.Events(ctx, eventChan); err != nil && err != context.Canceled {
			t.Errorf("Events returned error: %v", err)
		}
	}()
	
	// Emit an event
	mockSrc.EmitEvent(event)
	
	// Check that we received the event
	receivedEvent := <-eventChan
	if receivedEvent.LedgerSeq != event.LedgerSeq {
		t.Errorf("Expected ledger seq %d, got %d", event.LedgerSeq, receivedEvent.LedgerSeq)
	}
	
	// Cleanup
	if err := src.Close(); err != nil {
		t.Errorf("Failed to close source: %v", err)
	}
}

func TestMockProcessor(t *testing.T) {
	// Create a mock processor
	proc := testfixtures.CreateMockProcessor("test-processor")
	
	// Check if we can convert it back to a MockProcessor
	mockProc, ok := testfixtures.AsMockProcessor(proc)
	if !ok {
		t.Fatal("Failed to convert Processor to MockProcessor")
	}
	
	// Initialize the processor
	if err := proc.Init(map[string]any{"test": "config"}); err != nil {
		t.Fatalf("Failed to initialize processor: %v", err)
	}
	
	// Create an event envelope
	event := source.EventEnvelope{
		LedgerSeq: 123,
		Payload:   []byte("test payload"),
		Cursor:    "test-cursor",
	}
	
	// Process the event
	ctx := context.Background()
	msgs, err := proc.Process(ctx, &event)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}
	
	// Check the output
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Type != "mock" {
		t.Errorf("Expected message type 'mock', got '%s'", msgs[0].Type)
	}
	
	// Flush the processor
	if err := proc.Flush(ctx); err != nil {
		t.Fatalf("Failed to flush processor: %v", err)
	}
	
	// Check the stats
	stats := mockProc.Stats()
	if !stats["init_called"].(bool) {
		t.Error("Init should have been called")
	}
	if !stats["process_called"].(bool) {
		t.Error("Process should have been called")
	}
	if !stats["flush_called"].(bool) {
		t.Error("Flush should have been called")
	}
	if stats["processed_events"].(int) != 1 {
		t.Errorf("Expected 1 processed event, got %d", stats["processed_events"].(int))
	}
}

func TestMockSink(t *testing.T) {
	// Create a mock sink
	sink := testfixtures.CreateMockSink()
	
	// Check if we can convert it back to a MockSink
	mockSink, ok := testfixtures.AsMockSink(sink)
	if !ok {
		t.Fatal("Failed to convert Sink to MockSink")
	}
	
	// Connect to the sink
	if err := sink.Connect(map[string]any{"test": "config"}); err != nil {
		t.Fatalf("Failed to connect to sink: %v", err)
	}
	
	// Create some messages
	msgs := []*processor.Message{
		{
			Type:    "test-type",
			Payload: []byte("test payload"),
		},
	}
	
	// Write the messages
	ctx := context.Background()
	if err := sink.Write(ctx, msgs); err != nil {
		t.Fatalf("Failed to write messages: %v", err)
	}
	
	// Close the sink
	if err := sink.Close(); err != nil {
		t.Fatalf("Failed to close sink: %v", err)
	}
	
	// Check the messages
	receivedMsgs := mockSink.Messages()
	if len(receivedMsgs) != 1 {
		t.Fatalf("Expected 1 batch of messages, got %d", len(receivedMsgs))
	}
	if len(receivedMsgs[0]) != 1 {
		t.Fatalf("Expected 1 message in batch, got %d", len(receivedMsgs[0]))
	}
	if receivedMsgs[0][0].Type != "test-type" {
		t.Errorf("Expected message type 'test-type', got '%s'", receivedMsgs[0][0].Type)
	}
	
	// Check the stats
	stats := mockSink.Stats()
	if !stats["connect_called"].(bool) {
		t.Error("Connect should have been called")
	}
	if !stats["write_called"].(bool) {
		t.Error("Write should have been called")
	}
	if !stats["close_called"].(bool) {
		t.Error("Close should have been called")
	}
	if stats["message_count"].(int) != 1 {
		t.Errorf("Expected 1 batch of messages, got %d", stats["message_count"].(int))
	}
}