package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))

	switch format {
	case FormatTable, FormatJSON, FormatCSV:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported output format %q; expected one of: table, json, csv", value)
	}
}

// writeJSON renders v as the CLI's canonical JSON: two-space indent, one
// trailing newline (Encode adds it). Every FormatJSON branch goes through
// here, so the indent is decided once rather than per formatter.
func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
