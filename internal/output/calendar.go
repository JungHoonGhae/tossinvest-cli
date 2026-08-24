package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteMarketCalendar renders one month of scheduled market events.
//
// The table form groups by date rather than printing a date column: a calendar
// is read as "what happens on which day", and repeating 2026-08-05 across four
// consecutive rows makes that harder, not easier.
func WriteMarketCalendar(w io.Writer, format Format, c domain.MarketCalendar) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, c)

	case FormatCSV:
		var csvRows [][]string
		for _, e := range c.Events {
			var forecast, actual, historical, unit string
			if e.Indicator != nil {
				forecast, actual, historical = num(e.Indicator.Forecast), num(e.Indicator.Actual), num(e.Indicator.Historical)
				unit = e.Indicator.Unit
			}
			csvRows = append(csvRows, []string{
				e.Date, e.Kind, e.Title, e.Symbol, e.Note, forecast, actual, historical, unit,
			})
		}
		return writeCSV(w, []string{
			"date", "kind", "title", "symbol", "note", "forecast", "actual", "historical", "unit",
		}, csvRows)
	}

	if c.Summary != "" {
		if _, err := fmt.Fprintf(w, "%s\n", c.Summary); err != nil {
			return err
		}
		if c.SummaryDetail != "" {
			if _, err := fmt.Fprintf(w, "%s\n", c.SummaryDetail); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(c.Events) == 0 {
		if _, err := fmt.Fprintf(w, "%s 에 예정된 일정이 없습니다.\n", c.Month); err != nil {
			return err
		}
		return writeCalendarWarnings(w, c)
	}

	lastDate := ""
	for _, e := range c.Events {
		if e.Date != lastDate {
			if lastDate != "" {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "%s\n", e.Date); err != nil {
				return err
			}
			lastDate = e.Date
		}
		line := fmt.Sprintf("  %-11s %s", kindLabel(e.Kind), e.Title)
		if e.Symbol != "" {
			line += "  (" + e.Symbol + ")"
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		// The forecast is the part a bare date lacks: knowing a print is due
		// matters less than knowing what the street expects against last time.
		if e.Indicator != nil {
			if detail := indicatorLine(e.Indicator); detail != "" {
				if _, err := fmt.Fprintf(w, "  %-11s %s\n", "", detail); err != nil {
					return err
				}
			}
		}
		if e.Note != "" {
			if _, err := fmt.Fprintf(w, "  %-11s %s\n", "", e.Note); err != nil {
				return err
			}
		}
	}
	return writeCalendarWarnings(w, c)
}

func writeCalendarWarnings(w io.Writer, c domain.MarketCalendar) error {
	for _, warn := range c.Warnings {
		if _, err := fmt.Fprintf(w, "\n⚠ %s\n", warn); err != nil {
			return err
		}
	}
	return nil
}

func kindLabel(kind string) string {
	switch kind {
	case "economic":
		return "지표"
	case "earnings_kr":
		return "실적(국내)"
	case "earnings_us":
		return "실적(미국)"
	case "holiday":
		return "휴장"
	default:
		return kind
	}
}

func indicatorLine(i *domain.CalendarIndicator) string {
	parts := ""
	add := func(label string, v *float64) {
		if v == nil {
			return
		}
		if parts != "" {
			parts += " · "
		}
		parts += label + " " + num(v)
	}
	add("예상", i.Forecast)
	add("실제", i.Actual)
	add("직전", i.Historical)
	if parts != "" && i.Unit != "" {
		parts += " (" + i.Unit + ")"
	}
	return parts
}

func num(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
