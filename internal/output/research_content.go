package output

import (
	"fmt"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"io"
	"strconv"
	"strings"
)

func writeResearchRows(w io.Writer, format Format, header []string, rows [][]string) error {
	switch format {
	case FormatCSV:
		return writeCSV(w, header, rows)
	case FormatTable:
		aligns := make([]Align, len(header))
		return renderTable(w, header, rows, aligns...)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
func WriteEarningTranscript(w io.Writer, format Format, data domain.EarningTranscript) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	if format == FormatTable {
		if !data.Available {
			_, err := fmt.Fprintln(w, i18n.T("output.transcript.empty"))
			return err
		}
		for _, p := range data.Paragraphs {
			if _, err := fmt.Fprintf(w, "[%s–%ss] %s\n%s\n", formatFloat(p.StartSeconds), formatFloat(p.EndSeconds), p.ID, p.OriginalText); err != nil {
				return err
			}
			if p.Translation != nil {
				for _, line := range p.Translation.Lines {
					if _, err := fmt.Fprintf(w, "  %s: %s\n", line.Speaker, line.Text); err != nil {
						return err
					}
				}
			} else if _, err := fmt.Fprintln(w, "  "+i18n.T("output.transcript.translationEmpty")); err != nil {
				return err
			}
			if p.Summary != nil {
				if _, err := fmt.Fprintf(w, "  "+i18n.T("output.transcript.summary")+"\n", p.Summary.Text); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintln(w, "  "+i18n.T("output.transcript.summaryEmpty")); err != nil {
				return err
			}
		}
		return nil
	}
	rows := [][]string{}
	for _, p := range data.Paragraphs {
		translated := []string{}
		summary := ""
		if p.Translation != nil {
			for _, line := range p.Translation.Lines {
				translated = append(translated, line.Speaker+": "+line.Text)
			}
		}
		if p.Summary != nil {
			summary = p.Summary.Text
		}
		rows = append(rows, []string{strconv.FormatInt(data.EventID, 10), p.ID, formatFloat(p.StartSeconds), formatFloat(p.EndSeconds), p.OriginalText, strings.Join(translated, "\n"), summary})
	}
	return writeResearchRows(w, format, []string{"event_id", "paragraph_id", "start_seconds", "end_seconds", "original", "translation", "summary"}, rows)
}
func WriteEarningReport(w io.Writer, format Format, data domain.EarningReport) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	rows := [][]string{}
	add := func(kind string, item domain.EarningAnalysisItem) {
		rows = append(rows, []string{kind, strconv.FormatInt(item.ID, 10), item.Title, strings.Join(item.Content, "\n"), ""})
	}
	if data.Analysis == nil {
		rows = append(rows, []string{"analysis", "", i18n.T("output.research.unavailable"), "", ""})
	} else {
		add("overall", data.Analysis.Overall)
		for _, p := range data.Analysis.Pros {
			add("pro", p)
		}
		for _, c := range data.Analysis.Cons {
			add("con", c)
		}
	}
	if data.WatchPoints == nil {
		rows = append(rows, []string{"watch_points", "", i18n.T("output.research.unavailable"), "", ""})
	} else {
		for _, p := range data.WatchPoints.Points {
			rows = append(rows, []string{"watch_point", "", p.Title, strings.Join(p.Bullets, "\n"), ""})
		}
		for _, s := range data.WatchPoints.Sources {
			rows = append(rows, []string{"source", s.ID, s.Title, s.Provider, s.CreatedAt})
		}
	}
	return writeResearchRows(w, format, []string{"type", "id", "title", "content", "created_at"}, rows)
}
func WriteWatchlistNews(w io.Writer, format Format, data domain.WatchlistNews) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	rows := [][]string{}
	for _, n := range data.News {
		stocks := []string{}
		for _, s := range n.RelatedStocks {
			stocks = append(stocks, s.Name+" ("+formatFloat(s.Fluctuation)+")")
		}
		rows = append(rows, []string{n.ID, n.Title, n.Agency, n.CreatedAt, strings.Join(stocks, ", ")})
	}
	if format == FormatTable && len(rows) == 0 {
		_, err := fmt.Fprintln(w, i18n.T("output.watchlistNews.empty"))
		return err
	}
	return writeResearchRows(w, format, []string{"id", "title", "agency", "created_at", "related_stocks"}, rows)
}
