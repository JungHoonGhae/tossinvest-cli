package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func sectorDetailPrice(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}

func sectorDetailTotal(total, returned int) int {
	if total == 0 && returned > 0 {
		return returned
	}
	return total
}

// WriteSectorDetail renders the aggregate without discarding any structured
// data in JSON. CSV and table use one flat schema across stocks, ETFs, and news
// so automation does not have to parse multiple independently formatted blocks.
func WriteSectorDetail(w io.Writer, format Format, detail domain.SectorDetail) error {
	if format == FormatJSON {
		return writeJSON(w, detail)
	}
	stockTotal := sectorDetailTotal(detail.StockTotalCount, len(detail.Stocks))
	etfTotal := sectorDetailTotal(detail.ETFTotalCount, len(detail.ETFs))
	newsTotal := sectorDetailTotal(detail.NewsTotalCount, len(detail.News))
	rows := make([][]string, 0, len(detail.Stocks)+len(detail.ETFs)+len(detail.News))
	for _, item := range detail.Stocks {
		rows = append(rows, []string{
			"stock", strconv.Itoa(item.Rank), item.ProductCode, item.Name,
			formatFloat(item.ChangeRate), sectorDetailPrice(item.Price.Close), "", "", strconv.Itoa(stockTotal),
		})
	}
	for _, item := range detail.ETFs {
		code := item.Symbol
		if code == "" {
			code = item.ProductCode
		}
		rows = append(rows, []string{
			"etf", strconv.Itoa(item.Rank), code, item.Name,
			formatFloat(item.ChangeRate), sectorDetailPrice(item.Price.Close), "", "", strconv.Itoa(etfTotal),
		})
	}
	for _, item := range detail.News {
		rows = append(rows, []string{"news", "", item.ID, item.Title, "", "", item.Source, item.CreatedAt, strconv.Itoa(newsTotal)})
	}

	switch format {
	case FormatCSV:
		return writeCSV(w, []string{"type", "rank", "code", "name", "change_rate", "close", "source", "created_at", "total_count"}, rows)
	case FormatTable:
		if len(rows) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.sectorDetail.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.sectorDetail.summary"),
			detail.Name,
			len(detail.Stocks), sectorDetailTotal(detail.StockTotalCount, len(detail.Stocks)),
			len(detail.ETFs), sectorDetailTotal(detail.ETFTotalCount, len(detail.ETFs)),
			len(detail.News), sectorDetailTotal(detail.NewsTotalCount, len(detail.News))); err != nil {
			return err
		}
		if detail.Summary != "" {
			if _, err := fmt.Fprintln(w, detail.Summary); err != nil {
				return err
			}
		}
		if detail.Description != "" && detail.Description != detail.Summary {
			if _, err := fmt.Fprintln(w, detail.Description); err != nil {
				return err
			}
		}
		headers := []string{
			i18n.T("output.sectorDetail.header.type"),
			i18n.T("output.sectorDetail.header.rank"),
			i18n.T("output.sectorDetail.header.code"),
			i18n.T("output.sectorDetail.header.name"),
			i18n.T("output.sectorDetail.header.changeRate"),
			i18n.T("output.sectorDetail.header.close"),
			i18n.T("output.sectorDetail.header.source"),
			i18n.T("output.sectorDetail.header.createdAt"),
			i18n.T("output.sectorDetail.header.totalCount"),
		}
		return renderTable(w, headers, rows, AlignLeft, AlignRight, AlignLeft, AlignLeft, AlignRight, AlignRight, AlignLeft, AlignLeft, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
