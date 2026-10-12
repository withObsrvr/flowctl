package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/model"
	"google.golang.org/grpc"
)

type fakeProcessor struct {
	grpc.ClientStream
	events                     []*flowctlv1.Event
	recvErr, sendErr, closeErr error
	sent                       int
}

func (f *fakeProcessor) Recv() (*flowctlv1.Event, error) {
	if len(f.events) > 0 {
		e := f.events[0]
		f.events = f.events[1:]
		return e, nil
	}
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	return nil, io.EOF
}
func (f *fakeProcessor) Send(*flowctlv1.Event) error { f.sent++; return f.sendErr }
func (f *fakeProcessor) CloseSend() error            { return f.closeErr }

type fakeSink struct {
	grpc.ClientStream
	sent              int
	sendErr, closeErr error
	response          *flowctlv1.ConsumeResponse
	nilResponse       bool
	blocked, release  chan struct{}
}

func (f *fakeSink) Send(*flowctlv1.Event) error {
	if f.blocked != nil {
		close(f.blocked)
		<-f.release
	}
	f.sent++
	return f.sendErr
}
func (f *fakeSink) CloseAndRecv() (*flowctlv1.ConsumeResponse, error) {
	if f.nilResponse {
		return nil, f.closeErr
	}
	if f.response != nil {
		return f.response, f.closeErr
	}
	return &flowctlv1.ConsumeResponse{EventsConsumed: int64(f.sent)}, f.closeErr
}

func testOrchestrator(t *testing.T) *StreamOrchestrator {
	t.Helper()
	s := NewStreamOrchestrator(context.Background(), nil, &model.Pipeline{})
	t.Cleanup(func() { _ = s.Stop() })
	return s
}
func TestSourceTermination(t *testing.T) {
	failure := errors.New("source failed")
	for _, tc := range []struct {
		name    string
		events  int
		recvErr error
		bounded bool
		want    error
	}{
		{"before first", 0, failure, false, failure},
		{"after event", 1, failure, false, failure},
		{"bounded completion", 1, nil, true, nil},
		{"unbounded EOF", 1, nil, false, io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testOrchestrator(t)
			from := &fakeProcessor{recvErr: tc.recvErr}
			if tc.events > 0 {
				from.events = []*flowctlv1.Event{{Id: "1", Type: "ledger"}}
			}
			to := &fakeProcessor{}
			err := s.forwardSource(from, to, "source", "processor", tc.bounded)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) || to.sent != tc.events {
				t.Fatalf("err=%v sent=%d", err, to.sent)
			}
		})
	}
}

