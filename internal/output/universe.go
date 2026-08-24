package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteStockUniverse renders a market's tradable universe.
//
// The table prints every row: this is a list people pipe into other commands,
// and silently truncating it would make the count lie. Callers who want a
// slice use --output csv/json with head.
func WriteStockUniverse(w io.Writer, format Format, u domain.StockUniverse) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, u)
	case FormatCSV:
		var csvRows [][]string
		for _, s := range u.Stocks {
			csvRows = append(csvRows, []string{s.Symbol, s.Name, s.ISINCode, s.SecurityType, strconv.FormatBool(s.CommonShare)})
		}
		return writeCSV(w, []string{"symbol", "name", "isin_code", "security_type", "common_share"}, csvRows)
	case FormatTable:
		if len(u.Stocks) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.universe.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.universe.header"), u.Market, len(u.Stocks)); err != nil {
			return err
		}
		for _, s := range u.Stocks {
			if _, err := fmt.Fprintf(w, i18n.T("output.universe.line"), s.Symbol, s.Name, s.SecurityType); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
