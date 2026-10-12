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
		err := waitForTerminalOnStop(terminal, func() { cancelled = true })
		if !errors.Is(err, failure) || cancelled {
			t.Fatalf("err=%v cancelled=%t", err, cancelled)
		}
	})
	t.Run("failure captured during cancellation", func(t *testing.T) {
		terminal := make(chan error, 1)
		cancelled := false
		err := waitForTerminalOnStop(terminal, func() { cancelled = true; terminal <- failure })
		if !errors.Is(err, failure) || !cancelled {
			t.Fatalf("err=%v cancelled=%t", err, cancelled)
		}
	})
	t.Run("operator cancellation", func(t *testing.T) {
		terminal := make(chan error, 1)
		err := waitForTerminalOnStop(terminal, func() { terminal <- nil })
		if err != nil {
			t.Fatal(err)
		}
	})
}
