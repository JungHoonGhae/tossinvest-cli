package output

import (
	"encoding/csv"
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

// WriteJSON is writeJSON for callers outside this package. `tossctl ops` is a
// machine surface fixed to JSON regardless of --output, so it renders through
// here rather than growing its own encoder with a different indent.
func WriteJSON(w io.Writer, v any) error { return writeJSON(w, v) }

// writeJSON renders v as the CLI's canonical JSON: two-space indent, one
// trailing newline (Encode adds it). Every FormatJSON branch goes through
// here, so the indent is decided once rather than per formatter.
// writeCSV writes a header row followed by data rows.
//
// Every CSV branch in this package was doing this by hand: construct a
// csv.Writer, write the header, loop the rows, Flush, then surface Error().
// Spread across ~40 copies, each one had to remember that Flush does not report
// failures — cw.Error() does — and a copy that forgets truncates output
// silently instead of failing.
//
// The table branch deliberately keeps its own rendering: most writers print
// prose around the table (empty-state lines, totals, warnings), and folding
// that in would widen this seam until it stopped paying for itself.
func writeCSV(w io.Writer, header []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
