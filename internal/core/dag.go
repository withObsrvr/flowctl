package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/withobsrvr/flowctl/internal/processor"
	"github.com/withobsrvr/flowctl/internal/source"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// ProcessorNode represents a node in the processor DAG
type ProcessorNode struct {
	Processor       processor.Processor
	InputChannels   map[string]chan *processor.Message
	OutputChannels  map[string][]chan *processor.Message
	InputEventTypes []string
	WaitGroup       *sync.WaitGroup
}

// DAG represents a directed acyclic graph of processors
type DAG struct {
	Nodes          map[string]*ProcessorNode
	Sources        map[string]source.Source
	SourceChannels map[string]chan source.EventEnvelope
	SinkChannels   map[string]chan *processor.Message
	BufferSize     int
	WaitGroup      sync.WaitGroup
	Context        context.Context
	CancelFunc     context.CancelFunc
	mu             sync.RWMutex
}

// NewDAG creates a new DAG for processor chaining
func NewDAG(bufferSize int) *DAG {
	ctx, cancel := context.WithCancel(context.Background())
	return &DAG{
		Nodes:          make(map[string]*ProcessorNode),
		Sources:        make(map[string]source.Source),
		SourceChannels: make(map[string]chan source.EventEnvelope),
		SinkChannels:   make(map[string]chan *processor.Message),
		BufferSize:     bufferSize,
		Context:        ctx,
		CancelFunc:     cancel,
	}
}

// AddProcessor adds a processor to the DAG
func (d *DAG) AddProcessor(id string, proc processor.Processor, inputTypes []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Nodes[id]; exists {
		return fmt.Errorf("processor with ID %s already exists in the DAG", id)
	}

	node := &ProcessorNode{
		Processor:       proc,
		InputChannels:   make(map[string]chan *processor.Message),
		OutputChannels:  make(map[string][]chan *processor.Message),
		InputEventTypes: inputTypes,
		WaitGroup:       &sync.WaitGroup{},
	}

	// Create input channels for each event type
	for _, eventType := range inputTypes {
		node.InputChannels[eventType] = make(chan *processor.Message, d.BufferSize)
	}

	d.Nodes[id] = node
	return nil
}

// AddSource adds a source to the DAG
func (d *DAG) AddSource(id string, src source.Source) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Sources[id]; exists {
		return fmt.Errorf("source with ID %s already exists in the DAG", id)
	}

	d.Sources[id] = src
	d.SourceChannels[id] = make(chan source.EventEnvelope, d.BufferSize)
	return nil
}

// AddSinkChannel adds a sink channel to the DAG
func (d *DAG) AddSinkChannel(id string) chan *processor.Message {
	d.mu.Lock()
	defer d.mu.Unlock()

	channel := make(chan *processor.Message, d.BufferSize)
	d.SinkChannels[id] = channel
	return channel
}

// Connect connects a processor output to another processor's input
func (d *DAG) Connect(fromID, toID string, eventType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fromNode, exists := d.Nodes[fromID]
	if !exists {
		return fmt.Errorf("processor %s not found", fromID)
	}

	toNode, exists := d.Nodes[toID]
	if !exists {
		return fmt.Errorf("processor %s not found", toID)
	}

	// Check if the destination processor accepts this event type
	eventTypeAccepted := false
	for _, inputType := range toNode.InputEventTypes {
		if inputType == eventType {
			eventTypeAccepted = true
			break
		}
	}

	if !eventTypeAccepted {
		return fmt.Errorf("processor %s does not accept event type %s", toID, eventType)
	}

	// Get or create the input channel for this event type in the destination node
	inputChan, exists := toNode.InputChannels[eventType]
	if !exists {
		inputChan = make(chan *processor.Message, d.BufferSize)
		toNode.InputChannels[eventType] = inputChan
	}

	// Add the input channel to the output channels of the source node
	fromNode.OutputChannels[eventType] = append(fromNode.OutputChannels[eventType], inputChan)

	logger.Info("Connected processors in DAG",
		zap.String("from", fromID),
		zap.String("to", toID),
		zap.String("event_type", eventType))

	return nil
}

// ConnectSourceToProcessor connects a source to a processor
func (d *DAG) ConnectSourceToProcessor(sourceID, processorID string, eventType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, exists := d.Sources[sourceID]
	if !exists {
		return fmt.Errorf("source %s not found", sourceID)
	}

	procNode, exists := d.Nodes[processorID]
	if !exists {
		return fmt.Errorf("processor %s not found", processorID)
	}

	// Check if the processor accepts this event type
	eventTypeAccepted := false
	for _, inputType := range procNode.InputEventTypes {
		if inputType == eventType {
			eventTypeAccepted = true
			break
		}
	}

	if !eventTypeAccepted {
		return fmt.Errorf("processor %s does not accept event type %s", processorID, eventType)
	}

	// Create a source event adapter to convert source events to processor messages
	d.WaitGroup.Add(1)
	go func() {
		defer d.WaitGroup.Done()
		for {
			select {
			case <-d.Context.Done():
				return
			case evt, ok := <-d.SourceChannels[sourceID]:
				if !ok {
					return
				}

				// Create a message from the event
				msg := &processor.Message{
					Type:    eventType,
					Payload: evt.Payload,
				}

				// Send to all input channels of this event type
				if inputChan, ok := procNode.InputChannels[eventType]; ok {
					select {
					case <-d.Context.Done():
						return
					case inputChan <- msg:
						// Successfully sent to processor
					}
				}
			}
		}
	}()

	logger.Info("Connected source to processor in DAG",
		zap.String("source", sourceID),
		zap.String("processor", processorID),
		zap.String("event_type", eventType))

	return nil
}

