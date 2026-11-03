package official

import (
	"fmt"
	"io"
)

// maxJSONBodyBytes caps JSON response bodies read by the official client.
// It guards against unbounded memory growth from a misbehaving server or a
// transport that never ends the stream.
const maxJSONBodyBytes = 8 << 20 // 8 MiB

// readBodyCapped reads at most max+1 bytes from r, returning an explicit
// ErrTransport error when the body exceeds max bytes.
func readBodyCapped(r io.Reader, max int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTransport, err)
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("%w: response body exceeds %d bytes", ErrTransport, max)
	}
	return body, nil
}
