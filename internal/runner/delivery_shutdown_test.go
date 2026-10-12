package runner

import (
	"errors"
	"testing"
)

func TestWaitForTerminalOnStop(t *testing.T) {
	failure := errors.New("edge failed")
	t.Run("failure already reported", func(t *testing.T) {
		terminal := make(chan error, 1)
		terminal <- failure
		cancelled := false
		err, reaped := waitForTerminalOnStop(terminal, func() { cancelled = true })
		if !reaped || !errors.Is(err, failure) || cancelled {
			t.Fatalf("err=%v reaped=%t cancelled=%t", err, reaped, cancelled)
		}
	})
	t.Run("failure captured during cancellation", func(t *testing.T) {
		terminal := make(chan error, 1)
		cancelled := false
		err, reaped := waitForTerminalOnStop(terminal, func() { cancelled = true; terminal <- failure })
		if !reaped || !errors.Is(err, failure) || !cancelled {
			t.Fatalf("err=%v reaped=%t cancelled=%t", err, reaped, cancelled)
		}
	})
	t.Run("operator cancellation", func(t *testing.T) {
		terminal := make(chan error, 1)
		err, reaped := waitForTerminalOnStop(terminal, func() { terminal <- nil })
		if !reaped || err != nil {
			t.Fatalf("err=%v reaped=%t", err, reaped)
		}
	})
}
