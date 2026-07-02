package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWithSpinnerNonInteractiveRunsFn(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	called := false
	err := withSpinnerWith(r, w, "Checking latest version", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
}

func TestWithSpinnerNonInteractivePropagatesError(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	wantErr := errors.New("boom")
	err := withSpinnerWith(r, w, "Checking latest version", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestWithSpinnerNonInteractivePrintsLabelAndStatus(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	outR, outW, _ := os.Pipe()
	go func() {
		_ = withSpinnerWith(r, outW, "Checking latest version", func() error { return nil })
		outW.Close()
	}()
	buf := make([]byte, 256)
	n, _ := outR.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "Checking latest version") {
		t.Errorf("expected output to contain label, got %q", out)
	}
}
