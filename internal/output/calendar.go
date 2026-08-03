package output

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteEconomicCalendar renders the upcoming-releases window.
//
// The table form groups by date rather than printing a date column: a calendar
// is read as "what happens on which day", and repeating 2026-08-05 on four
// consecutive rows makes that harder, not easier.
func WriteEconomicCalendar(w io.Writer, format Format, c domain.EconomicCalendar) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, c)

	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"date", "time", "title", "group"}); err != nil {
			return err
		}
		for _, e := range c.Events {
			if err := cw.Write([]string{e.Date, e.Time, e.Title, e.Group}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	}

	if len(c.Events) == 0 {
		_, err := fmt.Fprintln(w, "다가오는 경제 일정이 없습니다.")
		return err
	}

	if c.Summary != "" {
		if _, err := fmt.Fprintf(w, "%s\n\n", c.Summary); err != nil {
			return err
		}
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
		// A blank time means "sometime that day" (the feed's 23:59 sentinel),
		// so the column is padded rather than filled with a fake clock time.
		when := e.Time
		if when == "" {
			when = "종일"
		}
		if _, err := fmt.Fprintf(w, "  %-5s  %s\n", when, e.Title); err != nil {
			return err
		}
	}
	return nil
}
