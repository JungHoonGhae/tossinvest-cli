package output

import (
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteProfitOverview renders the cumulative profit breakdown. JSON is
// language-invariant; the table view uses localized labels and aligned columns.
func WriteProfitOverview(w io.Writer, format Format, p domain.ProfitOverview) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	default: // table (CSV would be awkward for this nested shape; table only)
		enabled := colorEnabled(w, format)
		if _, err := fmt.Fprintf(w, i18n.T("output.profit.totalAssets"),
			formatKRW(p.TotalAssetAmount.KRW), usdOrDash(p.TotalAssetAmount)); err != nil {
			return err
		}
		earningKRW := profitText(formatKRW(p.EarningAmount.KRW), p.EarningAmount.KRW, enabled)
		earningUSD := usdOrDash(p.EarningAmount)
		if p.EarningAmount.USD != nil {
			earningUSD = profitText(earningUSD, *p.EarningAmount.USD, enabled)
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.profit.totalEarning"),
			earningKRW, earningUSD); err != nil {
			return err
		}

		headers := []string{
			i18n.T("output.profit.header.category"),
			i18n.T("output.profit.header.amountKRW"),
			i18n.T("output.profit.header.amountUSD"),
			i18n.T("output.profit.header.rate"),
		}
		aligns := []Align{AlignLeft, AlignRight, AlignRight, AlignRight}

		type categoryItem struct {
			label string
			t     domain.ProfitByType
		}
		items := []categoryItem{
			{i18n.T("output.profit.sales"), p.Sales},
			{i18n.T("output.profit.dividend"), p.Dividend},
			{i18n.T("output.profit.lending"), p.Lending},
			{i18n.T("output.profit.maturity"), p.Maturity},
		}

		var plainRows, coloredRows [][]string
		for _, item := range items {
			krwStr := formatKRW(item.t.Amount.KRW)
			usdStr := usdOrDash(item.t.Amount)
			rateStr := fmt.Sprintf("%.1f%%", item.t.EarningRate.KRW)

			plain := []string{item.label, krwStr, usdStr, rateStr}
			colored := []string{
				item.label,
				profitText(krwStr, item.t.Amount.KRW, enabled),
				usdStr,
				profitText(rateStr, item.t.EarningRate.KRW, enabled),
			}
			if item.t.Amount.USD != nil {
				colored[2] = profitText(usdStr, *item.t.Amount.USD, enabled)
			}
			plainRows = append(plainRows, plain)
			coloredRows = append(coloredRows, colored)
		}

		// Interest row
		interestKRW := formatKRW(p.Interest)
		plainInterest := []string{i18n.T("output.profit.interest"), interestKRW, "-", "-"}
		coloredInterest := []string{
			i18n.T("output.profit.interest"),
			profitText(interestKRW, p.Interest, enabled),
			"-",
			"-",
		}
		plainRows = append(plainRows, plainInterest)
		coloredRows = append(coloredRows, coloredInterest)

		return renderTableColored(w, headers, plainRows, coloredRows, aligns...)
	}
}
