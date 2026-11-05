package selfupdate

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestRunDevBuildReturnsErrDevBuild(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: MethodDev,
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if !errors.Is(err, ErrDevBuild) {
		t.Fatalf("expected ErrDevBuild, got %v", err)
	}
}

func TestRunUnknownMethodReturnsError(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: InstallMethod(99),
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if err == nil {
		t.Fatal("expected an error for an unknown InstallMethod, got nil")
	}
}

func TestRunBinaryWithoutVersionRefuses(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: MethodBinary,
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if err == nil {
		t.Fatal("expected an error for MethodBinary without a pinned version, got nil")
	}
}

func TestPinnedTagNormalizes(t *testing.T) {
	for in, want := range map[string]string{"0.54.1": "v0.54.1", "v0.54.1": "v0.54.1"} {
		got, err := pinnedTag(in)
		if err != nil {
			t.Fatalf("pinnedTag(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Fatalf("pinnedTag(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := pinnedTag(""); err == nil {
		t.Fatal("pinnedTag(\"\") should fail closed, got nil error")
	}
}
