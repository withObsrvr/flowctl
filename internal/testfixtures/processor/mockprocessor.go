//go:build test

package processor

import (
	"context"

	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/source"
)

// MockProcessor implements processor.Processor interface for testing
type MockProcessor struct {
	name            string
	processFunc     func(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error)
	initCalled      bool
	processCalled   bool
	flushCalled     bool
	processedEvents int
}

// NewMockProcessor creates a new mock processor that passes through events
func NewMockProcessor(name string) *MockProcessor {
	return &MockProcessor{
		name: name,
		processFunc: func(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
			return []*processor.Message{
				{
					Type:    "mock",
					Payload: e.Payload,
				},
			}, nil
		},
	}
}

// WithProcessFunc allows customizing the process function for testing
func (m *MockProcessor) WithProcessFunc(fn func(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error)) *MockProcessor {
	m.processFunc = fn
	return m
}

func (m *MockProcessor) Init(cfg map[string]any) error {
	m.initCalled = true
	return nil
}

func (m *MockProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
	m.processCalled = true
	m.processedEvents++
	return m.processFunc(ctx, e)
}

func (m *MockProcessor) Flush(ctx context.Context) error {
	m.flushCalled = true
	return nil
}

func (m *MockProcessor) Name() string {
	return m.name
}

// Stats returns information about mock processor calls for testing
func (m *MockProcessor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"init_called":      m.initCalled,
		"process_called":   m.processCalled,
		"flush_called":     m.flushCalled,
		"processed_events": m.processedEvents,
	}
}