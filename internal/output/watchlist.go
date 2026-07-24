package output

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WriteWatchlist(w io.Writer, format Format, items []domain.WatchlistItem) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, items)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"group", "symbol", "name", "currency", "base", "last"}); err != nil {
			return err
		}
		for _, item := range items {
			if err := writer.Write([]string{
				item.Group,
				item.Symbol,
				item.Name,
				item.Currency,
				formatFloat(item.Base),
				formatFloat(item.Last),
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		enabled := colorEnabled(w, format)
		headers := []string{
			i18n.T("output.watchlist.header.group"),
			i18n.T("output.watchlist.header.symbol"),
			i18n.T("output.watchlist.header.name"),
			i18n.T("output.watchlist.header.base"),
			i18n.T("output.watchlist.header.current"),
			i18n.T("output.watchlist.header.change"),
			i18n.T("output.watchlist.header.changeRate"),
			i18n.T("output.watchlist.header.currency"),
		}
		var plainRows, coloredRows [][]string
		for _, item := range items {
			change := item.Last - item.Base
			var changeRate float64
			if item.Base != 0 {
				changeRate = change / item.Base
			}
			changeStr := formatKRW(change)
			if change > 0 {
				changeStr = "+" + changeStr
			}
			rateStr := formatPct(changeRate)
			plain := []string{
				item.Group,
				item.Symbol,
				item.Name,
				formatKRW(item.Base),
				formatKRW(item.Last),
				changeStr,
				rateStr,
				item.Currency,
			}
			colored := []string{
				item.Group,
				item.Symbol,
				item.Name,
				formatKRW(item.Base),
				formatKRW(item.Last),
				profitText(changeStr, change, enabled),
				profitText(rateStr, changeRate, enabled),
				item.Currency,
			}
			plainRows = append(plainRows, plain)
			coloredRows = append(coloredRows, colored)
		}
		return renderTableColored(w, headers, plainRows, coloredRows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteWatchlistGroups(w io.Writer, format Format, groups []domain.WatchlistGroup) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, groups)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"id", "name", "type", "item_count"}); err != nil {
			return err
		}
		for _, g := range groups {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", g.ID), g.Name, g.Type, fmt.Sprintf("%d", g.ItemCount),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.watchlist.groups.header.id"),
			i18n.T("output.watchlist.groups.header.folder"),
			i18n.T("output.watchlist.groups.header.count"),
			i18n.T("output.watchlist.groups.header.type"),
		}
		rows := make([][]string, 0, len(groups))
		for _, g := range groups {
			rows = append(rows, []string{fmt.Sprintf("%d", g.ID), g.Name, fmt.Sprintf("%d", g.ItemCount), g.Type})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
