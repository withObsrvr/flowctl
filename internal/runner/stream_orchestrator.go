package runner

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/controlplane"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
)

// StreamOrchestrator manages data flow between pipeline components
type StreamOrchestrator struct {
	controlPlane     *controlplane.EmbeddedControlPlane
	pipeline         *model.Pipeline
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	connections      map[string]*grpc.ClientConn
	processorStreams map[string]flowctlv1.ProcessorService_ProcessClient
	mu               sync.Mutex
}

// NewStreamOrchestrator creates a new stream orchestrator
func NewStreamOrchestrator(ctx context.Context, controlPlane *controlplane.EmbeddedControlPlane, pipeline *model.Pipeline) *StreamOrchestrator {
	orchCtx, cancel := context.WithCancel(ctx)
	return &StreamOrchestrator{
		controlPlane:     controlPlane,
		pipeline:         pipeline,
		ctx:              orchCtx,
		cancel:           cancel,
		connections:      make(map[string]*grpc.ClientConn),
		processorStreams: make(map[string]flowctlv1.ProcessorService_ProcessClient),
	}
}

// WireAll wires all components in the pipeline together
func (s *StreamOrchestrator) WireAll() error {
	logger.Info("Starting stream orchestrator to wire components")

	// Get all registered components from control plane
	services, err := s.controlPlane.GetServiceList()
	if err != nil {
		return fmt.Errorf("failed to get service list: %w", err)
	}

	// Build a map of component ID to registered info
	componentMap := make(map[string]*flowctlv1.ComponentInfo)
	for _, service := range services {
		if service.Component != nil {
			componentMap[service.Component.Id] = service.Component
		}
	}

	// Wire sources to first processors (or all if no chaining)
	for _, source := range s.pipeline.Spec.Sources {
		_, ok := componentMap[source.ID]
		if !ok {
			logger.Error("Source not found in registered components", zap.String("source", source.ID))
			continue
		}

		// Find first processor (or processors that accept this source's output)
		var targetProcessors []model.Component
		if len(s.pipeline.Spec.Processors) > 0 {
			// CYCLE 2: Use first processor in chain
			targetProcessors = []model.Component{s.pipeline.Spec.Processors[0]}
			logger.Info("Wiring source to first processor in chain",
				zap.String("source", source.ID),
				zap.String("first_processor", targetProcessors[0].ID))
		}

		for _, processor := range targetProcessors {
			logger.Info("Wiring source to processor",
				zap.String("source", source.ID),
				zap.String("processor", processor.ID))

			s.wg.Add(1)
			go func(src model.Component, proc model.Component) {
				defer s.wg.Done()
				processStream, err := s.wireSourceToProcessor(src, proc)
				if err != nil {
					logger.Error("Failed to wire source to processor",
						zap.Error(err),
						zap.String("source", src.ID),
						zap.String("processor", proc.ID))
					return
				}

				// Store the processor stream for downstream components
				s.mu.Lock()
				s.processorStreams[proc.ID] = processStream
				s.mu.Unlock()
			}(source, processor)
		}
	}

	// CYCLE 2: Wire processor chains (proc[i] -> proc[i+1])
	for i := 0; i < len(s.pipeline.Spec.Processors)-1; i++ {
		currentProc := s.pipeline.Spec.Processors[i]
		nextProc := s.pipeline.Spec.Processors[i+1]

		logger.Info("Wiring processor chain",
			zap.String("from", currentProc.ID),
			zap.String("to", nextProc.ID),
			zap.Int("chain_position", i))

		s.wg.Add(1)
		go func(curr, next model.Component) {
			defer s.wg.Done()

			// Wait for current processor stream to be available
			var currentStream flowctlv1.ProcessorService_ProcessClient
			for {
				s.mu.Lock()
				ps, ok := s.processorStreams[curr.ID]
				s.mu.Unlock()

				if ok {
					currentStream = ps
					break
				}

				// Wait for upstream wiring to complete
				select {
				case <-s.ctx.Done():
					return
				case <-time.After(50 * time.Millisecond):
					// Continue loop
				}
			}

			// Wire current processor to next processor
			nextStream, err := s.wireProcessorToProcessor(curr, next, currentStream)
			if err != nil {
				logger.Error("Failed to wire processor chain",
					zap.Error(err),
					zap.String("from", curr.ID),
					zap.String("to", next.ID))
				return
			}

			// Store next processor stream for downstream
			s.mu.Lock()
			s.processorStreams[next.ID] = nextStream
			s.mu.Unlock()
		}(currentProc, nextProc)
	}

	// Wire last processor to sinks (FAN-OUT)
	if len(s.pipeline.Spec.Processors) > 0 && len(s.pipeline.Spec.Sinks) > 0 {
		lastProcessor := s.pipeline.Spec.Processors[len(s.pipeline.Spec.Processors)-1]

		logger.Info("Wiring last processor to sinks (fan-out)",
			zap.String("processor", lastProcessor.ID),
			zap.Int("num_sinks", len(s.pipeline.Spec.Sinks)))

		s.wg.Add(1)
		go func(proc model.Component, sinks []model.Component) {
			defer s.wg.Done()

			// Wait for last processor stream to be available
			var processStream flowctlv1.ProcessorService_ProcessClient
			for {
				s.mu.Lock()
				ps, ok := s.processorStreams[proc.ID]
				s.mu.Unlock()

				if ok {
					processStream = ps
					break
				}

				// Wait for processor chain to complete
				select {
				case <-s.ctx.Done():
					return
				case <-time.After(50 * time.Millisecond):
					// Continue loop
				}
			}

			// CYCLE 2: Fan out to ALL sinks in parallel (broadcast pattern)
			if err := s.fanOutToSinks(proc, sinks, processStream); err != nil {
				logger.Error("Failed to fan out to sinks",
					zap.Error(err),
					zap.String("processor", proc.ID))
			}
		}(lastProcessor, s.pipeline.Spec.Sinks)
	}

	logger.Info("Stream orchestrator wired all components")
	return nil
}

