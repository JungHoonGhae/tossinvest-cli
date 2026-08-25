package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteStockReasoning renders Toss's AI explanation of a stock's move.
func WriteStockReasoning(w io.Writer, format Format, r domain.StockReasoning) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, r)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"symbol", "title", "summary", "direction", "keyword", "created_at"}); err != nil {
			return err
		}
		if err := writer.Write([]string{r.Symbol, r.Title, r.Summary, strconv.Itoa(r.Direction), r.Keyword, r.CreatedAt}); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if r.Summary == "" {
			_, err := fmt.Fprintln(w, i18n.T("output.reasoning.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.reasoning.header"), r.Symbol, r.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.reasoning.summary"), r.Summary); err != nil {
			return err
		}
		for _, s := range r.RelatedStock {
			// investmentTypeValue 는 서버 표시 문자열이다. 비어 있으면 줄을 만들지
			// 않는다 — 빈 괄호가 남으면 데이터가 있는데 못 읽은 것처럼 보인다.
			if s.InvestmentTypeValue == "" {
				if _, err := fmt.Fprintf(w, i18n.T("output.reasoning.relatedPlain"), s.Symbol, s.Name); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, i18n.T("output.reasoning.related"), s.Symbol, s.Name, s.InvestmentTypeValue); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteStockSignals renders the per-stock signal cards.
func WriteStockSignals(w io.Writer, format Format, s domain.StockSignals) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, s)
	case FormatCSV:
		var csvRows [][]string
		for _, sig := range s.Signals {
			csvRows = append(csvRows, []string{s.Symbol, sig.Label, sig.Info, strconv.FormatInt(sig.SignalID, 10), sig.DateTime})
		}
		return writeCSV(w, []string{"symbol", "label", "info", "signal_id", "datetime"}, csvRows)
	case FormatTable:
		if len(s.Signals) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.stockSignals.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.stockSignals.header"), s.Symbol); err != nil {
			return err
		}
		for _, sig := range s.Signals {
			// Label 은 서버 원문(호재/악재 등)이라 그대로 낸다.
			if _, err := fmt.Fprintf(w, i18n.T("output.stockSignals.line"), sig.Label, sig.Info, sig.DateTime); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteMarginNotice renders the receivable / forced-liquidation warning state.
func WriteMarginNotice(w io.Writer, format Format, n domain.MarginNotice) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, n)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"currency", "notice_type", "receivable_amount", "deadline_at", "forced_liquidated_at"}); err != nil {
			return err
		}
		if err := writer.Write([]string{n.Currency, n.NoticeType, formatFloat(n.ReceivableAmount), deref(n.DeadlineAt), deref(n.ForcedLiquidatedAt)}); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.marginNotice.header"),
			n.Currency, formatFloat(n.ReceivableAmount), n.NoticeType); err != nil {
			return err
		}
		// nil 인 날짜는 줄 자체를 만들지 않는다. 빈 값을 찍으면 기한이 있는데
		// 못 읽은 것처럼 보인다 — 정상 계좌는 전부 nil 이다.
		if n.DeadlineAt != nil {
			if _, err := fmt.Fprintf(w, i18n.T("output.marginNotice.deadline"), *n.DeadlineAt); err != nil {
				return err
			}
		}
		if n.ForcedLiquidatedAt != nil {
			if _, err := fmt.Fprintf(w, i18n.T("output.marginNotice.liquidated"), *n.ForcedLiquidatedAt); err != nil {
				return err
			}
		}
		if n.SuspensionStart != nil && n.SuspensionEnd != nil {
			if _, err := fmt.Fprintf(w, i18n.T("output.marginNotice.suspension"), *n.SuspensionStart, *n.SuspensionEnd); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSearchResults renders unified search hits.
func WriteSearchResults(w io.Writer, format Format, s domain.SearchResults) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, s)
	case FormatCSV:
		var csvRows [][]string
		for _, h := range s.Results {
			csvRows = append(csvRows, []string{h.Keyword, h.ProductCode, h.Symbol, h.CompanyName, h.Market})
		}
		return writeCSV(w, []string{"keyword", "product_code", "symbol", "company_name", "market"}, csvRows)
	case FormatTable:
		if len(s.Results) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.search.empty"))
			return err
		}
		headers := []string{
			i18n.T("output.search.header.symbol"),
			i18n.T("output.search.header.name"),
			i18n.T("output.search.header.market"),
			i18n.T("output.search.header.code"),
		}
		var rows [][]string
		for _, h := range s.Results {
			rows = append(rows, []string{h.Symbol, h.Keyword, h.Market, h.ProductCode})
		}
		aligns := []Align{AlignLeft, AlignLeft, AlignLeft, AlignLeft}
		return renderTable(w, headers, rows, aligns...)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
