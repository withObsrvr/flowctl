package sanitize

import (
	"strings"
	"testing"
)

func TestStringStripsInjectionAttempts(t *testing.T) {
	// Each input is something a hostile component could put in a service name or
	// relay in an error message. None of the output may contain ESC or any other
	// control character, or the terminal will act on it (CLI-22).
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OSC 7501 status spoof",
			input: "svc\x1b]7501;state=done\x1b\\",
			want:  "svc]7501;state=done\\",
		},
		{
			name:  "OSC 8 misleading hyperlink",
			input: "ledger\x1b]8;;https://evil.example\x07click\x1b]8;;\x07",
			want:  "ledger]8;;https://evil.exampleclick]8;;",
		},
		{
			name:  "OSC 52 clipboard write",
			input: "\x1b]52;c;cm0gLXJmIH4=\x07",
			want:  "]52;c;cm0gLXJmIH4=",
		},
		{
			name:  "window title set",
			input: "\x1b]0;pwned\x07name",
			want:  "]0;pwnedname",
		},
		{
			name:  "CSI cursor movement and erase",
			input: "real\x1b[2K\x1b[1Afake",
			want:  "real[2K[1Afake",
		},
		{
			name:  "carriage return overwrite",
			input: "HEALTHY\rUNHEALTHY",
			want:  "HEALTHYUNHEALTHY",
		},
		{
			name:  "DEL and C1 CSI",
			input: "a\x7fbc",
			want:  "abc",
		},
		{
			name:  "bell",
			input: "ding\x07",
			want:  "ding",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := String(tc.input)
			if got != tc.want {
				t.Errorf("String(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if strings.ContainsRune(got, 0x1b) {
				t.Errorf("String(%q) left an ESC in %q", tc.input, got)
			}
			for _, r := range got {
				if isControl(r) {
					t.Errorf("String(%q) left control char %U in %q", tc.input, r, got)
				}
			}
		})
	}
}

func TestStringKeepsNewlineAndTab(t *testing.T) {
	in := "line one\nline two\tcolumn"
	if got := String(in); got != in {
		t.Errorf("String(%q) = %q, want it unchanged", in, got)
	}
}

func TestLineStripsNewlineAndTab(t *testing.T) {
	// A newline in a table cell lets untrusted input forge an extra row.
	in := "svc\nFAKE-ID\tHEALTHY"
	want := "svcFAKE-IDHEALTHY"
	if got := Line(in); got != want {
		t.Errorf("Line(%q) = %q, want %q", in, got, want)
	}
}

func TestPreservesLegitimateText(t *testing.T) {
	// Sanitising must not mangle ordinary content, including non-ASCII, which
	// appears in asset codes and contract metadata.
	for _, in := range []string{
		"stellar-live-source",
		"ttp-processor-v2",
		"Test SDF Network ; September 2015",
		"café résumé",
		"日本語",
		"emoji 🟢 status",
		"GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR",
	} {
		if got := String(in); got != in {
			t.Errorf("String(%q) = %q, want it unchanged", in, got)
		}
		if got := Line(in); got != in {
			t.Errorf("Line(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestInvalidUTF8BecomesReplacementChar(t *testing.T) {
	// A lone continuation byte can make a terminal swallow the next byte and
	// resynchronise somewhere unintended.
	got := Line("ok\xffbad")
	if !strings.ContainsRune(got, '�') {
		t.Errorf("Line(%q) = %q, want a U+FFFD replacement", "ok\xffbad", got)
	}
	if strings.ContainsRune(got, 0xff) {
		t.Errorf("Line left a raw 0xff byte in %q", got)
	}
}

func TestCleanStringsAreNotReallocated(t *testing.T) {
	// The common case is clean input; it should not pay for a rebuild.
	in := "already-clean-value"
	if got := String(in); got != in {
		t.Fatalf("String(%q) = %q", in, got)
	}
	if got := Line(in); got != in {
		t.Fatalf("Line(%q) = %q", in, got)
	}
}

func TestMapSanitisesKeysAndValues(t *testing.T) {
	in := map[string]string{"na\x1bme": "va\x1blue"}
	got := Map(in)
	if _, ok := got["name"]; !ok {
		t.Errorf("Map did not sanitise the key: %#v", got)
	}
	if got["name"] != "value" {
		t.Errorf("Map did not sanitise the value: %#v", got)
	}
	if Map(nil) != nil {
		t.Error("Map(nil) should stay nil")
	}
}

func TestStrings(t *testing.T) {
	got := Strings([]string{"a\x1bb", "clean"})
	if got[0] != "ab" || got[1] != "clean" {
		t.Errorf("Strings = %#v", got)
	}
}

func FuzzLineNeverEmitsControlCharacters(f *testing.F) {
	// TEST-3: the sanitiser is the last line of defence for untrusted input, so
	// its invariant is fuzzed rather than only sampled.
	for _, seed := range []string{
		"", "plain", "\x1b]52;c;x\x07", "\xff\xfe", "tab\there", "日本語", "\x9b31m",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		for _, r := range Line(in) {
			if isControl(r) {
				t.Fatalf("Line(%q) emitted control char %U", in, r)
			}
		}
		for _, r := range String(in) {
			if r != '\n' && r != '\t' && isControl(r) {
				t.Fatalf("String(%q) emitted control char %U", in, r)
			}
		}
	})
}
