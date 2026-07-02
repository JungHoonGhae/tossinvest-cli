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
