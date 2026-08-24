package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
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
		if _, err := fmt.Fprintf(w, "Expected lending income  ·  1M: $%.2f  ·  1Y: $%.2f\n\n", l.OneMonthUSD, l.OneYearUSD); err != nil {
			return err
		}
		if len(l.Stocks) == 0 {
			_, err := fmt.Fprintln(w, "(no lendable holdings)")
			return err
		}
		headers := []string{"CODE", "NAME", "AMOUNT (USD)"}
		aligns := []Align{AlignLeft, AlignLeft, AlignRight}
		var rows [][]string
		for _, s := range l.Stocks {
			rows = append(rows, []string{s.ProductCode, s.Name, fmt.Sprintf("$%.4f", s.AmountUSD)})
		}
		return renderTable(w, headers, rows, aligns...)
	}
}
