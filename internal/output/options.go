package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteOptionExpiries renders the listed expirations for an underlying.
func WriteOptionExpiries(w io.Writer, format Format, e domain.OptionExpiries) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, e)
	case FormatCSV:
		var csvRows [][]string
		for _, x := range e.Expiries {
			csvRows = append(csvRows, []string{x.MaturityDate, x.MaturityDateTime, x.LiquidationDateTime, x.DisplayLiquidation})
		}
		return writeCSV(w, []string{"maturity_date", "maturity_datetime", "liquidation_datetime", "display_liquidation"}, csvRows)
	case FormatTable:
		if len(e.Expiries) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.optionExpiries.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.optionExpiries.header"), e.Symbol, len(e.Expiries)); err != nil {
			return err
		}
		for _, x := range e.Expiries {
			// DisplayLiquidation 은 서버 표시 문자열("거래 종료" 등)이다. 비면
			// 붙이지 않는다 — 빈 괄호는 값을 못 읽은 것처럼 보인다.
			note := x.DisplayLiquidation
			if x.CorporateActionName != "" {
				note = x.CorporateActionName
			}
			if note == "" {
				if _, err := fmt.Fprintf(w, i18n.T("output.optionExpiries.linePlain"), x.MaturityDate); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, i18n.T("output.optionExpiries.line"), x.MaturityDate, note); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteOptionChain renders one expiration's call/put chain.
func WriteOptionChain(w io.Writer, format Format, c domain.OptionChain) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, c)
	case FormatCSV:
		var csvRows [][]string
		for _, r := range c.Rows {
			csvRows = append(csvRows, []string{
				formatFloat(r.StrikePrice), r.CallGUID, strconv.Itoa(r.CallOpenInterest),
				r.PutGUID, strconv.Itoa(r.PutOpenInterest),
			})
		}
		return writeCSV(w, []string{"strike_price", "call_guid", "call_open_interest", "put_guid", "put_open_interest"}, csvRows)
	case FormatTable:
		if len(c.Rows) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.optionChain.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.optionChain.header"), c.Symbol, c.MaturityDate); err != nil {
			return err
		}
		headers := []string{"CALL OI", "STRIKE", "PUT OI"}
		var rows [][]string
		for _, r := range c.Rows {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.CallOpenInterest),
				formatFloat(r.StrikePrice),
				fmt.Sprintf("%d", r.PutOpenInterest),
			})
		}
		aligns := []Align{AlignRight, AlignRight, AlignRight}
		return renderTable(w, headers, rows, aligns...)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
