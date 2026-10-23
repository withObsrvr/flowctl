// Package apiversion validates the apiVersion field of a pipeline configuration.
//
// It exists because flowctl had three validators that disagreed. The CUE schema in
// schemas/cue/schema.cue enforced `^flowctl/v[0-9]+$`, while the Go loaders in
// internal/runner and internal/translator only checked the field was non-empty. A
// pipeline declaring `apiVersion: v99` was rejected by `flowctl validate` but
// accepted by `flowctl run`, which then interpreted it as v1 — exactly the silent
// reinterpretation VER-7 forbids.
package apiversion

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/withobsrvr/flowctl/internal/exitcode"
)

// Current is the apiVersion this build of flowctl writes.
const Current = "flowctl/v1"

// maxSupportedMajor is the highest pipeline major version this build understands.
// Raising it is a contract change: see docs/adr/ and VER-3.
const maxSupportedMajor = 1

// pattern matches the apiVersion grammar. It must stay in step with the same
// expression in schemas/cue/schema.cue, which is the authoritative schema (DATA-5).
var pattern = regexp.MustCompile(`^flowctl/v([0-9]+)$`)

// Validate reports whether v is an apiVersion this build supports.
//
// Errors are actionable as VER-7 requires: they name what was found, what is
// supported, and what to do about it. All of them classify as exit code 8, data
// validation failed.
func Validate(v string) error {
	if v == "" {
		return exitcode.Errorf(exitcode.DataInvalid,
			"apiVersion is required; expected %q", Current)
	}

	match := pattern.FindStringSubmatch(v)
	if match == nil {
		return exitcode.Errorf(exitcode.DataInvalid,
			"apiVersion %q is not valid; expected %q", v, Current)
	}

	major, err := strconv.Atoi(match[1])
	if err != nil {
		// Unreachable while the pattern only admits digits, but a future
		// loosening of the pattern should fail loudly rather than parse as 0.
		return exitcode.Errorf(exitcode.DataInvalid,
			"apiVersion %q has an unreadable version number", v)
	}

	if major > maxSupportedMajor {
		return exitcode.Errorf(exitcode.DataInvalid,
			"apiVersion %q is newer than this build of flowctl supports (up to %q); upgrade flowctl",
			v, Current)
	}
	if major < 1 {
		return exitcode.Errorf(exitcode.DataInvalid,
			"apiVersion %q is not a released pipeline version; expected %q", v, Current)
	}

	return nil
}

// Major returns the major version number of a valid apiVersion.
func Major(v string) (int, error) {
	if err := Validate(v); err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(pattern.FindStringSubmatch(v)[1])
	return n, nil
}

// ValidateKind checks the resource kind alongside the apiVersion, so the two
// always fail with the same error class.
func ValidateKind(kind string) error {
	if kind != "Pipeline" {
		return exitcode.Errorf(exitcode.DataInvalid,
			"kind must be %q, got %q", "Pipeline", fmt.Sprint(kind))
	}
	return nil
}
