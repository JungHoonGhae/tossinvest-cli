package output

import (
	"encoding/csv"
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
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"product_code", "name", "amount_usd"}); err != nil {
			return err
		}
		for _, s := range l.Stocks {
			if err := writer.Write([]string{s.ProductCode, s.Name, strconv.FormatFloat(s.AmountUSD, 'f', -1, 64)}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default: // table
		if _, err := fmt.Fprintf(w, "Expected lending income  ·  1M: $%.2f  ·  1Y: $%.2f\n", l.OneMonthUSD, l.OneYearUSD); err != nil {
			return err
		}
		if len(l.Stocks) == 0 {
			_, err := fmt.Fprintln(w, "(no lendable holdings)")
			return err
		}
		for _, s := range l.Stocks {
			if _, err := fmt.Fprintf(w, "  %-12s %-20s $%.4f\n", s.ProductCode, s.Name, s.AmountUSD); err != nil {
				return err
			}
		}
		return nil
	}
}
