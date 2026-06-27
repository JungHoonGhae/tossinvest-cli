package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func WriteWatchlist(w io.Writer, format Format, items []domain.WatchlistItem) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(items)
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
		headers := []string{"그룹", "종목", "이름", "기준가", "현재가", "등락", "등락률", "통화"}
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
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(groups)
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
		headers := []string{"ID", "폴더", "종목수", "구분"}
		rows := make([][]string, 0, len(groups))
		for _, g := range groups {
			rows = append(rows, []string{fmt.Sprintf("%d", g.ID), g.Name, fmt.Sprintf("%d", g.ItemCount), g.Type})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
