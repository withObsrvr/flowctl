package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/controlplane"
	"github.com/withobsrvr/flowctl/internal/model"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// DeliveryError identifies the edge and operation that failed without exposing
// component configuration or credentials.
type DeliveryError struct {
	Pipeline, RunID, From, To, Kind, Operation string
	EventID, EventType                         string
	Err                                        error
}

func (e *DeliveryError) Error() string {
	context := fmt.Sprintf("pipeline %s/%s edge %s %s -> %s %s", e.Pipeline, e.RunID, e.Kind, e.From, e.To, e.Operation)
	if e.EventID != "" {
		context += fmt.Sprintf(" event %s (%s)", e.EventID, e.EventType)
	}
	return context + ": " + e.Err.Error()
}
func (e *DeliveryError) Unwrap() error { return e.Err }

// StreamOrchestrator owns every transport edge until delivery completes or fails.
type StreamOrchestrator struct {
	controlPlane         *controlplane.EmbeddedControlPlane
	controlPlaneEndpoint string
	pipeline             *model.Pipeline
	runID                string
	ctx                  context.Context
	cancel               context.CancelFunc
	group                *errgroup.Group
	connections          map[string]*grpc.ClientConn
	mu                   sync.Mutex
	started              bool
}

func NewStreamOrchestrator(ctx context.Context, cp *controlplane.EmbeddedControlPlane, pipeline *model.Pipeline) *StreamOrchestrator {
	ctx, cancel := context.WithCancel(ctx)
	group, groupCtx := errgroup.WithContext(ctx)
	return &StreamOrchestrator{controlPlane: cp, pipeline: pipeline, ctx: groupCtx, cancel: cancel, group: group, connections: make(map[string]*grpc.ClientConn)}
}

func (s *StreamOrchestrator) edge(from, to, kind, operation string, event *flowctlv1.Event, err error) error {
	e := &DeliveryError{Pipeline: s.pipeline.Metadata.Name, RunID: s.runID, From: from, To: to, Kind: kind, Operation: operation, Err: err}
	if event != nil {
		e.EventID, e.EventType = event.Id, event.Type
	}
	return e
}

func (s *StreamOrchestrator) launch(fn func() error) {
	s.group.Go(func() error {
		err := fn()
		if err != nil && s.ctx.Err() != nil && (errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled || status.Code(err) == codes.DeadlineExceeded) {
			return nil
		}
		return err
	})
}

