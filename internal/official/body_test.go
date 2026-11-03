package official

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Wrapping a cancellation with the multi-%w form must traverse both the
// sentinel and the cause.
func TestTransportWrapTraversesSentinelAndCause(t *testing.T) {
	wrapped := fmt.Errorf("%w: %w", ErrTransport, context.Canceled)
	if !errors.Is(wrapped, ErrTransport) {
		t.Fatal("errors.Is(wrapped, ErrTransport) = false")
	}
	if !errors.Is(wrapped, context.Canceled) {
		t.Fatal("errors.Is(wrapped, context.Canceled) = false")
	}
}

type cancelTripper struct{}

func (cancelTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, context.Canceled
}

// doRequest must preserve the cancellation cause through its %w wrap.
func TestDoRequestPreservesCancellation(t *testing.T) {
	c := &Client{hc: &http.Client{Transport: cancelTripper{}}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.doRequest(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("errors.Is(err, ErrTransport) = false: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(err, context.Canceled) = false: %v", err)
	}
}

func TestReadBodyCapped(t *testing.T) {
	small := []byte(`{"result":{}}`)
	got, err := readBodyCapped(bytes.NewReader(small), maxJSONBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, small) {
		t.Fatal("body mismatch")
	}

	big := strings.NewReader(strings.Repeat("x", int(maxJSONBodyBytes)+1))
	if _, err := readBodyCapped(big, maxJSONBodyBytes); err == nil {
		t.Fatal("expected cap error")
	} else if !errors.Is(err, ErrTransport) {
		t.Fatalf("cap error must wrap ErrTransport: %v", err)
	}

	// A read failure must still wrap ErrTransport.
	if _, err := readBodyCapped(errReader{}, maxJSONBodyBytes); err == nil {
		t.Fatal("expected read error")
	} else if !errors.Is(err, ErrTransport) {
		t.Fatalf("read error must wrap ErrTransport: %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
