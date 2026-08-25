package output

import (
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteScreenerFilterRanges renders each filter's usable value span.
func WriteScreenerFilterRanges(w io.Writer, format Format, r domain.ScreenerFilterRanges) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, r)
	case FormatCSV:
		var csvRows [][]string
		for _, f := range r.Filters {
			min, max := "", ""
			if f.Min != nil {
				min = formatFloat(*f.Min)
			}
			if f.Max != nil {
				max = formatFloat(*f.Max)
			}
			csvRows = append(csvRows, []string{f.FilterID, f.Nation, min, max, f.BasedAt, f.Unavailable})
		}
		return writeCSV(w, []string{"filter_id", "nation", "min", "max", "based_at", "unavailable_reason"}, csvRows)
	case FormatTable:
		if len(r.Filters) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.screenerFilters.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.screenerFilters.header"), r.Nation); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.screenerFilters.header.filter"),
			i18n.T("output.screenerFilters.header.min"),
			i18n.T("output.screenerFilters.header.max"),
			i18n.T("output.screenerFilters.header.basedAt"),
		}
		var rows [][]string
		for _, f := range r.Filters {
			if f.Min == nil || f.Max == nil {
				rows = append(rows, []string{f.FilterID + " (" + f.Unavailable + ")", "-", "-", "-"})
				continue
			}
			basedAt := f.BasedAt
			if basedAt == "" {
				basedAt = "-"
			}
			rows = append(rows, []string{f.FilterID, compactNum(*f.Min), compactNum(*f.Max), basedAt})
		}
		aligns := []Align{AlignLeft, AlignRight, AlignLeft, AlignLeft}
		return renderTable(w, headers, rows, aligns...)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// compactNum trims a float to something a person can compare at a glance.
// %g with 6 significant digits keeps both 0.382436 and 1.50793e+15 readable,
// where the raw float32 spill (-8021.2705078125) is not.
func compactNum(v float64) string {
	return fmt.Sprintf("%.6g", v)
}
