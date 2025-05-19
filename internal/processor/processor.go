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

// For testing purposes, see internal/testfixtures/processor/mockprocessor.go
