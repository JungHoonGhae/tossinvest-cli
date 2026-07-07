package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WriteTrades(w io.Writer, format Format, list domain.TradeList) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"time", "price", "volume", "trade_type", "cumulative_volume"}); err != nil {
			return err
		}
		for _, t := range list.Trades {
			if err := cw.Write([]string{
				t.Time, formatFloat(t.Price), formatFloat(t.Volume),
				t.TradeType, formatFloat(t.CumulativeVolume),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.trades.header.time"),
			i18n.T("output.trades.header.price"),
			i18n.T("output.trades.header.volume"),
			i18n.T("output.trades.header.side"),
		}
		rows := make([][]string, 0, len(list.Trades))
		for _, t := range list.Trades {
			side := t.TradeType
			switch t.TradeType {
			case "BUY":
				side = i18n.T("output.trades.side.buy")
			case "SELL":
				side = i18n.T("output.trades.side.sell")
			}
			rows = append(rows, []string{t.Time, formatKRW(t.Price), formatFloat(t.Volume), side})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WritePriceLimits(w io.Writer, format Format, pl domain.PriceLimits) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(pl)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"date", "upper_limit", "lower_limit"}); err != nil {
			return err
		}
		if err := cw.Write([]string{pl.Date, formatFloat(pl.UpperLimit), formatFloat(pl.LowerLimit)}); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := pl.Name
		if name == "" {
			name = pl.Symbol
		}
		_, err := fmt.Fprintf(w,
			"%s (%s) · %s\n"+i18n.T("output.priceLimits.upper")+": %s\n"+i18n.T("output.priceLimits.lower")+": %s\n",
			name, pl.ProductCode, pl.Date,
			formatKRW(pl.UpperLimit), formatKRW(pl.LowerLimit),
		)
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteStockWarnings(w io.Writer, format Format, sw domain.StockWarnings) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sw)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"type", "title", "text", "level"}); err != nil {
			return err
		}
		for _, x := range sw.Warnings {
			if err := cw.Write([]string{x.Type, x.Title, x.Text, x.Level}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := sw.Name
		if name == "" {
			name = sw.Symbol
		}
		if len(sw.Warnings) == 0 {
			_, err := fmt.Fprintf(w, i18n.T("output.warnings.none"), name, sw.ProductCode)
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.warnings.count"), name, sw.ProductCode, len(sw.Warnings)); err != nil {
			return err
		}
		for _, x := range sw.Warnings {
			label := x.Title
			if label == "" {
				label = x.Type
			}
			line := "• " + label
			if x.Text != "" {
				line += " — " + x.Text
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteTradingHours(w io.Writer, format Format, th domain.TradingHours) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(th)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"market", "session", "date", "start_time", "end_time"}); err != nil {
			return err
		}
		for _, row := range [][]string{
			{"KR", "today", th.KR.Date, th.KR.StartTime, th.KR.EndTime},
			{"US", "today", th.US.Date, th.US.StartTime, th.US.EndTime},
			{"KR", "next", th.NextKR.Date, th.NextKR.StartTime, th.NextKR.EndTime},
			{"US", "next", th.NextUS.Date, th.NextUS.StartTime, th.NextUS.EndTime},
		} {
			if err := cw.Write(row); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.hours.header.market"),
			i18n.T("output.hours.header.session"),
			i18n.T("output.hours.header.date"),
			i18n.T("output.hours.header.open"),
			i18n.T("output.hours.header.close"),
		}
		today := i18n.T("output.hours.session.today")
		next := i18n.T("output.hours.session.next")
		rows := [][]string{
			{"KR", today, th.KR.Date, sessionTime(th.KR.StartTime), sessionTime(th.KR.EndTime)},
			{"US", today, th.US.Date, sessionTime(th.US.StartTime), sessionTime(th.US.EndTime)},
		}
		// Also show the next business day when today's session is closed.
		if th.KR.StartTime == "" && th.NextKR.Date != "" {
			rows = append(rows, []string{"KR", next, th.NextKR.Date, sessionTime(th.NextKR.StartTime), sessionTime(th.NextKR.EndTime)})
		}
		if th.US.StartTime == "" && th.NextUS.Date != "" {
			rows = append(rows, []string{"US", next, th.NextUS.Date, sessionTime(th.NextUS.StartTime), sessionTime(th.NextUS.EndTime)})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteExchangeRates(w io.Writer, format Format, er domain.ExchangeRates) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(er)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"code", "name", "base", "close"}); err != nil {
			return err
		}
		for _, r := range er.Rates {
			if err := cw.Write([]string{r.Code, r.Name, formatFloat(r.Base), formatFloat(r.Close)}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.fx.header.name"),
			i18n.T("output.fx.header.base"),
			i18n.T("output.fx.header.current"),
		}
		rows := make([][]string, 0, len(er.Rates))
		for _, r := range er.Rates {
			rows = append(rows, []string{r.Name, formatFloat(r.Base), formatFloat(r.Close)})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteScreenerPresets(w io.Writer, format Format, sp domain.ScreenerPresets) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sp)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"id", "name", "description"}); err != nil {
			return err
		}
		for _, p := range sp.Presets {
			if err := cw.Write([]string{p.ID, p.Name, p.Description}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.screener.presets.header.id"),
			i18n.T("output.screener.presets.header.name"),
			i18n.T("output.screener.presets.header.description"),
		}
		rows := make([][]string, 0, len(sp.Presets))
		for _, p := range sp.Presets {
			rows = append(rows, []string{p.ID, p.Name, p.Description})
		}
		if err := renderTable(w, headers, rows); err != nil {
			return err
		}
		_, err := fmt.Fprint(w, i18n.T("output.screener.presets.hint"))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteScreenerResult(w io.Writer, format Format, sr domain.ScreenerResult) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sr)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"product_code", "name", "close", "change", "change_rate"}); err != nil {
			return err
		}
		for _, s := range sr.Stocks {
			if err := cw.Write([]string{
				s.ProductCode, s.Name, formatFloat(s.Close), formatFloat(s.Change), formatFloat(s.ChangeRate),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.screener.result.header"),
			sr.PresetName, sr.Nation, sr.TotalCount, len(sr.Stocks)); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.screener.result.header.symbol"),
			i18n.T("output.screener.result.header.name"),
			i18n.T("output.screener.result.header.price"),
			i18n.T("output.screener.result.header.changeRate"),
		}
		rows := make([][]string, 0, len(sr.Stocks))
		for _, s := range sr.Stocks {
			rows = append(rows, []string{s.ProductCode, s.Name, formatFloat(s.Close), formatPct(s.ChangeRate)})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteAISignals(w io.Writer, format Format, sg domain.AISignals) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sg)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"asset_name", "title", "keyword", "fluctuation", "stock_code"}); err != nil {
			return err
		}
		for _, s := range sg.Signals {
			if err := cw.Write([]string{s.AssetName, s.Title, s.Keyword, s.Fluctuation, s.StockCode}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		label := sg.Label
		if label == "" {
			label = i18n.T("output.signals.defaultLabel")
		}
		if _, err := fmt.Fprintf(w, "%s\n", label); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.signals.header.symbol"),
			i18n.T("output.signals.header.signal"),
			i18n.T("output.signals.header.keyword"),
			i18n.T("output.signals.header.change"),
		}
		rows := make([][]string, 0, len(sg.Signals))
		for _, s := range sg.Signals {
			rows = append(rows, []string{s.AssetName, s.Title, s.Keyword, s.Fluctuation})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteTradingFlows(w io.Writer, format Format, tf domain.TradingFlows) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tf)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"date", "net_individuals", "net_foreigner", "net_institution"}); err != nil {
			return err
		}
		for _, f := range tf.Flows {
			if err := cw.Write([]string{
				f.Date, formatFloat(f.NetIndividuals), formatFloat(f.NetForeigner), formatFloat(f.NetInstitution),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := tf.Name
		if name == "" {
			name = tf.Symbol
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.flows.title"), name, tf.ProductCode); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.flows.header.date"),
			i18n.T("output.flows.header.individual"),
			i18n.T("output.flows.header.foreign"),
			i18n.T("output.flows.header.institution"),
		}
		rows := make([][]string, 0, len(tf.Flows))
		for _, f := range tf.Flows {
			rows = append(rows, []string{
				f.Date, signedInt(f.NetIndividuals), signedInt(f.NetForeigner), signedInt(f.NetInstitution),
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// signedInt formats a net volume with explicit +/- sign and thousands commas.
func signedInt(v float64) string {
	s := formatKRW(v)
	if v > 0 {
		return "+" + s
	}
	return s
}

func WriteMarketIndices(w io.Writer, format Format, mi domain.MarketIndices) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(mi)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"code", "name", "nation", "latest", "base", "change", "change_rate"}); err != nil {
			return err
		}
		for _, x := range mi.Indices {
			if err := cw.Write([]string{
				x.Code, x.Name, x.Nation, formatFloat(x.Latest), formatFloat(x.Base),
				formatFloat(x.Change), formatFloat(x.ChangeRate),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.indices.header.index"),
			i18n.T("output.indices.header.code"),
			i18n.T("output.indices.header.current"),
			i18n.T("output.indices.header.change"),
			i18n.T("output.indices.header.changeRate"),
		}
		rows := make([][]string, 0, len(mi.Indices))
		for _, x := range mi.Indices {
			sign := ""
			if x.Change > 0 {
				sign = "+"
			}
			rows = append(rows, []string{
				x.Name,
				x.Code,
				fmt.Sprintf("%.2f", x.Latest),
				sign + fmt.Sprintf("%.2f", x.Change),
				formatPct(x.ChangeRate),
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteIndexQuote renders a single index's detailed quote.
func WriteIndexQuote(w io.Writer, format Format, q domain.IndexQuote) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(q)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"code", "name", "open", "high", "low", "close", "change", "change_rate", "high_52w", "low_52w"}); err != nil {
			return err
		}
		if err := cw.Write([]string{
			q.Code, q.Name, formatFloat(q.Open), formatFloat(q.High), formatFloat(q.Low), formatFloat(q.Close),
			formatFloat(q.Change), formatFloat(q.ChangeRate), formatFloat(q.High52w), formatFloat(q.Low52w),
		}); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		sign := ""
		if q.Change > 0 {
			sign = "+"
		}
		fmt.Fprintf(w, "%s (%s)\n", q.Name, q.Code)
		fmt.Fprintf(w, "  %-8s %.2f  (%s%.2f, %s)\n", i18n.T("output.indexQuote.current"), q.Close, sign, q.Change, formatPct(q.ChangeRate))
		fmt.Fprintf(w, "  %-8s %.2f / %.2f / %.2f\n", i18n.T("output.indexQuote.ohl"), q.Open, q.High, q.Low)
		if q.High52w != 0 || q.Low52w != 0 {
			fmt.Fprintf(w, "  %-8s %s %.2f / %s %.2f\n", i18n.T("output.indexQuote.week52"), i18n.T("output.indexQuote.week52.high"), q.High52w, i18n.T("output.indexQuote.week52.low"), q.Low52w)
		}
		if q.Volume != 0 {
			fmt.Fprintf(w, "  %-8s %s\n", i18n.T("output.indexQuote.volume"), formatFloat(q.Volume))
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteStockRanking(w io.Writer, format Format, sr domain.StockRanking) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sr)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"rank", "symbol", "name", "market", "product_code"}); err != nil {
			return err
		}
		for _, x := range sr.Stocks {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", x.Rank), x.Symbol, x.Name, x.Market, x.ProductCode,
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.ranking.header.rank"),
			i18n.T("output.ranking.header.symbol"),
			i18n.T("output.ranking.header.name"),
			i18n.T("output.ranking.header.market"),
		}
		rows := make([][]string, 0, len(sr.Stocks))
		for _, x := range sr.Stocks {
			rows = append(rows, []string{fmt.Sprintf("%d", x.Rank), x.Symbol, x.Name, x.Market})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// sessionTime trims the "HH:MM:SS.mmm" wire format down to "HH:MM", and shows a
// dash for a closed/holiday session (null → empty string).
func sessionTime(s string) string {
	if s == "" {
		return i18n.T("output.session.closed")
	}
	if len(s) >= 5 {
		return s[:5]
	}
	return s
}

// WriteOrderBook renders the bid/ask depth ladder (호가). Offers (매도) are
// printed high-to-low above the spread, bids (매수) below, matching how a
// Korean orderbook ladder reads top-to-bottom.
func WriteOrderBook(w io.Writer, format Format, ob domain.OrderBook) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(ob)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"side", "level", "price", "volume"}); err != nil {
			return err
		}
		for i, lv := range ob.Offers {
			if err := cw.Write([]string{"offer", fmt.Sprintf("%d", i+1), formatFloat(lv.Price), formatFloat(lv.Volume)}); err != nil {
				return err
			}
		}
		for i, lv := range ob.Bids {
			if err := cw.Write([]string{"bid", fmt.Sprintf("%d", i+1), formatFloat(lv.Price), formatFloat(lv.Volume)}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := ob.Name
		if name == "" {
			name = ob.Symbol
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.orderbook.title"), name, ob.ProductCode); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%-12s %12s  %s\n", i18n.T("output.orderbook.header.volume"), i18n.T("output.orderbook.header.price"), i18n.T("output.orderbook.asks")); err != nil {
			return err
		}
		// Offers high-to-low (worst ask at top, best ask just above spread).
		for i := len(ob.Offers) - 1; i >= 0; i-- {
			lv := ob.Offers[i]
			if _, err := fmt.Fprintf(w, "%12s  %12s\n", formatFloat(lv.Volume), formatKRW(lv.Price)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s\n", "──────────────────────────"); err != nil {
			return err
		}
		// Bids best-first (highest bid just below spread).
		for _, lv := range ob.Bids {
			if _, err := fmt.Fprintf(w, "%12s  %12s  %s\n", formatKRW(lv.Price), formatFloat(lv.Volume), i18n.T("output.orderbook.bids")); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, i18n.T("output.orderbook.totalLine"), formatFloat(ob.TotalOffer), formatFloat(ob.TotalBid))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSellableQuantity renders how many shares can be sold now.
func WriteSellableQuantity(w io.Writer, format Format, sq domain.SellableQuantity) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sq)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"product_code", "symbol", "sellable_quantity"}); err != nil {
			return err
		}
		if err := cw.Write([]string{sq.ProductCode, sq.Symbol, formatFloat(sq.Quantity)}); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := sq.Name
		if name == "" {
			name = sq.Symbol
		}
		_, err := fmt.Fprintf(w, i18n.T("output.sellable.line"), name, sq.ProductCode, formatFloat(sq.Quantity))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteCommission renders the commission and tax rate for a symbol.
func WriteCommission(w io.Writer, format Format, c domain.Commission) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(c)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"product_code", "symbol", "commission_rate", "tax_rate"}); err != nil {
			return err
		}
		if err := cw.Write([]string{c.ProductCode, c.Symbol, formatFloat(c.CommissionRate), formatFloat(c.TaxRate)}); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		name := c.Name
		if name == "" {
			name = c.Symbol
		}
		_, err := fmt.Fprintf(w, i18n.T("output.commission.line"),
			name, c.ProductCode, formatPercent(c.CommissionRate), formatPercent(c.TaxRate))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// formatPercent renders a fractional rate (0.002) as a percent string (0.2%).
func formatPercent(rate float64) string {
	return formatFloat(rate*100) + "%"
}

// WriteInvestorRankings renders per-investor-type net-buy rankings.
func WriteInvestorRankings(w io.Writer, format Format, ir domain.InvestorRankings) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(ir)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"investor_type", "rank", "product_code", "name", "net_buy_amount", "close"}); err != nil {
			return err
		}
		for _, r := range ir.Rankings {
			for _, s := range r.Stocks {
				if err := cw.Write([]string{r.InvestorType, fmt.Sprintf("%d", s.Rank), s.ProductCode, s.Name, formatFloat(s.NetBuyAmount), formatFloat(s.Close)}); err != nil {
					return err
				}
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		for _, r := range ir.Rankings {
			if _, err := fmt.Fprintf(w, i18n.T("output.investorRankings.header"), r.InvestorType); err != nil {
				return err
			}
			for _, s := range r.Stocks {
				if _, err := fmt.Fprintf(w, "%2d   %-18s  %s\n", s.Rank, s.Name, formatKRW(s.NetBuyAmount)); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteEarningCalls renders the upcoming earnings-call calendar.
func WriteEarningCalls(w io.Writer, format Format, ec domain.EarningCalls) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(ec)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"live_at", "company_name", "company_code", "title", "status", "category"}); err != nil {
			return err
		}
		for _, e := range ec.Events {
			if err := cw.Write([]string{e.LiveAt, e.CompanyName, e.CompanyCode, e.Title, e.Status, e.Category}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if len(ec.Events) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.earnings.empty"))
			return err
		}
		if _, err := fmt.Fprint(w, i18n.T("output.earnings.header")); err != nil {
			return err
		}
		for _, e := range ec.Events {
			when := e.LiveAt
			if len(when) >= 16 {
				when = when[:16] // YYYY-MM-DDTHH:MM
			}
			if _, err := fmt.Fprintf(w, "%-17s  %-14s  %s\n", when, e.CompanyName, e.Category); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// divAmt renders a dual-currency dividend amount (e.g. "1,234,567원  $1,234.56").
func divAmt(a domain.DividendAmount) string {
	s := formatKRW(a.KRW) + i18n.T("output.dividends.krwSuffix")
	if a.USD != 0 {
		s += fmt.Sprintf("  $%s", formatFloat(a.USD))
	}
	return s
}

// WriteDividends renders an annual dividend report.
func WriteDividends(w io.Writer, format Format, d domain.Dividends) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"month", "total_krw", "total_usd"}); err != nil {
			return err
		}
		for _, m := range d.Monthly {
			if err := cw.Write([]string{fmt.Sprintf("%d", m.Month), formatFloat(m.Summary.Total.KRW), formatFloat(m.Summary.Total.USD)}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		basis := i18n.T("output.dividends.basis.receivedEstimated")
		if d.ByPaymentDate {
			basis = i18n.T("output.dividends.basis.paymentDate")
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.dividends.yearHeader"), d.Year, basis); err != nil {
			return err
		}
		fmt.Fprintf(w, "  %-7s  %s\n", i18n.T("output.dividends.total"), divAmt(d.Summary.Total))
		fmt.Fprintf(w, "  %-7s  %s\n", i18n.T("output.dividends.paid"), divAmt(d.Summary.Paid))
		fmt.Fprintf(w, "  %-7s  %s\n", i18n.T("output.dividends.estimated"), divAmt(d.Summary.Estimated))
		if d.Summary.Tax != nil {
			fmt.Fprintf(w, "  %-7s  %s\n", i18n.T("output.dividends.tax"), divAmt(*d.Summary.Tax))
		}
		if len(d.Regions) > 0 {
			fmt.Fprint(w, i18n.T("output.dividends.byRegion"))
			for _, r := range d.Regions {
				fmt.Fprintf(w, "  %-3s  %s\n", strings.ToUpper(r.Region), divAmt(r.Summary.Total))
			}
		}
		if len(d.Monthly) > 0 {
			fmt.Fprint(w, i18n.T("output.dividends.byMonth"))
			for _, m := range d.Monthly {
				if m.Summary.Total.KRW == 0 && m.Summary.Total.USD == 0 {
					continue
				}
				fmt.Fprintf(w, "  %2d%s  %s\n", m.Month, i18n.T("output.dividends.monthSuffix"), divAmt(m.Summary.Total))
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteCommunityRanking renders a community leaderboard. Columns vary by type.
func WriteCommunityRanking(w io.Writer, format Format, r domain.CommunityRanking) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"rank", "nickname", "user_profile_id", "description", "profit_amount_krw", "profit_rate", "following_count", "following_increase"}); err != nil {
			return err
		}
		for _, u := range r.Users {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", u.Rank), u.Nickname, fmt.Sprintf("%d", u.UserProfileID), u.Description,
				formatFloat(u.ProfitAmountKRW), formatFloat(u.ProfitRate),
				fmt.Sprintf("%d", u.FollowingCount), fmt.Sprintf("%d", u.FollowingIncrease),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if len(r.Users) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.community.empty"))
			return err
		}
		switch r.Type {
		case "TOP_10_PROFIT_ROSS_AMOUNT":
			fmt.Fprint(w, i18n.T("output.community.profitHeader"))
			for _, u := range r.Users {
				fmt.Fprintf(w, "%2d   %-16s  %12s%s  %.1f%%\n", u.Rank, u.Nickname, formatKRW(u.ProfitAmountKRW), i18n.T("output.community.krwSuffix"), u.ProfitRate*100)
			}
		case "TOP_10_FOLLOWING_INCREASE":
			fmt.Fprint(w, i18n.T("output.community.followingHeader"))
			for _, u := range r.Users {
				fmt.Fprintf(w, "%2d   %-16s  %7d  +%d\n", u.Rank, u.Nickname, u.FollowingCount, u.FollowingIncrease)
			}
		default: // INFLUENCER
			fmt.Fprint(w, i18n.T("output.community.influencerHeader"))
			for _, u := range r.Users {
				fmt.Fprintf(w, "%2d   %s\n", u.Rank, u.Nickname)
				if u.Description != "" {
					desc := strings.ReplaceAll(u.Description, "\n", " ")
					fmt.Fprintf(w, "     %s\n", desc)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSectors renders an industry (TICS) list with fluctuation rates.
// WriteThemeRankings renders the TICS theme fluctuation ranking. Table output
// colours the change rate (KR convention: red up / blue down) only when the
// colour gate is on; JSON/CSV are byte-identical regardless.
func WriteThemeRankings(w io.Writer, format Format, r domain.ThemeRankings) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"ranking", "tics_id", "title", "change_rate", "rise_company_count", "total_count"}); err != nil {
			return err
		}
		for _, t := range r.Items {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", t.Ranking), t.TicsID, t.Title,
				formatFloat(t.ChangeRate), fmt.Sprintf("%d", t.RiseCompanyCount), fmt.Sprintf("%d", t.TotalCount),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if len(r.Items) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.themes.empty"))
			return err
		}
		enabled := colorEnabled(w, format)
		headers := []string{
			i18n.T("output.themes.header.theme"),
			i18n.T("output.themes.header.changeRate"),
			i18n.T("output.themes.header.riseTotal"),
		}
		plain := make([][]string, 0, len(r.Items))
		disp := make([][]string, 0, len(r.Items))
		for _, t := range r.Items {
			name := fmt.Sprintf("%2d. %s", t.Ranking, t.Title)
			rate := fmt.Sprintf("%+.2f%%", t.ChangeRate)
			rise := fmt.Sprintf("%d/%d", t.RiseCompanyCount, t.TotalCount)
			plain = append(plain, []string{name, rate, rise})
			disp = append(disp, []string{name, profitText(rate, t.ChangeRate, enabled), rise})
		}
		return renderTableColored(w, headers, plain, disp)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteSectors(w io.Writer, format Format, sectors domain.Sectors) error {
	list := sectors.Items
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sectors)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"id", "title", "company_count", "one_day_rate", "one_month_rate", "one_year_rate"}); err != nil {
			return err
		}
		for _, s := range list {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", s.ID), s.Title, fmt.Sprintf("%d", s.CompanyCount),
				formatFloat(s.OneDayRate), formatFloat(s.OneMonthRate), formatFloat(s.OneYearRate),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if len(list) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.sectors.empty"))
			return err
		}
		if _, err := fmt.Fprint(w, i18n.T("output.sectors.header")); err != nil {
			return err
		}
		for _, s := range list {
			if _, err := fmt.Fprintf(w, "%-14s  %5d  %7.2f%%  %7.2f%%  %7.2f%%\n",
				s.Title, s.CompanyCount, s.OneDayRate, s.OneMonthRate, s.OneYearRate); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteNewsBriefing renders the personalized AI news briefing.
func WriteNewsBriefing(w io.Writer, format Format, b domain.NewsBriefing) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(b)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"category", "title", "agency", "created_at"}); err != nil {
			return err
		}
		for _, it := range b.Items {
			for _, n := range it.News {
				if err := cw.Write([]string{it.CategoryType, n.Title, n.Agency, n.CreatedAt}); err != nil {
					return err
				}
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		if len(b.Items) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.briefing.empty"))
			return err
		}
		for _, it := range b.Items {
			header := it.CategoryType
			if len(it.Keywords) > 0 {
				header += " · " + strings.Join(it.Keywords, ", ")
			}
			fmt.Fprintf(w, "\n[%s]\n", header)
			for _, n := range it.News {
				agency := n.Agency
				if agency != "" {
					agency = " (" + agency + ")"
				}
				fmt.Fprintf(w, "  · %s%s\n", n.Title, agency)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteRanking renders the official stock ranking (거래대금/등락률 상위 등).
func WriteRanking(w io.Writer, format Format, r domain.Ranking) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"rank", "symbol", "currency", "last_price", "base_price", "change_rate", "trading_volume", "trading_amount"}); err != nil {
			return err
		}
		for _, x := range r.Items {
			if err := cw.Write([]string{
				fmt.Sprintf("%d", x.Rank), x.Symbol, x.Currency,
				strconv.FormatFloat(x.LastPrice, 'f', -1, 64),
				strconv.FormatFloat(x.BasePrice, 'f', -1, 64),
				strconv.FormatFloat(x.ChangeRate, 'f', -1, 64),
				strconv.FormatFloat(x.TradingVolume, 'f', -1, 64),
				strconv.FormatFloat(x.TradingAmount, 'f', -1, 64),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.officialRanking.header.rank"),
			i18n.T("output.officialRanking.header.symbol"),
			i18n.T("output.officialRanking.header.price"),
			i18n.T("output.officialRanking.header.changeRate"),
			i18n.T("output.officialRanking.header.amount"),
		}
		rows := make([][]string, 0, len(r.Items))
		for _, x := range r.Items {
			rows = append(rows, []string{
				fmt.Sprintf("%d", x.Rank),
				x.Symbol,
				strconv.FormatFloat(x.LastPrice, 'f', -1, 64),
				fmt.Sprintf("%.2f%%", x.ChangeRate*100),
				strconv.FormatFloat(x.TradingAmount, 'f', -1, 64),
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteMarketIndicatorPrices renders official market-indicator current prices.
func WriteMarketIndicatorPrices(w io.Writer, format Format, p domain.MarketIndicatorPrices) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"symbol", "last_price", "timestamp"}); err != nil {
			return err
		}
		for _, x := range p.Indicators {
			if err := cw.Write([]string{x.Symbol, strconv.FormatFloat(x.LastPrice, 'f', -1, 64), x.Timestamp}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.marketIndicator.header.symbol"),
			i18n.T("output.marketIndicator.header.price"),
			i18n.T("output.marketIndicator.header.time"),
		}
		rows := make([][]string, 0, len(p.Indicators))
		for _, x := range p.Indicators {
			rows = append(rows, []string{x.Symbol, strconv.FormatFloat(x.LastPrice, 'f', -1, 64), x.Timestamp})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteMarketIndicatorCandles renders official market-indicator OHLCV candles.
func WriteMarketIndicatorCandles(w io.Writer, format Format, c domain.MarketIndicatorCandles) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(c)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"timestamp", "open", "high", "low", "close", "volume"}); err != nil {
			return err
		}
		for _, x := range c.Candles {
			if err := cw.Write([]string{
				x.Timestamp,
				strconv.FormatFloat(x.Open, 'f', -1, 64),
				strconv.FormatFloat(x.High, 'f', -1, 64),
				strconv.FormatFloat(x.Low, 'f', -1, 64),
				strconv.FormatFloat(x.Close, 'f', -1, 64),
				strconv.FormatFloat(x.Volume, 'f', -1, 64),
			}); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		headers := []string{
			i18n.T("output.indicatorCandle.header.time"),
			i18n.T("output.indicatorCandle.header.open"),
			i18n.T("output.indicatorCandle.header.high"),
			i18n.T("output.indicatorCandle.header.low"),
			i18n.T("output.indicatorCandle.header.close"),
			i18n.T("output.indicatorCandle.header.volume"),
		}
		rows := make([][]string, 0, len(c.Candles))
		for _, x := range c.Candles {
			rows = append(rows, []string{
				x.Timestamp,
				strconv.FormatFloat(x.Open, 'f', -1, 64),
				strconv.FormatFloat(x.High, 'f', -1, 64),
				strconv.FormatFloat(x.Low, 'f', -1, 64),
				strconv.FormatFloat(x.Close, 'f', -1, 64),
				strconv.FormatFloat(x.Volume, 'f', -1, 64),
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
