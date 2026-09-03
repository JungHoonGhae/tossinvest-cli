package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteAISignalDetail renders the full structured response as JSON and a
// compact, loss-aware projection for human/CSV output.
func WriteAISignalDetail(w io.Writer, format Format, detail domain.AISignalDetail) error {
	if format == FormatJSON {
		return writeJSON(w, detail)
	}
	if !detail.Found {
		if format == FormatCSV {
			return writeCSV(w,
				[]string{"type", "asset_code", "asset_name", "signal_id", "title", "description", "source", "created_at"},
				nil,
			)
		}
		_, err := fmt.Fprintf(w, i18n.T("output.aiSignalDetail.empty"), detail.ProductCode, detail.ProductType)
		return err
	}

	rows := [][]string{{
		"issue", detail.Issue.AssetCode, detail.Issue.AssetName, detail.SignalID,
		strings.Join(detail.Keywords, ", "), strings.Join(detail.Issue.Description, " "), "", detail.CreatedAt,
	}}
	for _, item := range detail.News {
		rows = append(rows, []string{"news", detail.Issue.AssetCode, detail.Issue.AssetName, item.ID, item.Title, "", item.Agency, item.CreatedAt})
	}
	for _, item := range detail.Related {
		rows = append(rows, []string{
			"related", item.AssetCode, item.AssetName, item.SignalID,
			item.Relationship.Relation, strings.Join(item.Description, " "), "", "",
		})
	}

	switch format {
	case FormatCSV:
		return writeCSV(w,
			[]string{"type", "asset_code", "asset_name", "signal_id", "title", "description", "source", "created_at"},
			rows,
		)
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.aiSignalDetail.summary"),
			detail.Issue.AssetName, detail.ProductCode, detail.ProductType, detail.SignalDirection); err != nil {
			return err
		}
		if detail.Description != "" {
			if _, err := fmt.Fprintln(w, detail.Description); err != nil {
				return err
			}
		}
		for _, line := range detail.Issue.Description {
			if _, err := fmt.Fprintf(w, "  · %s\n", line); err != nil {
				return err
			}
		}
		if len(detail.News) > 0 {
			if _, err := fmt.Fprintln(w, i18n.T("output.aiSignalDetail.news")); err != nil {
				return err
			}
			for _, item := range detail.News {
				agency := item.Agency
				if agency != "" {
					agency = " (" + agency + ")"
				}
				if _, err := fmt.Fprintf(w, "  · %s%s\n", item.Title, agency); err != nil {
					return err
				}
			}
		}
		if len(detail.Related) > 0 {
			heading := i18n.T("output.aiSignalDetail.related")
			if detail.RelatedCallout != "" {
				heading += " · " + detail.RelatedCallout
			}
			if _, err := fmt.Fprintln(w, heading); err != nil {
				return err
			}
			for _, item := range detail.Related {
				relation := item.Relationship.Relation
				if relation != "" {
					relation = " · " + relation
				}
				if _, err := fmt.Fprintf(w, "  · %s (%s)%s", item.AssetName, item.AssetCode, relation); err != nil {
					return err
				}
				if len(item.Description) > 0 {
					if _, err := fmt.Fprintf(w, " — %s", strings.Join(item.Description, " ")); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