// WireAll opens the streams and starts supervised forwarding. Wait must be called
// after WireAll, including when wiring fails, to reap already-started edges.
func (s *StreamOrchestrator) WireAll() error {
	if s.started {
		return fmt.Errorf("orchestrator already wired")
	}
	s.started = true
	if len(s.pipeline.Spec.Sources) == 0 || len(s.pipeline.Spec.Sinks) == 0 {
		return fmt.Errorf("pipeline %s requires sources and sinks for stream wiring (or explicit registrationOnly mode)", s.pipeline.Metadata.Name)
	}
	for _, source := range s.pipeline.Spec.Sources {
		target := "sinks"
		if len(s.pipeline.Spec.Processors) > 0 {
			target = s.pipeline.Spec.Processors[0].ID
		}
		srcInfo, err := s.getComponentEndpoint(source.ID)
		if err != nil {
			s.cancel()
			return s.edge(source.ID, target, "source-processor", "registration", nil, err)
		}
		srcConn, err := s.getConnection(srcInfo.Endpoint)
		if err != nil {
			s.cancel()
			return s.edge(source.ID, target, "source-processor", "connect", nil, err)
		}
		params := map[string]string{}
		if v := source.Env["START_LEDGER"]; v != "" {
			params["start_ledger"] = v
		}
		if v := source.Env["END_LEDGER"]; v != "" {
			params["end_ledger"] = v
		}
		sourceStream, err := flowctlv1.NewSourceServiceClient(srcConn).StreamEvents(s.ctx, &flowctlv1.StreamRequest{Params: params})
		if err != nil {
			s.cancel()
			return s.edge(source.ID, target, "source-processor", "open", nil, err)
		}

		// Each source owns its own processor streams. Sharing a bidi stream between
		// sources would allow concurrent Send/Recv and obscure which edge failed.
		streams := make([]flowctlv1.ProcessorService_ProcessClient, len(s.pipeline.Spec.Processors))
		for i, proc := range s.pipeline.Spec.Processors {
			info, err := s.getComponentEndpoint(proc.ID)
			if err != nil {
				s.cancel()
				return s.edge(source.ID, proc.ID, "processor", "registration", nil, err)
			}
			conn, err := s.getConnection(info.Endpoint)
			if err != nil {
				s.cancel()
				return s.edge(source.ID, proc.ID, "processor", "connect", nil, err)
			}
			streams[i], err = flowctlv1.NewProcessorServiceClient(conn).Process(s.ctx)
			if err != nil {
				s.cancel()
				return s.edge(source.ID, proc.ID, "processor", "open", nil, err)
			}
		}
		last := source.ID
		kind := "source-sink"
		if len(streams) > 0 {
			last = s.pipeline.Spec.Processors[len(streams)-1].ID
			kind = "processor-sink"
		}
		sinks := make([]sinkStream, 0, len(s.pipeline.Spec.Sinks))
		for _, sink := range s.pipeline.Spec.Sinks {
			info, err := s.getComponentEndpoint(sink.ID)
			if err != nil {
				s.cancel()
				return s.edge(last, sink.ID, kind, "registration", nil, err)
			}
			conn, err := s.getConnection(info.Endpoint)
			if err != nil {
				s.cancel()
				return s.edge(last, sink.ID, kind, "connect", nil, err)
			}
			stream, err := flowctlv1.NewConsumerServiceClient(conn).Consume(s.ctx)
			if err != nil {
				s.cancel()
				return s.edge(last, sink.ID, kind, "open", nil, err)
			}
			sinks = append(sinks, sinkStream{id: sink.ID, accepted: info.InputEventTypes, stream: stream})
		}
		if len(streams) == 0 {
			bounded := params["end_ledger"] != ""
			s.launch(func() error {
				return s.fanOut(sourceReceiver{sourceStream, s.ctx, bounded}, source.ID, sinks, "source-sink")
			})
			continue
		}
		first := s.pipeline.Spec.Processors[0].ID
		bounded := params["end_ledger"] != ""
		s.launch(func() error { return s.forwardSource(sourceStream, streams[0], source.ID, first, bounded) })
		for i := 0; i < len(streams)-1; i++ {
			from, to := s.pipeline.Spec.Processors[i].ID, s.pipeline.Spec.Processors[i+1].ID
			current, next := streams[i], streams[i+1]
			s.launch(func() error { return s.forward(current, next, from, to) })
		}
		s.launch(func() error { return s.fanOut(streams[len(streams)-1], last, sinks) })
	}
	return nil
}

func (s *StreamOrchestrator) forwardSource(from interface {
	Recv() (*flowctlv1.Event, error)
}, to flowctlv1.ProcessorService_ProcessClient, fromID, toID string, bounded bool) error {
	for {
		event, err := from.Recv()
		if err == io.EOF {
			if !bounded && s.ctx.Err() == nil {
				return s.edge(fromID, toID, "source-processor", "recv (unexpected EOF)", nil, io.ErrUnexpectedEOF)
			}
			if err := to.CloseSend(); err != nil {
				return s.edge(fromID, toID, "source-processor", "close", nil, err)
			}
			return nil
		}
		if err != nil {
			return s.edge(fromID, toID, "source-processor", "recv", nil, err)
		}
		if err := to.Send(event); err != nil {
			return s.edge(fromID, toID, "source-processor", "send", event, err)
		}
	}
}

func (s *StreamOrchestrator) forward(from, to flowctlv1.ProcessorService_ProcessClient, fromID, toID string) error {
	for {
		event, err := from.Recv()
		if err == io.EOF {
			if err := to.CloseSend(); err != nil {
				return s.edge(fromID, toID, "processor-processor", "close", nil, err)
			}
			return nil
		}
		if err != nil {
			return s.edge(fromID, toID, "processor-processor", "recv", nil, err)
		}
		if err := to.Send(event); err != nil {
			return s.edge(fromID, toID, "processor-processor", "send", event, err)
		}
	}
}

