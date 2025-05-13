package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/withobsrvr/flowctl/internal/processor"
)

// Sink defines the interface for data sinks
type Sink interface {
	// Connect initializes the sink with configuration
	Connect(cfg map[string]any) error

	// Write sends messages to the sink
	Write(ctx context.Context, msgs []*processor.Message) error

	// Close shuts down the sink
	Close() error

	// Healthy returns an error if the sink is not healthy
	Healthy() error
}

// StdoutSink implements Sink interface for testing
type StdoutSink struct {
	encoder *json.Encoder
}

// NewStdoutSink creates a new sink that writes to stdout
func NewStdoutSink() *StdoutSink {
	return &StdoutSink{
		encoder: json.NewEncoder(os.Stdout),
	}
}

func (s *StdoutSink) Connect(cfg map[string]any) error {
	return nil
}

func (s *StdoutSink) Write(ctx context.Context, msgs []*processor.Message) error {
	for _, msg := range msgs {
		if err := s.encoder.Encode(msg); err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}
	}
	return nil
}

func (s *StdoutSink) Close() error {
	return nil
}

func (s *StdoutSink) Healthy() error {
	return nil
}
