package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteOverseasTransferIncome renders the overseas transfer-income (양도소득)
// report. JSON/CSV are language-invariant; the table view shows the tax
// summary plus per-stock profit/loss lines.
func WriteOverseasTransferIncome(w io.Writer, format Format, t domain.OverseasTransferIncome) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, t)
	case FormatCSV:
		writer := csv.NewWriter(w)
		header := []string{"symbol", "name", "sell_quantity", "sell_amount", "buy_amount", "expense", "profit_loss", "settlement_date", "settled"}
		if err := writer.Write(header); err != nil {
			return err
		}
		for _, s := range t.Stocks {
			row := []string{
				s.Symbol, s.Name,
				strconv.FormatFloat(s.SellQuantity, 'f', -1, 64),
				strconv.FormatFloat(s.SellAmount, 'f', -1, 64),
				strconv.FormatFloat(s.BuyAmount, 'f', -1, 64),
				strconv.FormatFloat(s.Expense, 'f', -1, 64),
				strconv.FormatFloat(s.ProfitLoss, 'f', -1, 64),
				s.SettlementDate,
				strconv.FormatBool(s.Settled),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default: // table
		if _, err := fmt.Fprintf(w, "%d년 해외주식 양도소득  ·  세율 %.0f%% (+지방세 %.0f%%)  ·  기본공제 %.0f원\n",
			t.Year, t.TaxRate, t.LocalTaxRate, t.BaseDeduction); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "총 양도손익 %.0f원  ·  양도소득세 %.0f원  ·  총 세액 %.0f원\n\n",
			t.TotalProfitLoss, t.TransferIncomeTax, t.TotalTax); err != nil {
			return err
		}
		if len(t.Stocks) == 0 {
			_, err := fmt.Fprintln(w, "(매도 종목 없음)")
			return err
		}
		for _, s := range t.Stocks {
			if _, err := fmt.Fprintf(w, "  %-6s %-18s 손익 %14.0f원  (매도 %.0f · 매수 %.0f)  %s\n",
				s.Symbol, s.Name, s.ProfitLoss, s.SellAmount, s.BuyAmount, s.SettlementDate); err != nil {
				return err
			}
		}
		return nil
	}
}

// WriteRIAReport renders the RIA (해외주식 양도세 절세 계좌) tax-saving
// projection. Amounts are KRW.
func WriteRIAReport(w io.Writer, format Format, r domain.RIAReport) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, r)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"metric", "value"}); err != nil {
			return err
		}
		rows := [][2]string{
			{"estimated_transfer_income_tax", formatFloat(r.EstimatedTransferIncomeTax)},
			{"estimated_tax_saving", formatFloat(r.EstimatedTaxSaving)},
			{"final_transfer_income_tax", formatFloat(r.FinalTransferIncomeTax)},
			{"total_transfer_income_amount", formatFloat(r.TotalTransferIncomeAmount)},
			{"normal_account_transfer_income", formatFloat(r.NormalAccountTransferIncome)},
			{"ria_account_transfer_income", formatFloat(r.RIAAccountTransferIncome)},
			{"base_deduction", formatFloat(r.BaseDeduction)},
			{"ria_deduction_total", formatFloat(r.Deduction.TotalAmount)},
			{"profit_after_deduction", formatFloat(r.ProfitAfterDeduction)},
			{"local_tax", formatFloat(r.LocalTax)},
		}
		if r.MaxTaxSaving != nil {
			rows = append(rows, [2]string{"max_tax_saving", formatFloat(*r.MaxTaxSaving)})
		}
		if r.ZeroReasonCode != "" {
			rows = append(rows, [2]string{"zero_reason_code", r.ZeroReasonCode})
		}
		if r.Limit != nil {
			rows = append(rows,
				[2]string{"limit_total", formatFloat(r.Limit.TotalLimit)},
				[2]string{"limit_remaining", formatFloat(r.Limit.RemainingLimit)},
			)
		}
		for _, row := range rows {
			if err := writer.Write(row[:]); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		// 절세액이 헤드라인이다 — RIA 공제를 넣기 전/후 세액을 나란히 둔다.
		if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.header"),
			formatFloat(r.EstimatedTaxSaving)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.taxLine"),
			formatFloat(r.EstimatedTransferIncomeTax), formatFloat(r.FinalTransferIncomeTax)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.incomeLine"),
			formatFloat(r.TotalTransferIncomeAmount),
			formatFloat(r.NormalAccountTransferIncome), formatFloat(r.RIAAccountTransferIncome)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.deductionLine"),
			formatFloat(r.BaseDeduction), formatFloat(r.Deduction.TotalAmount),
			formatFloat(r.Deduction.DeductionRate)); err != nil {
			return err
		}
		// 분기 가중치는 RIA 공제 계산의 핵심이라 합계만 보면 왜 그 금액인지 알 수 없다.
		for _, q := range r.Deduction.QuarterlyProfitLoss {
			if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.quarterLine"),
				q.Quarter, formatFloat(q.TotalProfitLoss),
				formatFloat(q.Weight), formatFloat(q.WeightedProfitLoss)); err != nil {
				return err
			}
		}
		if r.Limit != nil {
			if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.limitLine"),
				formatFloat(r.Limit.RemainingLimit), formatFloat(r.Limit.TotalLimit)); err != nil {
				return err
			}
		}
		if r.MaxTaxSaving != nil {
			if *r.MaxTaxSaving > 0 {
				if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.maxSaving"), formatFloat(*r.MaxTaxSaving)); err != nil {
					return err
				}
			} else if r.ZeroReasonCode != "" {
				// 서버 원문 코드를 그대로 낸다 — 토스가 웹에 매핑을 안 실어서
				// 번역하면 추측이 된다.
				if _, err := fmt.Fprintf(w, i18n.T("output.riaReport.noSaving"), r.ZeroReasonCode); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
