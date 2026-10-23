package sanitize

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Message returns a copy of m with every string field stripped of terminal
// control characters, ready to be printed to a terminal (CLI-22).
//
// Use it on anything that arrived from the control plane. Service IDs, component
// names, descriptions, endpoints, event-type strings, metadata keys and values,
// and relayed error messages are all written by processes flowctl did not start,
// and several commands print them straight to the operator's terminal. One
// crafted value is enough to emit its own OSC sequence.
//
// The original message is not modified: a command that also emits JSON must send
// the unsanitised value, because encoding/json escapes control characters anyway
// and stripping them would corrupt data the consumer is entitled to receive
// intact. Commands rendering through internal/cliout do not need this at all —
// cliout sanitises at the point it formats a cell, which is the better place.
// This exists for the commands not yet converted to it.
//
// Bytes fields are left alone: they carry encoded payloads, not display text.
func Message[T proto.Message](m T) T {
	if !m.ProtoReflect().IsValid() {
		// A typed nil. Returning it unchanged keeps callers from having to
		// nil-check before sanitising.
		return m
	}
	clone := proto.Clone(m)
	sanitizeMessage(clone.ProtoReflect())
	return clone.(T)
}

// sanitizeMessage walks every populated field of m, in place.
func sanitizeMessage(m protoreflect.Message) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			sanitizeMap(fd, v.Map())
		case fd.IsList():
			sanitizeList(fd, v.List())
		case fd.Kind() == protoreflect.StringKind:
			m.Set(fd, protoreflect.ValueOfString(Line(v.String())))
		case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
			sanitizeMessage(v.Message())
		}
		return true
	})
}

func sanitizeList(fd protoreflect.FieldDescriptor, list protoreflect.List) {
	for i := 0; i < list.Len(); i++ {
		switch fd.Kind() {
		case protoreflect.StringKind:
			list.Set(i, protoreflect.ValueOfString(Line(list.Get(i).String())))
		case protoreflect.MessageKind, protoreflect.GroupKind:
			sanitizeMessage(list.Get(i).Message())
		}
	}
}

func sanitizeMap(fd protoreflect.FieldDescriptor, mp protoreflect.Map) {
	keyKind := fd.MapKey().Kind()
	valKind := fd.MapValue().Kind()

	// Keys cannot be rewritten in place, so collect first and rebuild. A
	// hostile component controls metadata keys as well as values, and a key
	// is just as capable of carrying an escape sequence.
	type entry struct {
		key protoreflect.MapKey
		val protoreflect.Value
	}
	var entries []entry
	mp.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		entries = append(entries, entry{k, v})
		return true
	})

	for _, e := range entries {
		newKey := e.key
		if keyKind == protoreflect.StringKind {
			newKey = protoreflect.ValueOfString(Line(e.key.String())).MapKey()
		}

		newVal := e.val
		switch valKind {
		case protoreflect.StringKind:
			newVal = protoreflect.ValueOfString(Line(e.val.String()))
		case protoreflect.MessageKind, protoreflect.GroupKind:
			sanitizeMessage(e.val.Message())
		}

		if newKey.Interface() != e.key.Interface() {
			mp.Clear(e.key)
		}
		mp.Set(newKey, newVal)
	}
}
