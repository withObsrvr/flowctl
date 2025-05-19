//go:build !test

// Package testfixtures provides test helper functions and mock objects.
// This file provides production stubs that delegate to real implementations.
package testfixtures

import (
	"context"

	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/sink"
	"github.com/withobsrvr/flowctl/internal/source"
)

// CreateMockSource creates a placeholder source for development.
// In production, this should be replaced with a real source.
func CreateMockSource() source.Source {
	// In production, we return a simple placeholder source
	return &placeholderSource{}
}

// CreateMockProcessor creates a placeholder processor for development.
// In production, this should be replaced with a real processor.
func CreateMockProcessor(name string) processor.Processor {
	// In production, we return a simple placeholder processor
	return &placeholderProcessor{name: name}
}

// CreateMockSink creates a placeholder sink for development.
// In production, this should be replaced with a real sink.
func CreateMockSink() sink.Sink {
	// In production, we return stdout sink
	return sink.NewStdoutSink()
}

// AsMockSource attempts to convert a Source to a MockSource.
// In production, this always returns false.
func AsMockSource(src source.Source) (interface{}, bool) {
	return nil, false
}

// AsMockProcessor attempts to convert a Processor to a MockProcessor.
// In production, this always returns false.
func AsMockProcessor(proc processor.Processor) (interface{}, bool) {
	return nil, false
}

// AsMockSink attempts to convert a Sink to a MockSink.
// In production, this always returns false.
func AsMockSink(snk sink.Sink) (interface{}, bool) {
	return nil, false
}

// placeholderSource is a minimal implementation of source.Source for development
type placeholderSource struct{}

func (s *placeholderSource) Open(ctx context.Context) error {
	return nil
}

func (s *placeholderSource) Events(ctx context.Context, out chan<- source.EventEnvelope) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *placeholderSource) Close() error {
	return nil
}

func (s *placeholderSource) Healthy() error {
	return nil
}

// placeholderProcessor is a minimal implementation of processor.Processor for development
type placeholderProcessor struct {
	name string
}

func (p *placeholderProcessor) Init(cfg map[string]any) error {
	return nil
}

func (p *placeholderProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
	return []*processor.Message{
		{
			Type:    "placeholder",
			Payload: e.Payload,
		},
	}, nil
}

func (p *placeholderProcessor) Flush(ctx context.Context) error {
	return nil
}

func (p *placeholderProcessor) Name() string {
	return p.name
}