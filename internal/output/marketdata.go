package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
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
		headers := []string{"시각", "체결가", "수량", "구분"}
		rows := make([][]string, 0, len(list.Trades))
		for _, t := range list.Trades {
			side := t.TradeType
			switch t.TradeType {
			case "BUY":
				side = "매수"
			case "SELL":
				side = "매도"
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
			"%s (%s) · %s\n상한가: %s\n하한가: %s\n",
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
			_, err := fmt.Fprintf(w, "%s (%s): 매수 유의사항 없음\n", name, sw.ProductCode)
			return err
		}
		if _, err := fmt.Fprintf(w, "%s (%s) 매수 유의사항 %d건\n", name, sw.ProductCode, len(sw.Warnings)); err != nil {
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
		headers := []string{"시장", "구분", "일자", "개장", "마감"}
		rows := [][]string{
			{"KR", "오늘", th.KR.Date, sessionTime(th.KR.StartTime), sessionTime(th.KR.EndTime)},
			{"US", "오늘", th.US.Date, sessionTime(th.US.StartTime), sessionTime(th.US.EndTime)},
		}
		// 오늘 휴장 시 다음 영업일도 표시
		if th.KR.StartTime == "" && th.NextKR.Date != "" {
			rows = append(rows, []string{"KR", "다음", th.NextKR.Date, sessionTime(th.NextKR.StartTime), sessionTime(th.NextKR.EndTime)})
		}
		if th.US.StartTime == "" && th.NextUS.Date != "" {
			rows = append(rows, []string{"US", "다음", th.NextUS.Date, sessionTime(th.NextUS.StartTime), sessionTime(th.NextUS.EndTime)})
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
		headers := []string{"이름", "기준", "현재"}
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
		headers := []string{"ID", "이름", "설명"}
		rows := make([][]string, 0, len(sp.Presets))
		for _, p := range sp.Presets {
			rows = append(rows, []string{p.ID, p.Name, p.Description})
		}
		if err := renderTable(w, headers, rows); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "\n실행: tossctl market screener <ID> [--nation kr|us]")
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
		if _, err := fmt.Fprintf(w, "%s (%s) — %d종목 중 상위 %d\n",
			sr.PresetName, sr.Nation, sr.TotalCount, len(sr.Stocks)); err != nil {
			return err
		}
		headers := []string{"종목", "이름", "현재가", "변동률"}
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
			label = "AI 시그널"
		}
		if _, err := fmt.Fprintf(w, "%s\n", label); err != nil {
			return err
		}
		headers := []string{"종목", "시그널", "키워드", "등락"}
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
		if _, err := fmt.Fprintf(w, "%s (%s) 수급 — 순매수(주)\n", name, tf.ProductCode); err != nil {
			return err
		}
		headers := []string{"일자", "개인", "외국인", "기관"}
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
		headers := []string{"지수", "코드", "현재", "변동", "변동률"}
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
		fmt.Fprintf(w, "  현재     %.2f  (%s%.2f, %s)\n", q.Close, sign, q.Change, formatPct(q.ChangeRate))
		fmt.Fprintf(w, "  시/고/저  %.2f / %.2f / %.2f\n", q.Open, q.High, q.Low)
		if q.High52w != 0 || q.Low52w != 0 {
			fmt.Fprintf(w, "  52주     고 %.2f / 저 %.2f\n", q.High52w, q.Low52w)
		}
		if q.Volume != 0 {
			fmt.Fprintf(w, "  거래량   %s\n", formatFloat(q.Volume))
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
		headers := []string{"순위", "종목", "이름", "시장"}
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
		return "휴장"
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
		if _, err := fmt.Fprintf(w, "%s (%s) · 호가\n", name, ob.ProductCode); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%-12s %12s  매도(offer)\n", "잔량", "호가"); err != nil {
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
			if _, err := fmt.Fprintf(w, "%12s  %12s  매수(bid)\n", formatKRW(lv.Price), formatFloat(lv.Volume)); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "\n총 매도잔량: %s · 총 매수잔량: %s\n", formatFloat(ob.TotalOffer), formatFloat(ob.TotalBid))
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
		_, err := fmt.Fprintf(w, "%s (%s)\n매도가능수량: %s주\n", name, sq.ProductCode, formatFloat(sq.Quantity))
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
		_, err := fmt.Fprintf(w, "%s (%s)\n수수료율: %s\n세금(거래세)율: %s\n",
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
			if _, err := fmt.Fprintf(w, "\n[%s] 순매수 상위\n순위  종목                    순매수\n", r.InvestorType); err != nil {
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
			_, err := fmt.Fprintln(w, "예정된 어닝콜이 없습니다")
			return err
		}
		if _, err := fmt.Fprintf(w, "예정 어닝콜\n일시               기업            구분\n"); err != nil {
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
	s := formatKRW(a.KRW) + "원"
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
		basis := "수령·예정 기준"
		if d.ByPaymentDate {
			basis = "지급일 기준"
		}
		if _, err := fmt.Fprintf(w, "%d년 배당 (%s)\n", d.Year, basis); err != nil {
			return err
		}
		fmt.Fprintf(w, "  총 배당  %s\n", divAmt(d.Summary.Total))
		fmt.Fprintf(w, "  수령     %s\n", divAmt(d.Summary.Paid))
		fmt.Fprintf(w, "  예정     %s\n", divAmt(d.Summary.Estimated))
		if d.Summary.Tax != nil {
			fmt.Fprintf(w, "  세금     %s\n", divAmt(*d.Summary.Tax))
		}
		if len(d.Regions) > 0 {
			fmt.Fprintf(w, "지역별\n")
			for _, r := range d.Regions {
				fmt.Fprintf(w, "  %-3s  %s\n", strings.ToUpper(r.Region), divAmt(r.Summary.Total))
			}
		}
		if len(d.Monthly) > 0 {
			fmt.Fprintf(w, "월별\n")
			for _, m := range d.Monthly {
				if m.Summary.Total.KRW == 0 && m.Summary.Total.USD == 0 {
					continue
				}
				fmt.Fprintf(w, "  %2d월  %s\n", m.Month, divAmt(m.Summary.Total))
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
			_, err := fmt.Fprintln(w, "랭킹 데이터가 없습니다")
			return err
		}
		switch r.Type {
		case "TOP_10_PROFIT_ROSS_AMOUNT":
			fmt.Fprintf(w, "수익금 랭킹\n순위  닉네임            수익금          수익률\n")
			for _, u := range r.Users {
				fmt.Fprintf(w, "%2d   %-16s  %12s원  %.1f%%\n", u.Rank, u.Nickname, formatKRW(u.ProfitAmountKRW), u.ProfitRate*100)
			}
		case "TOP_10_FOLLOWING_INCREASE":
			fmt.Fprintf(w, "팔로워 급증 랭킹\n순위  닉네임            팔로워     증가\n")
			for _, u := range r.Users {
				fmt.Fprintf(w, "%2d   %-16s  %7d  +%d\n", u.Rank, u.Nickname, u.FollowingCount, u.FollowingIncrease)
			}
		default: // INFLUENCER
			fmt.Fprintf(w, "인플루언서 랭킹\n순위  닉네임\n")
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
			_, err := fmt.Fprintln(w, "테마 데이터가 없습니다")
			return err
		}
		enabled := colorEnabled(w, format)
		if _, err := fmt.Fprintf(w, "순위  테마            등락률      상승/전체\n"); err != nil {
			return err
		}
		for _, t := range r.Items {
			rateStr := fmt.Sprintf("%+8.2f%%", t.ChangeRate)
			if _, err := fmt.Fprintf(w, "%3d   %-14s  %s  %d/%d\n",
				t.Ranking, t.Title, profitText(rateStr, t.ChangeRate, enabled), t.RiseCompanyCount, t.TotalCount); err != nil {
				return err
			}
		}
		return nil
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
			_, err := fmt.Fprintln(w, "업종 데이터가 없습니다")
			return err
		}
		if _, err := fmt.Fprintf(w, "업종            종목수    1일      1개월     1년\n"); err != nil {
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
			_, err := fmt.Fprintln(w, "브리핑이 없습니다")
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