// ConnectProcessorToSink connects a processor to a sink
func (d *DAG) ConnectProcessorToSink(processorID, sinkID string, eventType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	procNode, exists := d.Nodes[processorID]
	if !exists {
		return fmt.Errorf("processor %s not found", processorID)
	}

	sinkChan, exists := d.SinkChannels[sinkID]
	if !exists {
		return fmt.Errorf("sink %s not found", sinkID)
	}

	// Add the sink channel to the output channels of the processor
	procNode.OutputChannels[eventType] = append(procNode.OutputChannels[eventType], sinkChan)

	logger.Info("Connected processor to sink in DAG",
		zap.String("processor", processorID),
		zap.String("sink", sinkID),
		zap.String("event_type", eventType))

	return nil
}

// Start starts the DAG and all processors
func (d *DAG) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Start all sources
	for id, src := range d.Sources {
		if err := src.Open(ctx); err != nil {
			return fmt.Errorf("failed to open source %s: %w", id, err)
		}

		d.WaitGroup.Add(1)
		go func(srcID string, s source.Source) {
			defer d.WaitGroup.Done()
			if err := s.Events(ctx, d.SourceChannels[srcID]); err != nil {
				logger.Error("Error from source",
					zap.String("source_id", srcID),
					zap.Error(err))
			}
		}(id, src)

		logger.Info("Started source", zap.String("source_id", id))
	}

	// Initialize all processors
	for id, node := range d.Nodes {
		if err := node.Processor.Init(nil); err != nil {
			return fmt.Errorf("failed to initialize processor %s: %w", id, err)
		}

		// Start processor goroutines for each input channel
		for eventType, inputChan := range node.InputChannels {
			node.WaitGroup.Add(1)
			go d.runProcessor(id, node, eventType, inputChan)
		}

		logger.Info("Started processor", zap.String("processor_id", id))
	}

	return nil
}

// runProcessor processes messages from an input channel
func (d *DAG) runProcessor(id string, node *ProcessorNode, eventType string, inputChan chan *processor.Message) {
	defer node.WaitGroup.Done()

	for {
		select {
		case <-d.Context.Done():
			logger.Info("Stopping processor due to context done",
				zap.String("processor_id", id),
				zap.String("event_type", eventType))
			return
		case msg, ok := <-inputChan:
			if !ok {
				logger.Info("Input channel closed, stopping processor",
					zap.String("processor_id", id),
					zap.String("event_type", eventType))
				return
			}

			// Create event envelope from message for processor
			envelope := &source.EventEnvelope{
				Payload: msg.Payload,
			}

			// Process the message
			outputs, err := node.Processor.Process(d.Context, envelope)
			if err != nil {
				logger.Error("Error processing message",
					zap.String("processor_id", id),
					zap.String("event_type", eventType),
					zap.Error(err))
				continue
			}

			// Forward outputs to connected processors/sinks
			for _, output := range outputs {
				// Find channels to send this output to
				if outputChans, ok := node.OutputChannels[output.Type]; ok {
					for _, outChan := range outputChans {
						select {
						case <-d.Context.Done():
							return
						case outChan <- output:
							// Successfully sent to next processor or sink
						}
					}
				}
			}
		}
	}
}

// Stop gracefully shuts down the DAG
func (d *DAG) Stop() error {
	logger.Info("Stopping DAG")

	// Cancel the context to signal all goroutines to stop
	d.CancelFunc()

	// Close all source channels
	for id, ch := range d.SourceChannels {
		close(ch)
		logger.Debug("Closed source channel", zap.String("source_id", id))
	}

	// Wait for all main goroutines to finish
	d.WaitGroup.Wait()
	logger.Debug("All DAG goroutines completed")

	// Close all sources
	for id, src := range d.Sources {
		if err := src.Close(); err != nil {
			logger.Error("Error closing source",
				zap.String("source_id", id),
				zap.Error(err))
		}
	}

	// Wait for all processor goroutines to finish
	for id, node := range d.Nodes {
		node.WaitGroup.Wait()
		logger.Debug("All processor goroutines completed", zap.String("processor_id", id))
	}

	// Close sink channels
	for id, ch := range d.SinkChannels {
		close(ch)
		logger.Debug("Closed sink channel", zap.String("sink_id", id))
	}

	logger.Info("DAG stopped")
	return nil
}