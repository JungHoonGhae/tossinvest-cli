package client

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadBodyCappedUnderCap(t *testing.T) {
	t.Parallel()

	data, err := readBodyCapped(bytes.NewReader([]byte(`{"ok":true}`)), maxJSONBodyBytes)
	if err != nil {
		t.Fatalf("readBodyCapped: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data = %q", data)
	}
}

func TestReadBodyCappedOverCap(t *testing.T) {
	t.Parallel()

	big := bytes.NewReader(make([]byte, maxJSONBodyBytes+16))
	if _, err := readBodyCapped(big, maxJSONBodyBytes); err == nil {
		t.Fatal("expected over-cap error, got nil")
	} else if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %q, want exceeds message", err)
	}
}

func TestReadBodyCappedBundleBoundary(t *testing.T) {
	t.Parallel()

	atCap := bytes.NewReader(make([]byte, maxBundleBytes))
	if _, err := readBodyCapped(atCap, maxBundleBytes); err != nil {
		t.Fatalf("at-cap read: %v", err)
	}
	overCap := bytes.NewReader(make([]byte, maxBundleBytes+1))
	if _, err := readBodyCapped(overCap, maxBundleBytes); err == nil {
		t.Fatal("expected over-cap bundle error, got nil")
	}
}
