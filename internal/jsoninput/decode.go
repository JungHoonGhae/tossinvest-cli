// Package jsoninput decodes JSON received from user-controlled command and
// protocol boundaries without eagerly rounding numbers through float64.
package jsoninput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Decode has json.Unmarshal semantics plus json.Number preservation for values
// decoded through interface{} and rejection of trailing JSON values.
func Decode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
