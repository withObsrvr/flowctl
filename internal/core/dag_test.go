package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/source"
)

// TestDAGBasicFlow tests the basic flow of a small DAG with one source, one processor, and one sink
func TestDAGBasicFlow(t *testing.T) {
	// Create a new DAG
	dag := NewDAG(10)
	
	// Create a mock source
	mockSrc := &mockSource{id: "source1"}
	err := dag.AddSource("source1", mockSrc)
	if err != nil {
		t.Fatalf("Failed to add source: %v", err)
	}
	
	// Create a mock processor
	mockProc := &mockProcessor{id: "processor1"}
	err = dag.AddProcessor("processor1", mockProc, []string{"test-event"})
	if err != nil {
		t.Fatalf("Failed to add processor: %v", err)
	}
	
	// Add a sink channel
	sinkChan := dag.AddSinkChannel("sink1")
	
	// Connect source to processor
	err = dag.ConnectSourceToProcessor("source1", "processor1", "test-event")
	if err != nil {
		t.Fatalf("Failed to connect source to processor: %v", err)
	}
	
	// Connect processor to sink
	err = dag.ConnectProcessorToSink("processor1", "sink1", "processed-event")
	if err != nil {
		t.Fatalf("Failed to connect processor to sink: %v", err)
	}
	
	// Create a context with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Start the DAG
	err = dag.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start DAG: %v", err)
	}
	
	// Create a WaitGroup to wait for sink processing
	var wg sync.WaitGroup
	wg.Add(1)
	
	// Collect messages from the sink channel
	var receivedMessages []*processor.Message
	go func() {
		defer wg.Done()
		
		// Wait for at least one message or timeout
		select {
		case msg := <-sinkChan:
			receivedMessages = append(receivedMessages, msg)
		case <-ctx.Done():
			return
		}
	}()
	
	// Wait for the sink processing to complete or timeout
	wg.Wait()
	
	// Stop the DAG
	err = dag.Stop()
	if err != nil {
		t.Fatalf("Failed to stop DAG: %v", err)
	}
	
	// Verify we received at least one message if not timed out
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Test timed out waiting for messages")
	}
	
	if len(receivedMessages) == 0 {
		t.Fatal("No messages received from the sink")
	}
	
	// Verify the message content
	msg := receivedMessages[0]
	if msg.Type != "processed-event" {
		t.Errorf("Expected message type 'processed-event', got '%s'", msg.Type)
	}
}

