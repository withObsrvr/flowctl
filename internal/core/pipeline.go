package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/sink"
	"github.com/withobsrvr/flowctl/internal/source"
	"github.com/withobsrvr/flowctl/internal/testfixtures"
)

// Pipeline represents a data processing pipeline
type Pipeline struct {
	cfg       *config.Config
	src       source.Source
	procs     []processor.Processor
	snk       sink.Sink
	eventChan chan source.EventEnvelope
	msgChan   chan []*processor.Message
	wg        sync.WaitGroup
}

// NewPipeline creates a new pipeline from configuration
func NewPipeline(cfg *config.Config) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// For now, we'll use mock components from testfixtures
	src := testfixtures.CreateMockSource()
	procs := make([]processor.Processor, len(cfg.Processors))
	for i, p := range cfg.Processors {
		procs[i] = testfixtures.CreateMockProcessor(p.Name)
	}
	snk := sink.NewStdoutSink()

	return &Pipeline{
		cfg:       cfg,
		src:       src,
		procs:     procs,
		snk:       snk,
		eventChan: make(chan source.EventEnvelope, 100),
		msgChan:   make(chan []*processor.Message, 100),
	}, nil
}

// Start begins processing data through the pipeline
func (p *Pipeline) Start(ctx context.Context) error {
	// Start source
	if err := p.src.Open(ctx); err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}

	// Start processors
	for _, proc := range p.procs {
		if err := proc.Init(nil); err != nil {
			return fmt.Errorf("failed to init processor %s: %w", proc.Name(), err)
		}
	}

	// Start sink
	if err := p.snk.Connect(nil); err != nil {
		return fmt.Errorf("failed to connect sink: %w", err)
	}

	// Start pipeline components
	p.wg.Add(3)
	go p.runSource(ctx)
	go p.runProcessors(ctx)
	go p.runSink(ctx)

	return nil
}

// Stop gracefully shuts down the pipeline
func (p *Pipeline) Stop() error {
	close(p.eventChan)
	close(p.msgChan)
	p.wg.Wait()

	if err := p.src.Close(); err != nil {
		return fmt.Errorf("failed to close source: %w", err)
	}

	if err := p.snk.Close(); err != nil {
		return fmt.Errorf("failed to close sink: %w", err)
	}

	return nil
}

func (p *Pipeline) runSource(ctx context.Context) {
	defer p.wg.Done()
	if err := p.src.Events(ctx, p.eventChan); err != nil {
		// TODO: Handle error
	}
}

func (p *Pipeline) runProcessors(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-p.eventChan:
			if !ok {
				return
			}
			var msgs []*processor.Message
			var err error
			for _, proc := range p.procs {
				msgs, err = proc.Process(ctx, &event)
				if err != nil {
					// TODO: Handle error
					continue
				}
			}
			if len(msgs) > 0 {
				p.msgChan <- msgs
			}
		}
	}
}

func (p *Pipeline) runSink(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msgs, ok := <-p.msgChan:
			if !ok {
				return
			}
			if err := p.snk.Write(ctx, msgs); err != nil {
				// TODO: Handle error
			}
		}
	}
}
