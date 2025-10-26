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

	// Register all pipeline components (new)
	for _, pipelineConfig := range p.pipeline.Spec.Pipelines {
		// Pipeline components are managed separately by the orchestrator
		// They don't participate in the DAG event flow
		logger.Info("Pipeline component registered",
			zap.String("id", pipelineConfig.ID),
			zap.String("image", pipelineConfig.Image),
			zap.String("type", pipelineConfig.Type))
	}

	// Pre-index sources and processors by ID for direct lookup
	sourceMap := make(map[string]model.Component)
	for _, src := range p.pipeline.Spec.Sources {
		sourceMap[src.ID] = src
	}
	
	processorMap := make(map[string]model.Component)
	for _, proc := range p.pipeline.Spec.Processors {
		processorMap[proc.ID] = proc
	}
	
	// Connect sources to processors
	for _, procConfig := range p.pipeline.Spec.Processors {
		// For each input, find the source or processor
		for _, inputID := range procConfig.Inputs {
			// Check if input is a source
			if srcConfig, exists := sourceMap[inputID]; exists {
				// Connect source to processor
				for _, eventType := range srcConfig.OutputEventTypes {
					if err := p.dag.ConnectSourceToProcessor(srcConfig.ID, procConfig.ID, eventType); err != nil {
						return fmt.Errorf("failed to connect source to processor: %w", err)
					}
				}
				continue // Found a source, no need to check processors
			}
			
			// Check if input is a processor
			if upstreamProc, exists := processorMap[inputID]; exists {
				// Connect upstream processor to this processor
				for _, eventType := range upstreamProc.OutputEventTypes {
					if err := p.dag.Connect(upstreamProc.ID, procConfig.ID, eventType); err != nil {
						return fmt.Errorf("failed to connect processors: %w", err)
					}
				}
			}
		}
	}

	// Connect processors to sinks
	for _, sinkConfig := range p.pipeline.Spec.Sinks {
		// For each input, find the processor
		for _, inputID := range sinkConfig.Inputs {
			// Find the processor that matches this input ID using the pre-indexed map
			if procConfig, exists := processorMap[inputID]; exists {
				// Connect processor to sink
				for _, eventType := range procConfig.OutputEventTypes {
					if err := p.dag.ConnectProcessorToSink(procConfig.ID, sinkConfig.ID, eventType); err != nil {
						return fmt.Errorf("failed to connect processor to sink: %w", err)
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
				case msg, ok := <-ch:
					if !ok {
						logger.Info("Sink channel closed, stopping sink", zap.String("sink_id", sinkID))
						return
					}

					if err := s.Write(ctx, []*processor.Message{msg}); err != nil {
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
	ticker := time.NewTicker(100 * time.Millisecond) // Faster for testing
	defer ticker.Stop()

	// Create a single event immediately for tests to ensure they can proceed
	// This is especially important for tests that expect at least one event
	initialEvent := source.EventEnvelope{
		LedgerSeq: 12345,
		Payload:   []byte(`{"data": "initial test event from ` + s.id + `"}`),
		Cursor:    "test-cursor-initial",
	}

	// Send the initial event, but only if context hasn't been canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- initialEvent:
		// Initial event sent successfully
	}

	// Use an atomic flag to safely track context cancellation
	ctxCanceled := false

	// Setup a goroutine to monitor context cancellation
	ctxDone := ctx.Done()
	go func() {
		<-ctxDone
		ctxCanceled = true
	}()

	for !ctxCanceled {
		select {
		case <-ctxDone:
			ctxCanceled = true
			return ctx.Err()
		case <-ticker.C:
			// Double-check context before creating the event
			if ctxCanceled {
				return ctx.Err()
			}

			// Create a test event
			event := source.EventEnvelope{
				LedgerSeq: 12345,
				Payload:   []byte(`{"data": "test event from ` + s.id + `"}`),
				Cursor:    "test-cursor",
			}
			
			// Try to send, but check context again before sending
			if ctxCanceled {
				return ctx.Err()
			}

			// Use a separate select with timeout to avoid blocking forever
			select {
			case <-ctxDone:
				ctxCanceled = true
				return ctx.Err()
			case out <- event:
				// Event sent successfully
			case <-time.After(50 * time.Millisecond):
				// Couldn't send within timeout, check if context is canceled
				if ctxCanceled {
					return ctx.Err()
				}
			}
		}
	}

	return ctx.Err()
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