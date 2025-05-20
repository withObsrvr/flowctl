package core

import (
	"context"
	"fmt"
	"time"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/sink"
	"github.com/withobsrvr/flowctl/internal/source"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// DAGPipeline represents a data processing pipeline implemented with a DAG of buffered channels
type DAGPipeline struct {
	cfg         *config.Config
	pipeline    *model.Pipeline
	dag         *DAG
	sources     map[string]source.Source
	processors  map[string]processor.Processor
	sinks       map[string]sink.Sink
	sinkChan    chan *processor.Message
}

// NewDAGPipeline creates a new pipeline from configuration
func NewDAGPipeline(cfg *config.Config, pipeline *model.Pipeline) (*DAGPipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create DAG with default buffer size
	dag := NewDAG(1000) // Adjust buffer size as needed
	sources := make(map[string]source.Source)
	processors := make(map[string]processor.Processor)
	sinks := make(map[string]sink.Sink)

	return &DAGPipeline{
		cfg:        cfg,
		pipeline:   pipeline,
		dag:        dag,
		sources:    sources,
		processors: processors,
		sinks:      sinks,
	}, nil
}

// buildPipelineGraph builds the DAG based on the pipeline configuration
func (p *DAGPipeline) buildPipelineGraph() error {
	// Register all sources
	for _, srcConfig := range p.pipeline.Spec.Sources {
		src, err := createSource(srcConfig) // This would create the actual source based on config
		if err != nil {
			return fmt.Errorf("failed to create source %s: %w", srcConfig.ID, err)
		}

		p.sources[srcConfig.ID] = src
		if err := p.dag.AddSource(srcConfig.ID, src); err != nil {
			return fmt.Errorf("failed to add source to DAG: %w", err)
		}
	}

	// Register all processors
	for _, procConfig := range p.pipeline.Spec.Processors {
		proc, err := createProcessor(procConfig) // This would create the actual processor based on config
		if err != nil {
			return fmt.Errorf("failed to create processor %s: %w", procConfig.ID, err)
		}

		p.processors[procConfig.ID] = proc
		
		// Determine input event types from the inputs
		inputTypes := determineInputEventTypes(procConfig, p.pipeline)
		
		if err := p.dag.AddProcessor(procConfig.ID, proc, inputTypes); err != nil {
			return fmt.Errorf("failed to add processor to DAG: %w", err)
		}
	}

	// Register all sinks
	for _, sinkConfig := range p.pipeline.Spec.Sinks {
		snk, err := createSink(sinkConfig) // This would create the actual sink based on config
		if err != nil {
			return fmt.Errorf("failed to create sink %s: %w", sinkConfig.ID, err)
		}

		p.sinks[sinkConfig.ID] = snk
		
		// Create a sink channel for each sink
		p.dag.AddSinkChannel(sinkConfig.ID)
	}

	// Connect sources to processors
	for _, procConfig := range p.pipeline.Spec.Processors {
		// For each input, find the source
		for _, inputID := range procConfig.Inputs {
			// Check if input is a source
			for _, srcConfig := range p.pipeline.Spec.Sources {
				if srcConfig.ID == inputID {
					// Connect source to processor
					// We need to determine the event types this source produces
					for _, eventType := range srcConfig.OutputEventTypes {
						if err := p.dag.ConnectSourceToProcessor(srcConfig.ID, procConfig.ID, eventType); err != nil {
							return fmt.Errorf("failed to connect source to processor: %w", err)
						}
					}
				}
			}

			// Check if input is a processor
			for _, upstreamProcConfig := range p.pipeline.Spec.Processors {
				if upstreamProcConfig.ID == inputID {
					// Connect upstream processor to this processor
					// We need to determine the event types the upstream processor produces
					for _, eventType := range upstreamProcConfig.OutputEventTypes {
						if err := p.dag.Connect(upstreamProcConfig.ID, procConfig.ID, eventType); err != nil {
							return fmt.Errorf("failed to connect processors: %w", err)
						}
					}
				}
			}
		}
	}

	// Connect processors to sinks
	for _, sinkConfig := range p.pipeline.Spec.Sinks {
		// For each input, find the processor
		for _, inputID := range sinkConfig.Inputs {
			// Find the processor that matches this input ID
			for _, procConfig := range p.pipeline.Spec.Processors {
				if procConfig.ID == inputID {
					// Connect processor to sink
					// We need to determine the event types the processor produces
					for _, eventType := range procConfig.OutputEventTypes {
						if err := p.dag.ConnectProcessorToSink(procConfig.ID, sinkConfig.ID, eventType); err != nil {
							return fmt.Errorf("failed to connect processor to sink: %w", err)
						}
					}
				}
			}
		}
	}

	return nil
}

// Start begins processing data through the pipeline
func (p *DAGPipeline) Start(ctx context.Context) error {
	logger.Info("Starting DAG pipeline")

	// Build the pipeline graph
	if err := p.buildPipelineGraph(); err != nil {
		return fmt.Errorf("failed to build pipeline graph: %w", err)
	}

	// Initialize and start sinks
	for id, snk := range p.sinks {
		if err := snk.Connect(nil); err != nil {
			return fmt.Errorf("failed to connect sink %s: %w", id, err)
		}

		// Start a goroutine to process messages from the sink channel
		sinkChan := p.dag.SinkChannels[id]
		go func(sinkID string, s sink.Sink, ch chan *processor.Message) {
			ctx := context.Background() // Use a separate context for sinks
			
			for {
				select {
				case <-ctx.Done():
					return
				case msgs, ok := <-ch:
					if !ok {
						logger.Info("Sink channel closed, stopping sink", zap.String("sink_id", sinkID))
						return
					}

					if err := s.Write(ctx, []*processor.Message{msgs}); err != nil {
						logger.Error("Error writing to sink",
							zap.String("sink_id", sinkID),
							zap.Error(err))
					}
				}
			}
		}(id, snk, sinkChan)
	}

	// Start the DAG
	if err := p.dag.Start(ctx); err != nil {
		return fmt.Errorf("failed to start DAG: %w", err)
	}

	logger.Info("DAG pipeline started")
	return nil
}

// Stop gracefully shuts down the pipeline
func (p *DAGPipeline) Stop() error {
	logger.Info("Stopping DAG pipeline")
	
	// Stop the DAG first
	if err := p.dag.Stop(); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Close all sinks
	for id, snk := range p.sinks {
		if err := snk.Close(); err != nil {
			logger.Error("Error closing sink",
				zap.String("sink_id", id),
				zap.Error(err))
		}
	}

	logger.Info("DAG pipeline stopped")
	return nil
}

// Helper functions

// createSource creates a source from a component configuration
func createSource(component model.Component) (source.Source, error) {
	// This is a placeholder - in a real implementation, you would
	// create the appropriate source type based on the component configuration
	// For now, we'll use a mock source for testing
	return &mockSource{id: component.ID}, nil
}

// createProcessor creates a processor from a component configuration
func createProcessor(component model.Component) (processor.Processor, error) {
	// This is a placeholder - in a real implementation, you would
	// create the appropriate processor type based on the component configuration
	// For now, we'll use a mock processor for testing
	return &mockProcessor{id: component.ID}, nil
}

// createSink creates a sink from a component configuration
func createSink(component model.Component) (sink.Sink, error) {
	// This is a placeholder - in a real implementation, you would
	// create the appropriate sink type based on the component configuration
	// For now, we'll use the stdout sink for testing
	return sink.NewStdoutSink(), nil
}

// determineInputEventTypes determines the input event types for a processor
func determineInputEventTypes(processor model.Component, pipeline *model.Pipeline) []string {
	var inputTypes []string
	
	// For each input, find the corresponding component and get its output event types
	for _, inputID := range processor.Inputs {
		// Check sources
		for _, src := range pipeline.Spec.Sources {
			if src.ID == inputID {
				// Add output event types from this source
				for _, eventType := range src.OutputEventTypes {
					inputTypes = append(inputTypes, eventType)
				}
			}
		}
		
		// Check processors
		for _, proc := range pipeline.Spec.Processors {
			if proc.ID == inputID {
				// Add output event types from this processor
				for _, eventType := range proc.OutputEventTypes {
					inputTypes = append(inputTypes, eventType)
				}
			}
		}
	}
	
	return inputTypes
}

// Mock implementations for testing

// mockSource implements source.Source for testing
type mockSource struct {
	id string
}

func (s *mockSource) Open(ctx context.Context) error {
	return nil
}

func (s *mockSource) Events(ctx context.Context, out chan<- source.EventEnvelope) error {
	// Generate events periodically for testing
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Create a test event
			event := source.EventEnvelope{
				LedgerSeq: 12345,
				Payload:   []byte(`{"data": "test event from ` + s.id + `"}`),
				Cursor:    "test-cursor",
			}
			select {
			case <-ctx.Done():
				return nil
			case out <- event:
				// Event sent successfully
			}
		}
	}
}

func (s *mockSource) Close() error {
	return nil
}

func (s *mockSource) Healthy() error {
	return nil
}

// mockProcessor implements processor.Processor for testing
type mockProcessor struct {
	id string
}

func (p *mockProcessor) Init(cfg map[string]any) error {
	return nil
}

func (p *mockProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
	// Create a mock processed message
	msg := &processor.Message{
		Type:    "processed-event",
		Payload: []byte(`{"processed_data": "processed by ` + p.id + `", "original": "` + string(e.Payload) + `"}`),
	}
	return []*processor.Message{msg}, nil
}

func (p *mockProcessor) Flush(ctx context.Context) error {
	return nil
}

func (p *mockProcessor) Name() string {
	return p.id
}