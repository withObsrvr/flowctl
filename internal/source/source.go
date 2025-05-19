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

// For testing purposes, see internal/testfixtures/source/mocksource.go