// wireSourceToProcessor connects a source to a processor
// Returns the processor stream so it can be shared with downstream components
func (s *StreamOrchestrator) wireSourceToProcessor(source, processor model.Component) (flowctlv1.ProcessorService_ProcessClient, error) {
	// Get source endpoint from control plane
	sourceInfo, err := s.getComponentEndpoint(source.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source endpoint: %w", err)
	}

	// Get processor endpoint from control plane
	processorInfo, err := s.getComponentEndpoint(processor.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get processor endpoint: %w", err)
	}

	// Connect to source
	sourceConn, err := s.getConnection(sourceInfo.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source: %w", err)
	}

	// Connect to processor
	processorConn, err := s.getConnection(processorInfo.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to processor: %w", err)
	}

	// Create clients
	sourceClient := flowctlv1.NewSourceServiceClient(sourceConn)
	processorClient := flowctlv1.NewProcessorServiceClient(processorConn)

	// Set large message limits for all streams (50MB)
	maxMsgSize := 50 * 1024 * 1024 // 50MB

	logger.Info("Creating source stream with large message limits",
		zap.Int("maxMsgSize", maxMsgSize),
		zap.String("source", source.ID))

	// Build params from source component's environment variables
	params := make(map[string]string)

	// Pass START_LEDGER and END_LEDGER as stream parameters
	if startLedger, ok := source.Env["START_LEDGER"]; ok {
		params["start_ledger"] = startLedger
		logger.Info("Passing start_ledger to source stream", zap.String("start_ledger", startLedger))
	}
	if endLedger, ok := source.Env["END_LEDGER"]; ok {
		params["end_ledger"] = endLedger
		logger.Info("Passing end_ledger to source stream", zap.String("end_ledger", endLedger))
	}

	// Start streaming from source (limits set on connection)
	stream, err := sourceClient.StreamEvents(s.ctx, &flowctlv1.StreamRequest{
		Params: params,
		Resume: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start source stream: %w", err)
	}

	logger.Info("Creating processor stream with large message limits",
		zap.Int("maxMsgSize", maxMsgSize),
		zap.String("processor", processor.ID))

	// Create bidirectional processor stream (limits set on connection)
	processStream, err := processorClient.Process(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start processor stream: %w", err)
	}

	logger.Info("Started streaming from source to processor",
		zap.String("source", source.ID),
		zap.String("processor", processor.ID))

	// Read from source and write to processor in one goroutine
	go func() {
		for {
			envelope, err := stream.Recv()
			if err == io.EOF {
				logger.Info("Source stream ended", zap.String("source", source.ID))
				processStream.CloseSend()
				return
			}
			if err != nil {
				logger.Error("Error receiving from source", zap.Error(err), zap.String("source", source.ID))
				processStream.CloseSend()
				return
			}

			// Send event to processor
			if err := processStream.Send(envelope); err != nil {
				logger.Error("Error sending to processor", zap.Error(err), zap.String("processor", processor.ID))
				processStream.CloseSend()
				return
			}
		}
	}()

	// Return the processor stream so it can be read by downstream components
	return processStream, nil
}

// wireProcessorToProcessor connects one processor to another processor (for chaining)
// Returns the next processor's stream for further chaining
func (s *StreamOrchestrator) wireProcessorToProcessor(currentProc, nextProc model.Component, currentStream flowctlv1.ProcessorService_ProcessClient) (flowctlv1.ProcessorService_ProcessClient, error) {
	// Get next processor endpoint from control plane
	nextProcInfo, err := s.getComponentEndpoint(nextProc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next processor endpoint: %w", err)
	}

	// Connect to next processor
	nextProcConn, err := s.getConnection(nextProcInfo.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to next processor: %w", err)
	}

	// Create next processor client
	nextProcClient := flowctlv1.NewProcessorServiceClient(nextProcConn)

	// Create bidirectional stream for next processor
	nextProcessStream, err := nextProcClient.Process(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start next processor stream: %w", err)
	}

	logger.Info("Started processor chain link",
		zap.String("from", currentProc.ID),
		zap.String("to", nextProc.ID))

	// Forward events from current processor to next processor
	go func() {
		for {
			envelope, err := currentStream.Recv()
			if err == io.EOF {
				logger.Info("Processor stream ended in chain", zap.String("processor", currentProc.ID))
				nextProcessStream.CloseSend()
				return
			}
			if err != nil {
				logger.Error("Error receiving from processor in chain", zap.Error(err), zap.String("processor", currentProc.ID))
				nextProcessStream.CloseSend()
				return
			}

			// Send event to next processor
			if err := nextProcessStream.Send(envelope); err != nil {
				logger.Error("Error sending to next processor in chain", zap.Error(err), zap.String("next_processor", nextProc.ID))
				nextProcessStream.CloseSend()
				return
			}
		}
	}()

	// Return the next processor stream for further chaining or sink wiring
	return nextProcessStream, nil
}

// fanOutToSinks broadcasts events from a processor to multiple sinks in parallel
// CYCLE 2: Supports event type-based routing
func (s *StreamOrchestrator) fanOutToSinks(processor model.Component, sinks []model.Component, processStream flowctlv1.ProcessorService_ProcessClient) error {
	// Create sink connections and streams
	type sinkConnection struct {
		sink              model.Component
		stream            flowctlv1.ConsumerService_ConsumeClient
		acceptedEventTypes []string // CYCLE 2: Event type filtering
	}

	var sinkConns []sinkConnection

	// Connect to all sinks first
	for _, sink := range sinks {
		sinkInfo, err := s.getComponentEndpoint(sink.ID)
		if err != nil {
			logger.Error("Failed to get sink endpoint for fan-out", zap.Error(err), zap.String("sink", sink.ID))
			continue
		}

		sinkConn, err := s.getConnection(sinkInfo.Endpoint)
		if err != nil {
			logger.Error("Failed to connect to sink for fan-out", zap.Error(err), zap.String("sink", sink.ID))
			continue
		}

		sinkClient := flowctlv1.NewConsumerServiceClient(sinkConn)
		consumeStream, err := sinkClient.Consume(s.ctx)
		if err != nil {
			logger.Error("Failed to start consumer stream for fan-out", zap.Error(err), zap.String("sink", sink.ID))
			continue
		}

		// CYCLE 2: Get accepted event types from sink registration
		acceptedTypes := sinkInfo.InputEventTypes
		if len(acceptedTypes) > 0 {
			logger.Info("Sink accepts specific event types",
				zap.String("sink", sink.ID),
				zap.Strings("event_types", acceptedTypes))
		} else {
			logger.Info("Sink accepts all event types",
				zap.String("sink", sink.ID))
		}

		sinkConns = append(sinkConns, sinkConnection{
			sink:              sink,
			stream:            consumeStream,
			acceptedEventTypes: acceptedTypes,
		})
		logger.Info("Connected sink for fan-out", zap.String("sink", sink.ID))
	}

	if len(sinkConns) == 0 {
		return fmt.Errorf("no sinks successfully connected for fan-out")
	}

	logger.Info("Fan-out ready",
		zap.String("processor", processor.ID),
		zap.Int("num_sinks", len(sinkConns)))

	// Read from processor and broadcast to all sinks
	go func() {
		eventsSent := make(map[string]int64) // Track events sent per sink
		eventsFiltered := make(map[string]int64) // Track events filtered per sink

		for {
			envelope, err := processStream.Recv()
			if err == io.EOF {
				logger.Info("Processor stream ended, closing all sinks", zap.String("processor", processor.ID))
				// Log routing statistics
				for _, sc := range sinkConns {
					logger.Info("Sink routing statistics",
						zap.String("sink", sc.sink.ID),
						zap.Int64("events_sent", eventsSent[sc.sink.ID]),
						zap.Int64("events_filtered", eventsFiltered[sc.sink.ID]))
				}
				// Close all sink streams
				for _, sc := range sinkConns {
					sc.stream.CloseAndRecv()
				}
				return
			}
			if err != nil {
				logger.Error("Error receiving from processor for fan-out", zap.Error(err), zap.String("processor", processor.ID))
				// Close all sink streams
				for _, sc := range sinkConns {
					sc.stream.CloseAndRecv()
				}
				return
			}

			// CYCLE 2: Broadcast to matching sinks based on event type
			// Each sink gets its own goroutine to avoid blocking others
			for _, sc := range sinkConns {
				// Check if sink accepts this event type
				if !s.sinkAcceptsEventType(sc.acceptedEventTypes, envelope.Type) {
					eventsFiltered[sc.sink.ID]++
					continue
				}

				eventsSent[sc.sink.ID]++
				go func(sinkConn sinkConnection, event *flowctlv1.Event) {
					if err := sinkConn.stream.Send(event); err != nil {
						logger.Error("Error sending to sink in fan-out",
							zap.Error(err),
							zap.String("sink", sinkConn.sink.ID),
							zap.String("event_id", event.Id),
							zap.String("event_type", event.Type))
						// Don't stop other sinks if one fails
					}
				}(sc, envelope)
			}
		}
	}()

	return nil
}

// sinkAcceptsEventType checks if a sink accepts a given event type
// If acceptedTypes is empty, sink accepts all event types
func (s *StreamOrchestrator) sinkAcceptsEventType(acceptedTypes []string, eventType string) bool {
	// Empty list means accept all types
	if len(acceptedTypes) == 0 {
		return true
	}

	// Check if event type is in accepted list
	for _, acceptedType := range acceptedTypes {
		if acceptedType == eventType {
			return true
		}
	}

	return false
}

// wireProcessorToSink connects a processor to a sink using an existing processor stream
func (s *StreamOrchestrator) wireProcessorToSink(processor, sink model.Component, processStream flowctlv1.ProcessorService_ProcessClient) error {
	// Get sink endpoint from control plane
	sinkInfo, err := s.getComponentEndpoint(sink.ID)
	if err != nil {
		return fmt.Errorf("failed to get sink endpoint: %w", err)
	}

	// Connect to sink
	sinkConn, err := s.getConnection(sinkInfo.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to connect to sink: %w", err)
	}

	// Create sink client
	sinkClient := flowctlv1.NewConsumerServiceClient(sinkConn)

	// Start consumer stream (limits set on connection)
	consumeStream, err := sinkClient.Consume(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to start consumer stream: %w", err)
	}

	logger.Info("Started streaming from processor to sink",
		zap.String("processor", processor.ID),
		zap.String("sink", sink.ID))

	// Forward events from processor to sink
	go func() {
		for {
			envelope, err := processStream.Recv()
			if err == io.EOF {
				logger.Info("Processor stream ended", zap.String("processor", processor.ID))
				consumeStream.CloseAndRecv()
				return
			}
			if err != nil {
				logger.Error("Error receiving from processor", zap.Error(err), zap.String("processor", processor.ID))
				consumeStream.CloseAndRecv()
				return
			}

			// Send event to sink
			if err := consumeStream.Send(envelope); err != nil {
				logger.Error("Error sending to sink", zap.Error(err), zap.String("sink", sink.ID))
				continue
			}
		}
	}()

	return nil
}

// getComponentEndpoint gets the endpoint of a component from the control plane
func (s *StreamOrchestrator) getComponentEndpoint(componentID string) (*flowctlv1.ComponentInfo, error) {
	// Query control plane for component list
	services, err := s.controlPlane.GetServiceList()
	if err != nil {
		return nil, err
	}

	// Find the component by ID
	for _, service := range services {
		if service.Component != nil && service.Component.Id == componentID {
			return service.Component, nil
		}
	}

	return nil, fmt.Errorf("component %s not found", componentID)
}

// getConnection gets or creates a gRPC connection to an endpoint
func (s *StreamOrchestrator) getConnection(endpoint string) (*grpc.ClientConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we already have a connection
	if conn, ok := s.connections[endpoint]; ok {
		logger.Info("Reusing existing connection",
			zap.String("endpoint", endpoint))
		return conn, nil
	}

	// Create new connection with larger message limits (50MB)
	maxMsgSize := 50 * 1024 * 1024 // 50MB

	logger.Info("Creating NEW connection with message limits",
		zap.String("endpoint", endpoint),
		zap.Int("maxMsgSize", maxMsgSize))

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMsgSize),
			grpc.MaxCallSendMsgSize(maxMsgSize),
		),
	)
	if err != nil {
		return nil, err
	}

	s.connections[endpoint] = conn
	logger.Info("Connection created and cached",
		zap.String("endpoint", endpoint))
	return conn, nil
}

