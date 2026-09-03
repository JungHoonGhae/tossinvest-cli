package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteLendingExpected renders projected share-lending income. JSON/CSV are
// language-invariant; the table view uses plain labels.
func WriteLendingExpected(w io.Writer, format Format, l domain.LendingExpected) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, l)
	case FormatCSV:
		var csvRows [][]string
		for _, s := range l.Stocks {
			csvRows = append(csvRows, []string{s.ProductCode, s.Name, strconv.FormatFloat(s.AmountUSD, 'f', -1, 64)})
		}
		return writeCSV(w, []string{"product_code", "name", "amount_usd"}, csvRows)
	default: // table
		if _, err := fmt.Fprintf(w, "Expected lending income  ·  1M: $%.2f  ·  1Y: $%.2f\n", l.OneMonthUSD, l.OneYearUSD); err != nil {
			return err
		}
		if len(l.Stocks) == 0 {
			_, err := fmt.Fprintln(w, "(no lendable holdings)")
			return err
		}
		headers := []string{
			i18n.T("output.lending.header.code"),
			i18n.T("output.lending.header.name"),
			i18n.T("output.lending.header.amount"),
		}
		aligns := []Align{AlignLeft, AlignLeft, AlignRight}
		var rows [][]string
		for _, s := range l.Stocks {
			rows = append(rows, []string{s.ProductCode, s.Name, fmt.Sprintf("$%.4f", s.AmountUSD)})
		}
		return renderTable(w, headers, rows, aligns...)
	}
}

// WriteLendingRevenueRanking renders the anonymized share-lending leaderboard.
func WriteLendingRevenueRanking(w io.Writer, format Format, ranking domain.LendingRevenueRanking) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, ranking)
	case FormatCSV:
		rows := make([][]string, 0, len(ranking.Items))
		for _, item := range ranking.Items {
			rows = append(rows, []string{
				strconv.Itoa(item.Rank), item.UserName,
				formatFloat(item.Revenue), formatFloat(item.RevenueKRW),
			})
		}
		return writeCSV(w, []string{"rank", "user_name", "revenue", "revenue_krw"}, rows)
	case FormatTable:
		if len(ranking.Items) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.lendingTop.empty"))
			return err
		}
		rows := make([][]string, 0, len(ranking.Items))
		for _, item := range ranking.Items {
			rows = append(rows, []string{
				strconv.Itoa(item.Rank), item.UserName,
				formatFloat(item.Revenue), formatFloat(item.RevenueKRW),
			})
		}
		return renderTable(w, []string{
			i18n.T("output.lendingTop.header.rank"), i18n.T("output.lendingTop.header.user"),
			i18n.T("output.lendingTop.header.revenue"), i18n.T("output.lendingTop.header.revenueKRW"),
		}, rows, AlignRight, AlignLeft, AlignRight, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
