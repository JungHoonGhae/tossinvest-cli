package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func usdOrDash(d domain.DualCurrency) string {
	if d.USD == nil {
		return "-"
	}
	return fmt.Sprintf("$%.2f", *d.USD)
}

// periodLabel describes the window a period query covered, for the table
// header. Dates arrive in the API's YYYYMMDD; humans read YYYY-MM-DD.
func periodLabel(from, to string) string {
	if from == "" && to == "" {
		return "전체 기간"
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
		if _, err := fmt.Fprintf(w, "%s  (%s)\n\n", p.Type, periodLabel(p.From, p.To)); err != nil {
			return err
		}
		rows := []struct {
			label string
			v     domain.DualCurrency
			pct   bool
		}{
			{"수익금", p.EarningAmount, false},
			{"수익률", p.EarningRate, true},
			{"매입금액", p.PurchaseAmount, false},
		}
		for _, r := range rows {
			if r.pct {
				if _, err := fmt.Fprintf(w, "  %-8s %14.2f %%\n", r.label, r.v.KRW); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, "  %-8s %14.0f KRW  %10s\n",
				r.label, r.v.KRW, usdOrDash(r.v)); err != nil {
				return err
			}
		}
		return nil
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
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{
			"date", "market", "symbol", "name",
			"quantity", "profit_loss_krw", "profit_rate", "sell_krw", "buy_krw",
		}); err != nil {
			return err
		}
		for _, s := range p.Stocks {
			if err := cw.Write([]string{
				s.Date, s.MarketType, s.Symbol, s.Name,
				strconv.FormatFloat(s.Quantity, 'f', -1, 64),
				strconv.FormatFloat(s.ProfitLoss.KRW, 'f', -1, 64),
				strconv.FormatFloat(s.ProfitRate, 'f', -1, 64),
				strconv.FormatFloat(s.SellAmount.KRW, 'f', -1, 64),
				strconv.FormatFloat(s.BuyAmount.KRW, 'f', -1, 64),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	default:
		if len(p.Stocks) == 0 {
			_, err := fmt.Fprintf(w, "해당 기간(%s)에 실현손익 내역이 없습니다.\n",
				periodLabel(p.From, p.To))
			return err
		}
		// The currency is a property of the whole query (the rate basis), so it
		// belongs in the header rather than repeated down a column.
		if _, err := fmt.Fprintf(w, "종목별 실현손익  (%s)  %d건  ·  수익률 기준 %s\n\n",
			periodLabel(p.From, p.To), len(p.Stocks), p.Currency); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%-12s %-10s %-20s %10s %14s %9s\n",
			"DATE", "SYMBOL", "NAME", "QTY", "P/L (KRW)", "RATE"); err != nil {
			return err
		}
		var total float64
		for _, s := range p.Stocks {
			total += s.ProfitLoss.KRW
			if _, err := fmt.Fprintf(w, "%-12s %-10s %-20s %10.4g %14.0f %8.2f%%\n",
				s.Date, s.Symbol, truncate(s.Name, 20),
				s.Quantity, s.ProfitLoss.KRW, s.ProfitRate); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "\n%-44s %14.0f KRW\n", "합계", total)
		return err
	}
}

// truncate keeps wide product names from breaking the column layout.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
