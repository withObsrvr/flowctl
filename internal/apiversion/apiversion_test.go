package apiversion

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/withobsrvr/flowctl/internal/exitcode"
)

func TestValidateAcceptsCurrent(t *testing.T) {
	if err := Validate(Current); err != nil {
		t.Fatalf("Validate(%q) = %v, want nil", Current, err)
	}
}

func TestValidateRejects(t *testing.T) {
	// VER-7: an unsupported version is rejected with an actionable error, never
	// silently reinterpreted. Before this package, "v1" and "v99" both loaded and
	// ran as v1.
	tests := []struct {
		name     string
		in       string
		contains string
	}{
		{"empty", "", "required"},
		{"bare v1, the old loose form", "v1", "not valid"},
		{"no group", "v1/flowctl", "not valid"},
		{"wrong group", "nebu/v1", "not valid"},
		{"no version", "flowctl/", "not valid"},
		{"non-numeric", "flowctl/vX", "not valid"},
		{"trailing content", "flowctl/v1beta", "not valid"},
		{"leading space", " flowctl/v1", "not valid"},
		{"future major", "flowctl/v2", "upgrade flowctl"},
		{"far future major", "flowctl/v99", "upgrade flowctl"},
		{"zero", "flowctl/v0", "not a released"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.in)
			if err == nil {
				t.Fatalf("Validate(%q) = nil, want an error", tc.in)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("Validate(%q) = %q, want it to mention %q", tc.in, err, tc.contains)
			}
			// Bad configuration is a data validation failure, not a generic one.
			if got := exitcode.Of(err); got != exitcode.DataInvalid {
				t.Errorf("Validate(%q) exit code = %d, want %d", tc.in, got, exitcode.DataInvalid)
			}
		})
	}
}

func TestErrorsAreActionable(t *testing.T) {
	// VER-7 requires the error to name the supported range or an upgrade hint.
	err := Validate("flowctl/v2")
	if !strings.Contains(err.Error(), Current) {
		t.Errorf("error should name the supported version: %q", err)
	}
}

func TestMajor(t *testing.T) {
	got, err := Major("flowctl/v1")
	if err != nil || got != 1 {
		t.Errorf("Major = (%d, %v), want (1, nil)", got, err)
	}
	if _, err := Major("v1"); err == nil {
		t.Error("Major should reject an invalid apiVersion")
	}
}

func TestValidateKind(t *testing.T) {
	if err := ValidateKind("Pipeline"); err != nil {
		t.Errorf("ValidateKind(Pipeline) = %v", err)
	}
	err := ValidateKind("pipeline")
	if err == nil {
		t.Fatal("ValidateKind should be case-sensitive")
	}
	if got := exitcode.Of(err); got != exitcode.DataInvalid {
		t.Errorf("exit code = %d, want %d", got, exitcode.DataInvalid)
	}
}

// TestPatternMatchesCUESchema guards the duplication between this package and the
// authoritative CUE schema. DATA-5 allows generated or checked copies of a schema,
// never hand-edited ones that can drift; this is the check that keeps them honest.
func TestPatternMatchesCUESchema(t *testing.T) {
	path := filepath.Join("..", "..", "schemas", "cue", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s: %v", path, err)
	}

	// apiVersion: string & =~"^flowctl/v[0-9]+$"
	re := regexp.MustCompile(`apiVersion:\s*string\s*&\s*=~"([^"]+)"`)
	match := re.FindSubmatch(data)
	if match == nil {
		t.Fatalf("could not find the apiVersion constraint in %s; if the schema was "+
			"restructured, update this test and apiversion.pattern together", path)
	}

	// The Go pattern adds a capture group around the version number so Major can
	// read it. Strip it before comparing: the two must describe the same language,
	// not be byte-identical.
	ours := strings.NewReplacer("(", "", ")", "").Replace(pattern.String())
	if got := string(match[1]); got != ours {
		t.Errorf("CUE schema constrains apiVersion to %q but this package uses %q; "+
			"they must match (DATA-5)", got, ours)
	}
}

// TestCurrentSatisfiesPattern catches a Current constant that was bumped without
// widening maxSupportedMajor.
func TestCurrentSatisfiesPattern(t *testing.T) {
	major, err := Major(Current)
	if err != nil {
		t.Fatalf("Current = %q is not valid: %v", Current, err)
	}
	if major != maxSupportedMajor {
		t.Errorf("Current is %q (major %d) but maxSupportedMajor is %d",
			Current, major, maxSupportedMajor)
	}
}

func FuzzValidateNeverPanics(f *testing.F) {
	// TEST-3: apiVersion comes from a user-supplied config file.
	for _, seed := range []string{
		"", "flowctl/v1", "v1", "flowctl/v", "flowctl/v99999999999999999999",
		"flowctl/v-1", "flowctl/v1\n", "\x00", "FLOWCTL/V1",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		err := Validate(in)
		if err == nil {
			// The real invariant: anything accepted must resolve to a major
			// version this build supports. It need not be spelled exactly like
			// Current — the authoritative CUE pattern admits zero-padded forms
			// such as "flowctl/v001", and the two validators must agree.
			major, mErr := Major(in)
			if mErr != nil {
				t.Fatalf("Validate(%q) accepted an input Major rejects: %v", in, mErr)
			}
			if major < 1 || major > maxSupportedMajor {
				t.Fatalf("Validate(%q) accepted unsupported major %d", in, major)
			}
		}
	})
}
