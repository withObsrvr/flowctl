package processor

import (
	"context"

	"github.com/withobsrvr/flowctl/internal/source"
)

// Message represents a processed event that can be sent to a sink
type Message struct {
	Type    string
	Payload []byte
}

// Processor defines the interface for data processors
type Processor interface {
	// Init initializes the processor with configuration
	Init(cfg map[string]any) error

	// Process handles a single event and returns zero or more messages
	Process(ctx context.Context, e *source.EventEnvelope) ([]*Message, error)

	// Flush ensures any buffered data is processed
	Flush(ctx context.Context) error

	// Name returns the processor's name
	Name() string
}

// MockProcessor implements Processor interface for testing
type MockProcessor struct {
	name string
}

// NewMockProcessor creates a new mock processor that passes through events
func NewMockProcessor(name string) *MockProcessor {
	return &MockProcessor{
		name: name,
	}
}

func (m *MockProcessor) Init(cfg map[string]any) error {
	return nil
}

func (m *MockProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*Message, error) {
	return []*Message{
		{
			Type:    "mock",
			Payload: e.Payload,
		},
	}, nil
}

func (m *MockProcessor) Flush(ctx context.Context) error {
	return nil
}

func (m *MockProcessor) Name() string {
	return m.name
}