// TestDAGComplexFlow tests a more complex DAG with multiple processors
func TestDAGComplexFlow(t *testing.T) {
	// Create a new DAG
	dag := NewDAG(10)
	
	// Create a mock source
	mockSrc := &mockSource{id: "source1"}
	err := dag.AddSource("source1", mockSrc)
	if err != nil {
		t.Fatalf("Failed to add source: %v", err)
	}
	
	// Create mock processors
	proc1 := &mockProcessor{id: "processor1"}
	proc2 := &mockProcessor{id: "processor2"}
	proc3 := &mockProcessor{id: "processor3"}
	
	// Add processors to DAG
	err = dag.AddProcessor("processor1", proc1, []string{"test-event"})
	if err != nil {
		t.Fatalf("Failed to add processor1: %v", err)
	}
	
	err = dag.AddProcessor("processor2", proc2, []string{"processed-event"})
	if err != nil {
		t.Fatalf("Failed to add processor2: %v", err)
	}
	
	err = dag.AddProcessor("processor3", proc3, []string{"processed-event"})
	if err != nil {
		t.Fatalf("Failed to add processor3: %v", err)
	}
	
	// Add sink channels
	sink1Chan := dag.AddSinkChannel("sink1")
	sink2Chan := dag.AddSinkChannel("sink2")
	
	// Connect source to processor1
	err = dag.ConnectSourceToProcessor("source1", "processor1", "test-event")
	if err != nil {
		t.Fatalf("Failed to connect source to processor1: %v", err)
	}
	
	// Connect processor1 to processor2 and processor3 (fan-out)
	err = dag.Connect("processor1", "processor2", "processed-event")
	if err != nil {
		t.Fatalf("Failed to connect processor1 to processor2: %v", err)
	}
	
	err = dag.Connect("processor1", "processor3", "processed-event")
	if err != nil {
		t.Fatalf("Failed to connect processor1 to processor3: %v", err)
	}
	
	// Connect processor2 and processor3 to sinks
	err = dag.ConnectProcessorToSink("processor2", "sink1", "processed-event")
	if err != nil {
		t.Fatalf("Failed to connect processor2 to sink1: %v", err)
	}
	
	err = dag.ConnectProcessorToSink("processor3", "sink2", "processed-event")
	if err != nil {
		t.Fatalf("Failed to connect processor3 to sink2: %v", err)
	}
	
	// Create a context with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Start the DAG
	err = dag.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start DAG: %v", err)
	}
	
	// Create a WaitGroup to wait for sink processing
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Collect messages from sink1
	var sink1Messages []*processor.Message
	go func() {
		defer wg.Done()
		
		// Wait for at least one message or timeout
		select {
		case msg := <-sink1Chan:
			sink1Messages = append(sink1Messages, msg)
		case <-ctx.Done():
			return
		}
	}()
	
	// Collect messages from sink2
	var sink2Messages []*processor.Message
	go func() {
		defer wg.Done()
		
		// Wait for at least one message or timeout
		select {
		case msg := <-sink2Chan:
			sink2Messages = append(sink2Messages, msg)
		case <-ctx.Done():
			return
		}
	}()
	
	// Wait for the sink processing to complete or timeout
	wg.Wait()
	
	// Stop the DAG
	err = dag.Stop()
	if err != nil {
		t.Fatalf("Failed to stop DAG: %v", err)
	}
	
	// Verify we received messages if not timed out
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Test timed out waiting for messages")
	}
	
	if len(sink1Messages) == 0 {
		t.Fatal("No messages received from sink1")
	}
	
	if len(sink2Messages) == 0 {
		t.Fatal("No messages received from sink2")
	}
	
	// Success - both sinks received messages
	t.Logf("Sink1 received %d messages", len(sink1Messages))
	t.Logf("Sink2 received %d messages", len(sink2Messages))
}

// mockTestProcessor is a simpler mock processor for testing
type mockTestProcessor struct {
	id       string
	inputCh  chan *source.EventEnvelope
	outputCh chan *processor.Message
}

func newMockTestProcessor(id string) *mockTestProcessor {
	return &mockTestProcessor{
		id:       id,
		inputCh:  make(chan *source.EventEnvelope, 10),
		outputCh: make(chan *processor.Message, 10),
	}
}

func (p *mockTestProcessor) Init(cfg map[string]any) error {
	return nil
}

func (p *mockTestProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
	msg := &processor.Message{
		Type:    "processed-event",
		Payload: []byte(`{"processor":"` + p.id + `","data":"processed"}`),
	}
	return []*processor.Message{msg}, nil
}

func (p *mockTestProcessor) Flush(ctx context.Context) error {
	return nil
}

func (p *mockTestProcessor) Name() string {
	return p.id
}

// mockTestSource is a simpler mock source for testing
type mockTestSource struct {
	id      string
	outputCh chan source.EventEnvelope
}

func newMockTestSource(id string) *mockTestSource {
	return &mockTestSource{
		id:      id,
		outputCh: make(chan source.EventEnvelope, 10),
	}
}

func (s *mockTestSource) Open(ctx context.Context) error {
	return nil
}

func (s *mockTestSource) Events(ctx context.Context, out chan<- source.EventEnvelope) error {
	// Send a test event immediately
	event := source.EventEnvelope{
		LedgerSeq: 12345,
		Payload:   []byte(`{"source":"` + s.id + `","data":"test-data"}`),
		Cursor:    "test-cursor",
	}
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- event:
		// Event sent
	}
	
	// Keep the channel open until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

func (s *mockTestSource) Close() error {
	return nil
}

func (s *mockTestSource) Healthy() error {
	return nil
}