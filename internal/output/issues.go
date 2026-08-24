package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteMarketIssues renders the ranked issue board.
//
// full controls whether the backing articles are printed. The ranking is the
// point; 20 topics × 10 sources is a wall, so sources are opt-in.
func WriteMarketIssues(w io.Writer, format Format, m domain.MarketIssues, full bool) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, m)
	case FormatCSV:
		var csvRows [][]string
		for _, i := range m.Issues {
			csvRows = append(csvRows, []string{
				strconv.Itoa(i.Rank), i.RankStatus, i.Topic, i.Title,
				strconv.Itoa(i.SourceCount), i.Category,
			})
		}
		return writeCSV(w, []string{"rank", "rank_status", "topic", "title", "source_count", "category"}, csvRows)
	}

	if len(m.Issues) == 0 {
		_, err := fmt.Fprintln(w, "표시할 이슈가 없습니다.")
		return err
	}
	for _, i := range m.Issues {
		// The arrow carries the movement at a glance; the raw flag is kept for
		// anything the server adds beyond UP/DOWN.
		mark := map[string]string{"UP": "▲", "DOWN": "▼", "NEW": "＊", "SAME": "－"}[i.RankStatus]
		if mark == "" {
			mark = i.RankStatus
		}
		line := fmt.Sprintf("%2d %-2s %s", i.Rank, mark, i.Title)
		if i.Topic != "" && i.Topic != i.Title {
			line += "  — " + i.Topic
		}
		if i.SourceCount > 0 {
			line += fmt.Sprintf("  (%d건)", i.SourceCount)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if full {
			for _, s := range i.Sources {
				if _, err := fmt.Fprintf(w, "       · %s  [%s]\n", s.Title, s.Name); err != nil {
					return err
				}
			}
		}
	}
	if !full {
		_, err := fmt.Fprint(w, "\n(관련 기사를 보려면 --full)\n")
		return err
	}
	return nil
}
