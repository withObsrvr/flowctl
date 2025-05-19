//go:build test

package source

import (
	"context"

	"github.com/withobsrvr/flowctl/internal/source"
)

// MockSource implements source.Source interface for testing
type MockSource struct {
	ticker chan source.EventEnvelope
}

// NewMockSource creates a new mock source that emits events on a ticker
func NewMockSource() *MockSource {
	return &MockSource{
		ticker: make(chan source.EventEnvelope),
	}
}

func (m *MockSource) Open(ctx context.Context) error {
	return nil
}

func (m *MockSource) Events(ctx context.Context, out chan<- source.EventEnvelope) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-m.ticker:
			out <- event
		}
	}
}

func (m *MockSource) Close() error {
	close(m.ticker)
	return nil
}

func (m *MockSource) Healthy() error {
	return nil
}

// EmitEvent allows test code to inject events into the mock source
func (m *MockSource) EmitEvent(event source.EventEnvelope) {
	m.ticker <- event
}