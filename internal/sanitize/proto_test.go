package sanitize

import (
	"strings"
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/protobuf/proto"
)

// hostile is the kind of value a component can register under. It sets the
// window title, then erases the line so the forgery is not visible.
const hostile = "svc\x1b]0;pwned\x07\x1b[2Kinnocent"

func TestMessageSanitisesStringFields(t *testing.T) {
	in := &flowctlpb.ServiceInfo{
		ServiceId: hostile,
	}
	out := Message(in)

	if strings.ContainsRune(out.ServiceId, 0x1b) {
		t.Errorf("ServiceId still contains ESC: %q", out.ServiceId)
	}
	// ESC and BEL are removed; the inert remnants of the sequences stay, which
	// is the point — they are now ordinary printable text.
	if out.ServiceId != "svc]0;pwned[2Kinnocent" {
		t.Errorf("ServiceId = %q", out.ServiceId)
	}
	for _, r := range out.ServiceId {
		if isControl(r) {
			t.Errorf("control char %U survived", r)
		}
	}
}

func TestMessageDoesNotMutateTheOriginal(t *testing.T) {
	// A command that also emits JSON must send the unsanitised value, because
	// encoding/json escapes control characters anyway and stripping them would
	// corrupt data the consumer is entitled to receive intact.
	in := &flowctlpb.ServiceInfo{ServiceId: hostile}
	_ = Message(in)

	if in.ServiceId != hostile {
		t.Errorf("the input was mutated: %q", in.ServiceId)
	}
}

func TestMessageSanitisesRepeatedStrings(t *testing.T) {
	in := &flowctlpb.ServiceInfo{
		ServiceId:        "ok",
		InputEventTypes:  []string{"clean.v1", "evil\x1b[2K.v1"},
		OutputEventTypes: []string{"out\x07.v1"},
	}
	out := Message(in)

	for _, list := range [][]string{out.InputEventTypes, out.OutputEventTypes} {
		for _, v := range list {
			for _, r := range v {
				if isControl(r) {
					t.Errorf("control char %U survived in %q", r, v)
				}
			}
		}
	}
	if out.InputEventTypes[0] != "clean.v1" {
		t.Errorf("clean value was altered: %q", out.InputEventTypes[0])
	}
}

func TestMessageSanitisesMapKeysAndValues(t *testing.T) {
	// A hostile component controls metadata keys as well as values, and a key
	// is just as capable of carrying an escape sequence.
	in := &flowctlpb.ServiceInfo{
		ServiceId: "ok",
		Metadata: map[string]string{
			"ke\x1by": "value",
			"network": "test\x1b]52;c;x\x07net",
		},
	}
	out := Message(in)

	for k, v := range out.Metadata {
		for _, r := range k {
			if isControl(r) {
				t.Errorf("control char %U survived in key %q", r, k)
			}
		}
		for _, r := range v {
			if isControl(r) {
				t.Errorf("control char %U survived in value %q", r, v)
			}
		}
	}
	if _, ok := out.Metadata["key"]; !ok {
		t.Errorf("sanitised key missing: %#v", out.Metadata)
	}
	if got := out.Metadata["network"]; got != "test]52;c;xnet" {
		t.Errorf("network = %q", got)
	}
}

func TestMessageRecursesIntoNestedMessages(t *testing.T) {
	in := &flowctlv1.ComponentStatusResponse{
		Component: &flowctlv1.ComponentInfo{
			Id:       hostile,
			Name:     "name\x1b[1A",
			Endpoint: "host\x00:8080",
		},
	}
	out := Message(in)

	for _, v := range []string{out.Component.Id, out.Component.Name, out.Component.Endpoint} {
		for _, r := range v {
			if isControl(r) {
				t.Fatalf("control char %U survived in %q", r, v)
			}
		}
	}
}

func TestMessageLeavesBytesFieldsAlone(t *testing.T) {
	// Bytes carry encoded payloads, not display text. Stripping them would
	// corrupt a ledger.
	payload := []byte{0x00, 0x1b, 0x7f, 0x9b, 0xff}
	in := &flowctlpb.Event{
		EventType: "evil\x1b[2K",
		Payload:   payload,
	}
	out := Message(in)

	if strings.ContainsRune(out.EventType, 0x1b) {
		t.Errorf("EventType not sanitised: %q", out.EventType)
	}
	if string(out.Payload) != string(payload) {
		t.Errorf("Payload was altered: % x", out.Payload)
	}
}

func TestMessageHandlesNil(t *testing.T) {
	// So callers need not nil-check before sanitising.
	var in *flowctlpb.ServiceInfo
	if out := Message(in); out != nil {
		t.Errorf("Message(nil) = %v, want nil", out)
	}
}

func TestMessageLeavesCleanMessagesEqual(t *testing.T) {
	in := &flowctlpb.ServiceInfo{
		ServiceId:       "stellar-live-source",
		InputEventTypes: []string{"stellar.ledger.v1"},
		Metadata:        map[string]string{"network": "testnet"},
	}
	out := Message(in)
	if !proto.Equal(in, out) {
		t.Errorf("a clean message was altered:\n in: %v\nout: %v", in, out)
	}
}
