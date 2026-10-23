package exitcode

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// SignalError reports that the process shut down cleanly in response to a signal.
//
// It is not a failure. CLI-11 requires a tool to stop starting new work, finish or
// roll back the current unit, write its checkpoint and exit 130 or 143 — so the
// process must exit non-zero to say "interrupted, resume me", without printing an
// error the operator did not cause.
type SignalError struct {
	Signal os.Signal
}

func (e *SignalError) Error() string {
	return fmt.Sprintf("interrupted by %s", e.Signal)
}

// Code returns 130 for SIGINT and 143 for SIGTERM, per the 128+n convention.
func (e *SignalError) Code() int {
	switch e.Signal {
	case os.Interrupt, syscall.SIGINT:
		return Interrupted
	case syscall.SIGTERM:
		return Terminated
	}
	return Failure
}

// Signal returns an error representing a clean shutdown in response to sig.
func Signal(sig os.Signal) error {
	return &SignalError{Signal: sig}
}

// IsSignal reports whether err represents a clean signal shutdown rather than a
// failure. Execute uses it to decide whether to print an error message.
func IsSignal(err error) bool {
	_, ok := errors.AsType[*SignalError](err)
	return ok
}
