package source

import (
	"context"
)

// EventEnvelope represents a single event from the source
type EventEnvelope struct {
	LedgerSeq uint32
	Payload   []byte // raw XDR or processed proto
	Cursor    string // source bookmark
}

// Source defines the interface for data sources
type Source interface {
	// Open initializes the source
	Open(ctx context.Context) error

	// Events returns a channel that will receive events from the source
	Events(ctx context.Context, out chan<- EventEnvelope) error

	// Close shuts down the source
	Close() error

	// Healthy returns an error if the source is not healthy
	Healthy() error
}

// MockSource implements Source interface for testing
type MockSource struct {
	ticker chan EventEnvelope
}

// NewMockSource creates a new mock source that emits events on a ticker
func NewMockSource() *MockSource {
	return &MockSource{
		ticker: make(chan EventEnvelope),
	}
}

func (m *MockSource) Open(ctx context.Context) error {
	return nil
}

func (m *MockSource) Events(ctx context.Context, out chan<- EventEnvelope) error {
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
