//go:build !windows

package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/xpty"
)

func TestPickFromListWithUsesSuppliedPTY(t *testing.T) {
	pty, err := xpty.NewUnixPty(80, 24)
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	defer pty.Close()

	type result struct {
		selected string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		selected, err := PickFromListWith(
			pty.Slave(), pty.Slave(), "Select", []Item{{ID: "1", Label: "first"}},
		)
		done <- result{selected: selected, err: err}
	}()

	// A single-option picker starts on that option; Enter confirms it. Writing
	// to the PTY master proves the form reads the explicit slave, not os.Stdin.
	time.Sleep(50 * time.Millisecond)
	if _, err := pty.Master().Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.selected != "1" {
			t.Fatalf("selected = %q, want 1", got.selected)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("picker did not consume input from the supplied PTY")
	}
}
