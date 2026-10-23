// Package sanitize strips terminal control characters from untrusted strings.
//
// flowctl's control plane accepts registrations from processes it did not start, and
// prints what they send — service names, component metadata, relayed error messages —
// to the operator's terminal. Without stripping, a crafted service name can emit its
// own escape sequences: change the window title, plant a misleading OSC 8 hyperlink,
// or on permissive terminals write to the clipboard with OSC 52.
//
// This implements rule CLI-22 of the OBSRVR Engineering Standard v2.0. The characters
// removed are C0 (U+0000-U+001F) except newline and tab, DEL (U+007F), and C1
// (U+0080-U+009F). Removing ESC (U+001B) is what defeats the injection; the rest of
// the range is removed because it is never legitimate in a name or a message and
// terminals disagree about how to render it.
//
// JSON output does not need this: encoding/json escapes control characters, so they
// cannot terminate a string or reach the terminal as control codes. Applying it anyway
// would silently corrupt data that a consumer is entitled to receive intact.
package sanitize

import (
	"strings"
	"unicode/utf8"
)

// String removes control characters from s, keeping newline and tab.
//
// Invalid UTF-8 is replaced with U+FFFD, so the result is always well-formed: a
// terminal fed a lone continuation byte may otherwise consume the following byte and
// resynchronise somewhere the caller did not intend.
func String(s string) string {
	if isClean(s, true) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			// Either real U+FFFD or a decode failure; both render safely.
			b.WriteRune(utf8.RuneError)
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case isControl(r):
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Line removes control characters from s, including newline and tab.
//
// Use it for anything rendered into a single cell of a table or a single log field,
// where an embedded newline would let untrusted input forge an extra row or log line.
func Line(s string) string {
	if isClean(s, false) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			b.WriteRune(utf8.RuneError)
		case isControl(r):
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Strings applies Line to every element of in, returning a new slice.
func Strings(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = Line(s)
	}
	return out
}

// Map applies Line to every key and value of in, returning a new map. Component
// metadata arrives as a map[string]string and both halves reach the terminal.
func Map(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[Line(k)] = Line(v)
	}
	return out
}

// isControl reports whether r is C0, DEL or C1.
func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// isClean reports whether s needs no rewriting, so the common case allocates nothing.
func isClean(s string, allowWhitespace bool) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if allowWhitespace && (r == '\n' || r == '\t') {
			continue
		}
		if isControl(r) {
			return false
		}
	}
	return true
}
