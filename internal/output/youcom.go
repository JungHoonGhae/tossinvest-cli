package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/youcom"
)

// WriteYouComResearch renders a You.com Research API result: the synthesized
// content followed by its numbered, cited sources.
func WriteYouComResearch(w io.Writer, format Format, res youcom.ResearchResult) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"type", "index", "title", "url", "text"}); err != nil {
			return err
		}
		if res.Output.Content != "" {
			if err := cw.Write([]string{"content", "", "", "", res.Output.Content}); err != nil {
				return err
			}
		}
		for i, s := range res.Output.Sources {
			if err := cw.Write([]string{
				"source", strconv.Itoa(i + 1), s.Title, s.URL, strings.Join(s.Snippets, " | "),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if res.Output.Content == "" && len(res.Output.Sources) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.research.empty"))
			return err
		}
		if res.Output.Content != "" {
			if _, err := fmt.Fprintln(w, strings.TrimRight(res.Output.Content, "\n")); err != nil {
				return err
			}
		}
		if len(res.Output.Sources) > 0 {
			if _, err := fmt.Fprint(w, "\n"+i18n.T("output.research.sourcesHeader")); err != nil {
				return err
			}
			for i, s := range res.Output.Sources {
				title := s.Title
				if title == "" {
					title = s.URL
				}
				if _, err := fmt.Fprintf(w, "  %d. %s — %s\n", i+1, title, s.URL); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
