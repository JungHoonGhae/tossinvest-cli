package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteMarketNews renders the news feed. Like WriteNewsBriefing, the "table"
// form is a readable list rather than columns — a 300-character summary and a
// variable stock list do not fit a grid.
//
// full controls whether summaries are printed. They are the richest part of the
// payload but 50 of them is a wall, so skimming is the default and reading is
// opt-in. Related stocks and their moves are always shown: they are what this
// feed has that a plain headline list does not.
func WriteMarketNews(w io.Writer, format Format, n domain.MarketNews, full bool) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, n)

	case FormatCSV:
		var csvRows [][]string
		for _, it := range n.Items {
			csvRows = append(csvRows, []string{
				it.CreatedAt, it.Source, it.Title, stocksCSV(it.Stocks), it.Summary,
			})
		}
		return writeCSV(w, []string{"created_at", "source", "title", "stocks", "summary"}, csvRows)

	default:
		if len(n.Items) == 0 {
			_, err := fmt.Fprintf(w, "%s: 표시할 뉴스가 없습니다.\n", headline(n))
			return err
		}
		if _, err := fmt.Fprintf(w, "[%s]  %d건\n", headline(n), len(n.Items)); err != nil {
			return err
		}
		for _, it := range n.Items {
			if _, err := fmt.Fprintf(w, "\n%s  %s\n  %s\n",
				it.CreatedAt, it.Source, it.Title); err != nil {
				return err
			}
			if s := stocksLine(it.Stocks); s != "" {
				if _, err := fmt.Fprintf(w, "  관련: %s\n", s); err != nil {
					return err
				}
			}
			if full && it.Summary != "" {
				if _, err := fmt.Fprintf(w, "  %s\n", it.Summary); err != nil {
					return err
				}
			}
		}
		if !full && hasSummary(n.Items) {
			_, err := fmt.Fprint(w, "\n(요약을 보려면 --full)\n")
			return err
		}
		return nil
	}
}

// headline prefers the server's own label for the scope, falling back to the
// raw enum when it is absent.
func headline(n domain.MarketNews) string {
	if n.Title != "" {
		return n.Title
	}
	return n.Type
}

func hasSummary(items []domain.NewsItem) bool {
	for _, it := range items {
		if it.Summary != "" {
			return true
		}
	}
	return false
}

// stocksLine renders related stocks with their moves: "NVDA +2.1%  TSM -0.4%".
func stocksLine(stocks []domain.NewsRelatedStock) string {
	if len(stocks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stocks))
	for _, s := range stocks {
		parts = append(parts, fmt.Sprintf("%s %+.2f%%", s.Name, s.Fluctuation))
	}
	return strings.Join(parts, "   ")
}

// stocksCSV keeps one cell machine-splittable: "name:code:pct|name:code:pct".
func stocksCSV(stocks []domain.NewsRelatedStock) string {
	parts := make([]string, 0, len(stocks))
	for _, s := range stocks {
		parts = append(parts, s.Name+":"+s.Code+":"+strconv.FormatFloat(s.Fluctuation, 'f', -1, 64))
	}
	return strings.Join(parts, "|")
}