type sourceReceiver struct {
	stream interface {
		Recv() (*flowctlv1.Event, error)
	}
	ctx     context.Context
	bounded bool
}

func (r sourceReceiver) Recv() (*flowctlv1.Event, error) {
	event, err := r.stream.Recv()
	if err == io.EOF && r.ctx.Err() != nil {
		return nil, r.ctx.Err()
	}
	if err == io.EOF && !r.bounded {
		return nil, io.ErrUnexpectedEOF
	}
	return event, err
}

type sinkStream struct {
	id       string
	accepted []string
	stream   flowctlv1.ConsumerService_ConsumeClient
}

func accepts(types []string, eventType string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == eventType {
			return true
		}
	}
	return false
}

// Sending synchronously provides backpressure without dropping events. A blocked
// send is interrupted by the pipeline context when another edge fails.
func (s *StreamOrchestrator) fanOut(from interface {
	Recv() (*flowctlv1.Event, error)
}, fromID string, sinks []sinkStream, edgeKind ...string) error {
	kind := "processor-sink"
	if len(edgeKind) > 0 {
		kind = edgeKind[0]
	}
	for {
		event, err := from.Recv()
		if err == io.EOF {
			for _, sink := range sinks {
				response, err := sink.stream.CloseAndRecv()
				if err != nil {
					return s.edge(fromID, sink.id, kind, "close", nil, err)
				}
				if response.GetEventsFailed() > 0 || len(response.GetErrors()) > 0 {
					return s.edge(fromID, sink.id, kind, "close", nil, fmt.Errorf("sink reported %d failed events (%d errors)", response.GetEventsFailed(), len(response.GetErrors())))
				}
			}
			return nil
		}
		if err != nil {
			return s.edge(fromID, "sinks", kind, "recv", nil, err)
		}
		for _, sink := range sinks {
			if accepts(sink.accepted, event.Type) {
				if err := sink.stream.Send(event); err != nil {
					// gRPC may report EOF on Send while the actual server error is
					// available only from CloseAndRecv.
					if errors.Is(err, io.EOF) {
						if _, closeErr := sink.stream.CloseAndRecv(); closeErr != nil {
							err = closeErr
						}
					}
					return s.edge(fromID, sink.id, kind, "send", event, err)
				}
			}
		}
	}
}

func (s *StreamOrchestrator) getComponentEndpoint(id string) (*flowctlv1.ComponentInfo, error) {
	if s.controlPlane != nil {
		services, err := s.controlPlane.GetServiceList()
		if err != nil {
			return nil, err
		}
		for _, service := range services {
			if service.Component != nil && service.Component.Id == id {
				return service.Component, nil
			}
		}
	} else {
		conn, err := s.getConnection(s.controlPlaneEndpoint)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		resp, err := flowctlv1.NewControlPlaneServiceClient(conn).ListComponents(ctx, &flowctlv1.ListComponentsRequest{IncludeUnhealthy: true})
		if err != nil {
			return nil, err
		}
		for _, service := range resp.Components {
			if service.Component != nil && service.Component.Id == id {
				return service.Component, nil
			}
		}
	}
	return nil, fmt.Errorf("component %s not registered", id)
}

func (s *StreamOrchestrator) getConnection(endpoint string) (*grpc.ClientConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conn := s.connections[endpoint]; conn != nil {
		return conn, nil
	}
	const maxMessage = 50 * 1024 * 1024
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessage), grpc.MaxCallSendMsgSize(maxMessage)))
	if err == nil {
		s.connections[endpoint] = conn
	}
	return conn, err
}

// Wait reaps all forwarding loops; cancellation is not a delivery failure.
func (s *StreamOrchestrator) Wait() error {
	err := s.group.Wait()
	if s.ctx.Err() != nil && (errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled || err == nil) {
		return nil
	}
	return err
}

func (s *StreamOrchestrator) Stop() error {
	s.cancel()
	err := s.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, conn := range s.connections {
		_ = conn.Close()
	}
	return err
}
