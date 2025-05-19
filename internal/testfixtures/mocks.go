//go:build test

package testfixtures

import (
	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/sink"
	"github.com/withobsrvr/flowctl/internal/source"
	mockprocessor "github.com/withobsrvr/flowctl/internal/testfixtures/processor"
	mocksink "github.com/withobsrvr/flowctl/internal/testfixtures/sink"
	mocksource "github.com/withobsrvr/flowctl/internal/testfixtures/source"
)

// CreateMockSource creates a new mock source for testing
func CreateMockSource() source.Source {
	return mocksource.NewMockSource()
}

// CreateMockProcessor creates a new mock processor for testing
func CreateMockProcessor(name string) processor.Processor {
	return mockprocessor.NewMockProcessor(name)
}

// CreateMockSink creates a new mock sink for testing
func CreateMockSink() sink.Sink {
	return mocksink.NewMockSink()
}

// AsMockSource attempts to convert a Source to a MockSource for testing
func AsMockSource(src source.Source) (*mocksource.MockSource, bool) {
	mock, ok := src.(*mocksource.MockSource)
	return mock, ok
}

// AsMockProcessor attempts to convert a Processor to a MockProcessor for testing
func AsMockProcessor(proc processor.Processor) (*mockprocessor.MockProcessor, bool) {
	mock, ok := proc.(*mockprocessor.MockProcessor)
	return mock, ok
}

// AsMockSink attempts to convert a Sink to a MockSink for testing
func AsMockSink(snk sink.Sink) (*mocksink.MockSink, bool) {
	mock, ok := snk.(*mocksink.MockSink)
	return mock, ok
}