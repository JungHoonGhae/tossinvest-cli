package tui

import (
	"errors"
	"os"
	"testing"
)

func TestIsInteractiveFalseForPipe(t *testing.T) {
	r, _, _ := os.Pipe()
	defer r.Close()
	if IsInteractive(r, os.Stdout) {
		t.Fatal("pipe stdin must be non-interactive")
	}
}

func TestIsInteractiveFalseForNilFiles(t *testing.T) {
	if IsInteractive(nil, os.Stdout) {
		t.Fatal("nil input must not be interactive")
	}
	if IsInteractive(os.Stdin, nil) {
		t.Fatal("nil output must not be interactive")
	}
}

func TestSelectWithNonInteractiveErrors(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	_, err := selectWith(r, w, "choose", []string{"a", "b"})
	if !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("expected ErrNotInteractive, got %v", err)
	}
}

func TestPasswordWithNonInteractiveErrors(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	_, err := passwordWith(r, w, "secret")
	if !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("expected ErrNotInteractive, got %v", err)
	}
}

func TestConfirmWithNonInteractiveErrors(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	_, err := confirmWith(r, w, "are you sure?")
	if !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("expected ErrNotInteractive, got %v", err)
	}
}
