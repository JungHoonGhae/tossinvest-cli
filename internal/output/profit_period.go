package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// periodLabel describes the window a period query covered, for the table
// header. Dates arrive in the API's YYYYMMDD; humans read YYYY-MM-DD.
func periodLabel(from, to string) string {
	if from == "" && to == "" {
		return i18n.T("output.profitPeriod.allTime")
	}
	return fmt.Sprintf("%s ~ %s", dashDate(from), dashDate(to))
}

func dashDate(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:]
}

// WritePeriodProfit renders realized profit for one type over a date range.
// CSV is not offered: the payload is three scalars, so a table reads better and
// JSON already covers machine use.
func WritePeriodProfit(w io.Writer, format Format, p domain.PeriodProfit) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	default:
		enabled := colorEnabled(w, format)
		if _, err := fmt.Fprintf(w, "%s  (%s)\n\n", p.Type, periodLabel(p.From, p.To)); err != nil {
			return err
		}

		headers := []string{
			i18n.T("output.profitPeriod.header.item"),
			i18n.T("output.profitPeriod.header.amountKRW"),
			i18n.T("output.profitPeriod.header.amountUSD"),
		}
		aligns := []Align{AlignLeft, AlignRight, AlignRight}

		earningKRW := formatKRW(p.EarningAmount.KRW)
		earningUSD := usdOrDash(p.EarningAmount)
		rateStr := fmt.Sprintf("%.2f%%", p.EarningRate.KRW)
		purchaseKRW := formatKRW(p.PurchaseAmount.KRW)
		purchaseUSD := usdOrDash(p.PurchaseAmount)

		coloredEarningUSD := earningUSD
		if p.EarningAmount.USD != nil {
			coloredEarningUSD = profitText(earningUSD, *p.EarningAmount.USD, enabled)
		}

		coloredRows := [][]string{
			{i18n.T("output.profitPeriod.earningAmount"), profitText(earningKRW, p.EarningAmount.KRW, enabled), coloredEarningUSD},
			{i18n.T("output.profitPeriod.earningRate"), profitText(rateStr, p.EarningRate.KRW, enabled), "-"},
			{i18n.T("output.profitPeriod.purchaseAmount"), purchaseKRW, purchaseUSD},
		}

		return renderTable(w, headers, coloredRows, aligns...)
	}
}

// WriteDailyProfit renders the per-stock realized-profit breakdown. This one
// does offer CSV — it is a flat row set, which is exactly what people paste
// into a spreadsheet for tax season.
func WriteDailyProfit(w io.Writer, format Format, p domain.DailyProfit) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	case FormatCSV:
		var csvRows [][]string
		for _, s := range p.Stocks {
			csvRows = append(csvRows, []string{
				s.Date, s.MarketType, s.Symbol, s.Name,
				strconv.FormatFloat(s.Quantity, 'f', -1, 64),
				strconv.FormatFloat(s.ProfitLoss.KRW, 'f', -1, 64),
				strconv.FormatFloat(s.ProfitRate, 'f', -1, 64),
				strconv.FormatFloat(s.SellAmount.KRW, 'f', -1, 64),
				strconv.FormatFloat(s.BuyAmount.KRW, 'f', -1, 64),
			})
		}
		return writeCSV(w, []string{
			"date", "market", "symbol", "name",
			"quantity", "profit_loss_krw", "profit_rate", "sell_krw", "buy_krw",
		}, csvRows)
	default:
		if len(p.Stocks) == 0 {
			_, err := fmt.Fprintf(w, i18n.T("output.profitDaily.empty"),
				periodLabel(p.From, p.To))
			return err
		}
		enabled := colorEnabled(w, format)
		// The currency is a property of the whole query (the rate basis), so it
		// belongs in the header rather than repeated down a column.
		if _, err := fmt.Fprintf(w, i18n.T("output.profitDaily.title"),
			periodLabel(p.From, p.To), len(p.Stocks), p.Currency); err != nil {
			return err
		}

		headers := []string{
			i18n.T("output.profitDaily.header.date"),
			i18n.T("output.profitDaily.header.symbol"),
			i18n.T("output.profitDaily.header.name"),
			i18n.T("output.profitDaily.header.quantity"),
			i18n.T("output.profitDaily.header.pnl"),
			i18n.T("output.profitDaily.header.rate"),
		}
		aligns := []Align{AlignLeft, AlignLeft, AlignLeft, AlignRight, AlignRight, AlignRight}

		var coloredRows [][]string
		var total float64
		for _, s := range p.Stocks {
			total += s.ProfitLoss.KRW
			pnlStr := formatKRW(s.ProfitLoss.KRW)
			rateStr := fmt.Sprintf("%.2f%%", s.ProfitRate)
			name := truncateName(s.Name, 16)

			colored := []string{
				s.Date,
				s.Symbol,
				name,
				formatQty(s.Quantity),
				profitText(pnlStr, s.ProfitLoss.KRW, enabled),
				profitText(rateStr, s.ProfitRate, enabled),
			}
			coloredRows = append(coloredRows, colored)
		}

		if err := renderTable(w, headers, coloredRows, aligns...); err != nil {
			return err
		}

		totalStr := formatKRW(total)
		coloredTotal := profitText(totalStr, total, enabled)
		_, err := fmt.Fprintf(w, "\n%s: %s KRW\n", i18n.T("output.profitDaily.total"), coloredTotal)
		return err
	}
}
