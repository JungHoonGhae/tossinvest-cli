package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
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
