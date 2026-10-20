package exitcode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOfClassifiesGRPCStatus(t *testing.T) {
	// Every remote call flowctl makes fails as a gRPC status, so this mapping is
	// what gives a script something to branch on (CLI-15).
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, OK},
		{codes.NotFound, NotFound},
		{codes.Unauthenticated, Unauthorized},
		{codes.PermissionDenied, Unauthorized},
		{codes.AlreadyExists, Conflict},
		{codes.FailedPrecondition, Conflict},
		{codes.Aborted, Conflict},
		{codes.DeadlineExceeded, Timeout},
		{codes.Unavailable, Unavailable},
		{codes.InvalidArgument, Usage},
		{codes.OutOfRange, Usage},
		{codes.DataLoss, DataInvalid},
		{codes.Internal, Failure},
		{codes.Unknown, Failure},
	}

	for _, tc := range tests {
		t.Run(tc.code.String(), func(t *testing.T) {
			err := status.Error(tc.code, "boom")
			if got := Of(err); got != tc.want {
				t.Errorf("Of(%v) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

func TestOfUnwrapsWrappedGRPCStatus(t *testing.T) {
	// Commands add context with %w. status.FromError does not unwrap, so a naive
	// implementation would silently classify every wrapped RPC failure as 1.
	err := fmt.Errorf("listing services: %w",
		fmt.Errorf("dialling: %w", status.Error(codes.Unavailable, "refused")))
	if got := Of(err); got != Unavailable {
		t.Errorf("Of(wrapped Unavailable) = %d, want %d", got, Unavailable)
	}
}

func TestExplicitCodeWinsOverInference(t *testing.T) {
	// A command that knows better than the transport must be able to say so.
	err := Wrap(DataInvalid, status.Error(codes.Unavailable, "refused"))
	if got := Of(err); got != DataInvalid {
		t.Errorf("Of = %d, want %d", got, DataInvalid)
	}
}

func TestExplicitCodeFoundThroughWrapping(t *testing.T) {
	err := fmt.Errorf("outer: %w", Errorf(NotFound, "no such run"))
	if got := Of(err); got != NotFound {
		t.Errorf("Of = %d, want %d", got, NotFound)
	}
}

func TestOfStandardErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, OK},
		{"deadline", context.DeadlineExceeded, Timeout},
		{"wrapped deadline", fmt.Errorf("waiting: %w", context.DeadlineExceeded), Timeout},
		{"missing file", fmt.Errorf("open x: %w", fs.ErrNotExist), NotFound},
		{"permission", fmt.Errorf("open x: %w", fs.ErrPermission), Unauthorized},
		{"exists", fmt.Errorf("mkdir: %w", fs.ErrExist), Conflict},
		{"plain", errors.New("something broke"), Failure},
		// Cancellation has causes other than a signal; the signal paths set
		// Interrupted explicitly rather than relying on inference.
		{"cancelled", context.Canceled, Failure},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Of(tc.err); got != tc.want {
				t.Errorf("Of(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestOfRealFilesystemError(t *testing.T) {
	_, err := os.Open("/definitely/not/a/real/path/pipeline.yaml")
	if err == nil {
		t.Fatal("expected an error opening a missing file")
	}
	if got := Of(fmt.Errorf("reading pipeline: %w", err)); got != NotFound {
		t.Errorf("Of = %d, want %d", got, NotFound)
	}
}

func TestWrapNilStaysNil(t *testing.T) {
	// So `return exitcode.Wrap(code, err)` is safe on a path that may not fail.
	if err := Wrap(NotFound, nil); err != nil {
		t.Errorf("Wrap(_, nil) = %v, want nil", err)
	}
}

func TestErrorPreservesMessageAndChain(t *testing.T) {
	inner := errors.New("inner")
	err := Wrap(Conflict, fmt.Errorf("outer: %w", inner))
	if err.Error() != "outer: inner" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, inner) {
		t.Error("errors.Is could not reach the wrapped cause")
	}
}

func TestFromSignal(t *testing.T) {
	tests := []struct {
		sig  string
		want int
	}{
		{"interrupt", Interrupted},
		{"SIGINT", Interrupted},
		{"terminated", Terminated},
		{"SIGTERM", Terminated},
		{"SIGHUP", Failure},
	}
	for _, tc := range tests {
		if got := FromSignal(tc.sig); got != tc.want {
			t.Errorf("FromSignal(%q) = %d, want %d", tc.sig, got, tc.want)
		}
	}
}

func TestCodesMatchTheStandard(t *testing.T) {
	// CLI-15 fixes these numbers. Changing one is a breaking change to a public
	// contract, so it should fail here first and force an ADR.
	for name, pair := range map[string][2]int{
		"OK":           {OK, 0},
		"Failure":      {Failure, 1},
		"Usage":        {Usage, 2},
		"NotFound":     {NotFound, 3},
		"Unauthorized": {Unauthorized, 4},
		"Conflict":     {Conflict, 5},
		"Timeout":      {Timeout, 6},
		"Unavailable":  {Unavailable, 7},
		"DataInvalid":  {DataInvalid, 8},
		"Interrupted":  {Interrupted, 130},
		"Terminated":   {Terminated, 143},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %d, the standard says %d", name, pair[0], pair[1])
		}
	}
}
