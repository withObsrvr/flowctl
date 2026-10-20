// Package exitcode defines flowctl's process exit codes and maps errors onto them.
//
// The codes are a public contract: scripts and AI operators branch on them to decide
// whether a failure is worth retrying. They follow the OBSRVR Engineering Standard
// v2.0, rule CLI-15. See docs/adr/0001-cli-output-and-exit-code-contract.md.
//
// Commands should return errors wrapped with Wrap or Errorf so the cause survives to
// the top level, where Execute calls Of to pick the process exit code. An unwrapped
// error exits 1, which is always a safe default: "it failed, cause unclassified".
package exitcode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Exit codes. Codes 1 and 2 follow shell convention; 128+n means the process ended on
// signal n. Codes 10-63 are reserved for domain-specific additions, which must be
// documented in docs/adr/0001-cli-output-and-exit-code-contract.md before first use.
const (
	// OK means the operation succeeded.
	OK = 0
	// Failure means the operation failed for an unclassified reason. Whether it is
	// worth retrying depends on the error message.
	Failure = 1
	// Usage means a bad flag, argument or config file. Never retry.
	Usage = 2
	// NotFound means the named resource does not exist. Never retry.
	NotFound = 3
	// Unauthorized means not authenticated, or authenticated without permission.
	// Never retry.
	Unauthorized = 4
	// Conflict means a failed precondition: already exists, wrong state, version
	// mismatch. Retry only after resolving the conflict.
	Conflict = 5
	// Timeout means a wait expired. Safe to retry.
	Timeout = 6
	// Unavailable means a dependency could not be reached: network, RPC, storage.
	// Retry with backoff.
	Unavailable = 7
	// DataInvalid means validation failed: a gap, a duplicate, a bad checksum, a
	// schema violation. Do not retry; investigate.
	DataInvalid = 8
	// Interrupted means the process shut down cleanly after SIGINT. Safe to retry;
	// it resumes from its checkpoint.
	Interrupted = 130
	// Terminated means the process shut down cleanly after SIGTERM. Safe to retry;
	// it resumes from its checkpoint.
	Terminated = 143
)

// Error is an error carrying the exit code flowctl should exit with.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Wrap tags err with an exit code. It returns nil if err is nil, so it is safe to
// write `return exitcode.Wrap(exitcode.NotFound, err)` on a path that may not have
// failed.
func Wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}

// Errorf formats a new error and tags it with an exit code.
func Errorf(code int, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// Of returns the exit code for err.
//
// An explicit code set by Wrap or Errorf always wins. Otherwise Of infers one from
// well-known error types — gRPC status codes, context deadlines, filesystem errors —
// so that callers get a useful code without every call site having to classify by
// hand. Anything unrecognised is Failure.
func Of(err error) int {
	if err == nil {
		return OK
	}

	// An explicit code anywhere in the chain wins over inference.
	if coded, ok := errors.AsType[*Error](err); ok {
		return coded.Code
	}
	if sig, ok := errors.AsType[*SignalError](err); ok {
		return sig.Code()
	}

	// gRPC is how every remote call in flowctl fails, so classify it precisely.
	// status.FromError only reports ok for real status errors, which is what we
	// want: a plain error that happens to wrap one is still inspected via errors.As
	// by the grpcStatusOf helper below.
	if code, ok := grpcStatusOf(err); ok {
		switch code {
		case codes.OK:
			return OK
		case codes.NotFound:
			return NotFound
		case codes.Unauthenticated, codes.PermissionDenied:
			return Unauthorized
		case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
			return Conflict
		case codes.DeadlineExceeded:
			return Timeout
		case codes.Unavailable:
			return Unavailable
		case codes.InvalidArgument, codes.OutOfRange:
			return Usage
		case codes.DataLoss:
			return DataInvalid
		}
		return Failure
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return Timeout
	case errors.Is(err, fs.ErrNotExist):
		return NotFound
	case errors.Is(err, fs.ErrPermission):
		return Unauthorized
	case errors.Is(err, fs.ErrExist):
		return Conflict
	}

	// context.Canceled is deliberately not mapped to Interrupted. Cancellation has
	// many causes besides a signal, and the signal paths set Interrupted or
	// Terminated explicitly.
	return Failure
}

// grpcStatusOf reports the gRPC code carried by err, if any. It walks the error chain
// rather than calling status.FromError directly, because flowctl wraps RPC errors with
// %w to add context and status.FromError does not unwrap.
func grpcStatusOf(err error) (codes.Code, bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if se, ok := e.(interface{ GRPCStatus() *status.Status }); ok {
			return se.GRPCStatus().Code(), true
		}
	}
	return codes.Unknown, false
}

// FromSignal returns the exit code for a process that shut down cleanly after sig.
// Per CLI-15 the convention is 128+n. Signals flowctl does not handle report Failure,
// since reaching here with one is a bug rather than a clean shutdown.
func FromSignal(sig string) int {
	switch sig {
	case "interrupt", "SIGINT":
		return Interrupted
	case "terminated", "SIGTERM":
		return Terminated
	}
	return Failure
}
