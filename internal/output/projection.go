package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONOptions changes only serialization, never the request or stored data.
// Paths use JSON field names; arrays are traversed without an index.
type JSONOptions struct {
	Fields  []string
	Compact bool
}

func (o JSONOptions) Validate() error {
	for _, path := range o.Fields {
		for _, part := range strings.Split(path, ".") {
			if part == "" || strings.TrimSpace(part) != part || strings.ContainsAny(part, "[]* ,\t\n") {
				return fmt.Errorf("invalid field path %q; use comma-separated JSON names such as positions.symbol,positions.quantity", path)
			}
		}
	}
	return nil
}

type jsonWriter struct {
	io.Writer
	options JSONOptions
}

// WithJSONOptions attaches per-command settings without process-global state.
func WithJSONOptions(w io.Writer, options JSONOptions) io.Writer {
	return &jsonWriter{Writer: w, options: options}
}

// Project preserves envelopes, array order, nulls, and exact JSON numbers.
// Missing (including omitted optional) fields stay absent; empty arrays stay [].
func Project(value any, fields []string) (any, error) {
	if len(fields) == 0 {
		return value, nil
	}
	if err := (JSONOptions{Fields: fields}).Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return projectValue(decoded, fields), nil
}

func projectValue(value any, fields []string) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, row := range v {
			out[i] = projectValue(row, fields)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		children := map[string][]string{}
		whole := map[string]bool{}
		for _, path := range fields {
			key, rest, nested := strings.Cut(path, ".")
			if !nested {
				whole[key] = true
			} else {
				children[key] = append(children[key], rest)
			}
		}
		for key, child := range v {
			if whole[key] {
				out[key] = child
			} else if paths := children[key]; len(paths) > 0 {
				out[key] = projectValue(child, paths)
			}
		}
		return out
	default:
		return value
	}
}