func TestForwardingFailures(t *testing.T) {
	failure := errors.New("synthetic failure")
	for _, tc := range []struct {
		name                 string
		upstream, downstream fakeProcessor
		operation            string
	}{
		{"receive", fakeProcessor{recvErr: failure}, fakeProcessor{}, "recv"},
		{"send", fakeProcessor{events: []*flowctlv1.Event{{Id: "1", Type: "ledger"}}}, fakeProcessor{sendErr: failure}, "send event 1 (ledger)"},
		{"close", fakeProcessor{}, fakeProcessor{closeErr: failure}, "close"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testOrchestrator(t)
			err := s.forward(&tc.upstream, &tc.downstream, "processor-a", "processor-b")
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "processor-a -> processor-b") || !strings.Contains(err.Error(), tc.operation) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
func TestFanOutFailuresAndCompletion(t *testing.T) {
	failure := errors.New("sink failure")
	for _, tc := range []struct {
		name                       string
		recvErr, sendErr, closeErr error
		want                       string
	}{
		{name: "processor receive", recvErr: failure, want: "recv"},
		{name: "sink send", sendErr: failure, want: "send event 1"},
		{name: "sink close", closeErr: failure, want: "close"},
		{name: "success"},
		{name: "sink reports failed event", want: "failed 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testOrchestrator(t)
			source := &fakeProcessor{events: []*flowctlv1.Event{{Id: "1", Type: "ledger"}}, recvErr: tc.recvErr}
			healthy := &fakeSink{}
			sink := &fakeSink{sendErr: tc.sendErr, closeErr: tc.closeErr}
			if tc.name == "sink reports failed event" {
				sink.response = &flowctlv1.ConsumeResponse{EventsFailed: 1}
			}
			err := s.fanOut(source, "processor", []sinkStream{{id: "healthy", stream: healthy}, {id: "failing", stream: sink}})
			if tc.want == "" {
				if err != nil || healthy.sent != 1 || sink.sent != 1 {
					t.Fatalf("completion: err=%v healthy=%d sink=%d", err, healthy.sent, sink.sent)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || (tc.name != "sink reports failed event" && !errors.Is(err, failure)) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
func TestFanOutAcknowledgements(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *flowctlv1.ConsumeResponse
		missing  bool
		want     string
	}{
		{"short", &flowctlv1.ConsumeResponse{}, false, "expected 1 acknowledged 0"},
		{"excess", &flowctlv1.ConsumeResponse{EventsConsumed: 2}, false, "expected 1 acknowledged 2"},
		{"failed", &flowctlv1.ConsumeResponse{EventsConsumed: 1, EventsFailed: 1}, false, "failed 1"},
		{"errors", &flowctlv1.ConsumeResponse{EventsConsumed: 1, Errors: []string{"failure"}}, false, "errors 1"},
		{"nil", nil, true, "missing response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testOrchestrator(t)
			sink := &fakeSink{response: tc.response, nilResponse: tc.missing}
			err := s.fanOut(&fakeProcessor{events: []*flowctlv1.Event{{Type: "matching"}}}, "source", []sinkStream{{id: "bad-sink", stream: sink}})
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "bad-sink") {
				t.Fatalf("err=%v", err)
			}
		})
	}
	for _, events := range [][]*flowctlv1.Event{nil, {{Type: "ignored"}}, {{Type: "matching"}, {Type: "ignored"}}} {
		s := testOrchestrator(t)
		a, b := &fakeSink{}, &fakeSink{}
		err := s.fanOut(&fakeProcessor{events: events}, "source", []sinkStream{{id: "a", accepted: []string{"matching"}, stream: a}, {id: "b", accepted: []string{"ignored"}, stream: b}})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceBoundedDeclaration(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"END_LEDGER": "0"}, false},
		{map[string]string{"END_LEDGER": "120"}, false},
		{map[string]string{"FLOWCTL_SOURCE_BOUNDED": "true"}, true},
	} {
		if got := sourceBounded(tc.env); got != tc.want {
			t.Fatalf("%v: %v", tc.env, got)
		}
	}
}

func TestFanOutBackpressureDoesNotDropEvent(t *testing.T) {
	s := testOrchestrator(t)
	slow := &fakeSink{blocked: make(chan struct{}), release: make(chan struct{})}
	healthy := &fakeSink{}
	done := make(chan error, 1)
	go func() {
		done <- s.fanOut(&fakeProcessor{events: []*flowctlv1.Event{{Id: "1", Type: "test.event"}}}, "processor", []sinkStream{{id: "slow", stream: slow}, {id: "healthy", stream: healthy}})
	}()
	<-slow.blocked
	select {
	case err := <-done:
		t.Fatalf("fan-out completed while sink was blocked: %v", err)
	default:
	}
	close(slow.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if slow.sent != 1 || healthy.sent != 1 {
		t.Fatalf("lost event: slow=%d healthy=%d", slow.sent, healthy.sent)
	}
}

func TestWaitCancelsOtherEdges(t *testing.T) {
	s := testOrchestrator(t)
	failure := errors.New("source failed")
	exited := make(chan struct{})
	s.launch(func() error { <-s.ctx.Done(); close(exited); return s.ctx.Err() })
	s.launch(func() error { return s.edge("source", "processor", "source-processor", "recv", nil, failure) })
	if err := s.Wait(); !errors.Is(err, failure) {
		t.Fatalf("wait = %v", err)
	}
	<-exited
}
