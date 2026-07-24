package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WriteQuote(w io.Writer, format Format, quote domain.Quote) error {
	switch format {
	case FormatTable:
		enabled := colorEnabled(w, format)
		return writeQuoteTable(w, quote, enabled)
	case FormatJSON:
		return writeQuoteJSON(w, quote)
	case FormatCSV:
		return writeQuoteCSV(w, quote)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeQuoteJSON(w io.Writer, quote domain.Quote) error {
	return writeJSON(w, quote)
}

func writeQuoteCSV(w io.Writer, quote domain.Quote) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"product_code",
		"symbol",
		"name",
		"market_code",
		"market",
		"currency",
		"reference_price",
		"last",
		"change",
		"change_rate",
		"volume",
		"status",
		"badge_count",
		"notice_count",
		"fetched_at",
	}); err != nil {
		return err
	}

	if err := writer.Write([]string{
		quote.ProductCode,
		quote.Symbol,
		quote.Name,
		quote.MarketCode,
		quote.Market,
		quote.Currency,
		formatFloat(quote.ReferencePrice),
		formatFloat(quote.Last),
		formatFloat(quote.Change),
		formatFloat(quote.ChangeRate),
		formatFloat(quote.Volume),
		quote.Status,
		strconv.Itoa(quote.BadgeCount),
		strconv.Itoa(quote.NoticeCount),
		quote.FetchedAt.Format("2006-01-02T15:04:05Z07:00"),
	}); err != nil {
		return err
	}

	writer.Flush()
	return writer.Error()
}

func writeQuoteTable(w io.Writer, quote domain.Quote, enabled bool) error {
	changeStr := formatFloat(quote.Change)
	changeRateStr := fmt.Sprintf("%.2f%%", quote.ChangeRate*100)
	if _, err := fmt.Fprintf(
		w,
		i18n.T("output.quote.summary"),
		quote.ProductCode,
		quote.Symbol,
		quote.Name,
		quote.Market,
		quote.MarketCode,
		quote.Currency,
		formatFloat(quote.ReferencePrice),
		formatFloat(quote.Last),
		profitText(changeStr, quote.Change, enabled),
		profitText(changeRateStr, quote.ChangeRate, enabled),
		formatFloat(quote.Volume),
	); err != nil {
		return err
	}

	// Same-day OHLC + 52-week range + market cap/trading value/strength (only
	// populated when the v3 details endpoint filled them in).
	if quote.Open != 0 || quote.High != 0 || quote.Low != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.ohlc"),
			formatFloat(quote.Open), formatFloat(quote.High), formatFloat(quote.Low), formatFloat(quote.Last)); err != nil {
			return err
		}
	}
	if quote.High52w != 0 || quote.Low52w != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.week52"), formatFloat(quote.High52w), formatFloat(quote.Low52w)); err != nil {
			return err
		}
	}
	if quote.MarketCap != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.marketCap"), formatFloat(quote.MarketCap)); err != nil {
			return err
		}
	}
	if quote.TradingValue != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.tradingValue"), formatFloat(quote.TradingValue)); err != nil {
			return err
		}
	}
	if quote.TradingStrength != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.tradingStrength"), quote.TradingStrength); err != nil {
			return err
		}
	}
	if quote.UpperLimit != 0 || quote.LowerLimit != 0 {
		if _, err := fmt.Fprintf(w, i18n.T("output.quote.limits"), formatFloat(quote.UpperLimit), formatFloat(quote.LowerLimit)); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(w, i18n.T("output.quote.footer"),
		quote.Status, quote.BadgeCount, quote.NoticeCount,
		quote.FetchedAt.Format("2006-01-02 15:04:05Z07:00"))
	return err
}

func WriteQuotes(w io.Writer, format Format, quotes []domain.Quote) error {
	return WriteQuotesWithCharts(w, format, quotes, nil)
}

func WriteQuotesWithCharts(w io.Writer, format Format, quotes []domain.Quote, charts []domain.Chart) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, quotes)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{
			"symbol", "name", "market", "currency", "last", "change", "change_rate", "volume",
		}); err != nil {
			return err
		}
		for _, q := range quotes {
			if err := writer.Write([]string{
				q.Symbol, q.Name, q.Market, q.Currency,
				formatFloat(q.Last), formatFloat(q.Change), formatFloat(q.ChangeRate), formatFloat(q.Volume),
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		enabled := colorEnabled(w, format)
		showCharts := charts != nil
		headers := []string{
			i18n.T("output.quotes.header.symbol"),
			i18n.T("output.quotes.header.name"),
			i18n.T("output.quotes.header.price"),
			i18n.T("output.quotes.header.change"),
			i18n.T("output.quotes.header.changeRate"),
		}
		if showCharts {
			headers = append(headers, i18n.T("output.quotes.header.chart"))
		}
		var plainRows, coloredRows [][]string
		for i, q := range quotes {
			changeStr := formatKRW(q.Change)
			if q.Change > 0 {
				changeStr = "+" + changeStr
			}
			rateStr := formatPct(q.ChangeRate)
			plain := []string{
				q.Symbol,
				q.Name,
				formatKRW(q.Last),
				changeStr,
				rateStr,
			}
			colored := []string{
				q.Symbol,
				q.Name,
				formatKRW(q.Last),
				profitText(changeStr, q.Change, enabled),
				profitText(rateStr, q.ChangeRate, enabled),
			}
			if showCharts {
				sparkline := ""
				if i < len(charts) && len(charts[i].Candles) > 0 {
					sparkline = renderSparkline(charts[i].Candles, 20)
				}
				plain = append(plain, sparkline)
				colored = append(colored, sparkline)
			}
			plainRows = append(plainRows, plain)
			coloredRows = append(coloredRows, colored)
		}
		return renderTableColored(w, headers, plainRows, coloredRows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
