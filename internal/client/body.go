package client

import (
	"fmt"
	"io"
)

const maxJSONBodyBytes = 8 << 20

const maxBundleBytes = 64 << 20

func readBodyCapped(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("response body exceeds %d bytes", max)
	}
	return data, nil
}