// findDownstreamProcessorsFromRegistry finds processors that accept the given event types from control plane registry
func (s *StreamOrchestrator) findDownstreamProcessorsFromRegistry(outputTypes []string, componentMap map[string]*flowctlv1.ComponentInfo) []model.Component {
	var processors []model.Component

	for _, processor := range s.pipeline.Spec.Processors {
		processorInfo, ok := componentMap[processor.ID]
		if !ok {
			continue
		}

		// Check if processor accepts any of the output types
		for _, outputType := range outputTypes {
			for _, inputType := range processorInfo.InputEventTypes {
				if outputType == inputType {
					processors = append(processors, processor)
					break
				}
			}
		}
	}

	return processors
}

// findDownstreamSinksFromRegistry finds sinks that accept the given event types from control plane registry
func (s *StreamOrchestrator) findDownstreamSinksFromRegistry(outputTypes []string, componentMap map[string]*flowctlv1.ComponentInfo) []model.Component {
	var sinks []model.Component

	for _, sink := range s.pipeline.Spec.Sinks {
		sinkInfo, ok := componentMap[sink.ID]
		if !ok {
			continue
		}

		// Check if sink accepts any of the output types
		for _, outputType := range outputTypes {
			for _, inputType := range sinkInfo.InputEventTypes {
				if outputType == inputType {
					sinks = append(sinks, sink)
					break
				}
			}
		}
	}

	return sinks
}

// Stop stops the stream orchestrator and closes all connections
func (s *StreamOrchestrator) Stop() error {
	logger.Info("Stopping stream orchestrator")

	s.cancel()
	s.wg.Wait()

	// Close all connections
	s.mu.Lock()
	defer s.mu.Unlock()

	for endpoint, conn := range s.connections {
		logger.Info("Closing connection", zap.String("endpoint", endpoint))
		conn.Close()
	}

	return nil
}
