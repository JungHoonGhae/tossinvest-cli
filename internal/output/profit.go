package output

import (
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteProfitOverview renders the cumulative profit breakdown. JSON is
// language-invariant; the table view uses plain labels.
func WriteProfitOverview(w io.Writer, format Format, p domain.ProfitOverview) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	default: // table (CSV would be awkward for this nested shape; table only)
		usd := func(d domain.DualCurrency) string {
			if d.USD == nil {
				return "-"
			}
			return fmt.Sprintf("$%.2f", *d.USD)
		}
		if _, err := fmt.Fprintf(w, "Total assets   %.0f KRW  ·  %s\n",
			p.TotalAssetAmount.KRW, usd(p.TotalAssetAmount)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Total earning  %.0f KRW  ·  %s\n\n",
			p.EarningAmount.KRW, usd(p.EarningAmount)); err != nil {
			return err
		}
		row := func(label string, t domain.ProfitByType) error {
			_, err := fmt.Fprintf(w, "  %-10s %14.0f KRW  %8s  (rate %.1f%%)\n",
				label, t.Amount.KRW, usd(t.Amount), t.EarningRate.KRW)
			return err
		}
		if err := row("매매손익", p.Sales); err != nil {
			return err
		}
		if err := row("배당", p.Dividend); err != nil {
			return err
		}
		if err := row("대여료", p.Lending); err != nil {
			return err
		}
		if err := row("만기", p.Maturity); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "  %-10s %14.0f KRW\n", "예탁금이자", p.Interest)
		return err
	}
}
