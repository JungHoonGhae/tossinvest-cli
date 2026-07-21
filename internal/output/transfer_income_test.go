package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func dummyTransferIncome() domain.OverseasTransferIncome {
	return domain.OverseasTransferIncome{
		Year: 2025, TaxRate: 20, LocalTaxRate: 10, BaseDeduction: 2500000,
		TotalProfitLoss: -100, TotalTax: 0,
		Stocks: []domain.TransferIncomeStock{
			{Symbol: "DUMA", Name: "Dummy A", SellQuantity: 10, SellAmount: 1000, BuyAmount: 1200, ProfitLoss: -205, SettlementDate: "2025-05-01", Settled: true},
			{Symbol: "DUMB", Name: "Dummy B", SellQuantity: 3, SellAmount: 900, BuyAmount: 800, ProfitLoss: 98, SettlementDate: "2025-06-01", Settled: true},
		},
	}
}

func TestWriteTransferIncomeJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOverseasTransferIncome(&buf, FormatJSON, dummyTransferIncome()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), `"tax_rate": 20`) {
		t.Errorf("missing tax_rate in JSON: %s", buf.String())
	}
}

func TestWriteTransferIncomeCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOverseasTransferIncome(&buf, FormatCSV, dummyTransferIncome()); err != nil {
		t.Fatalf("err: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2
		t.Fatalf("want 3 CSV lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "symbol,name,sell_quantity") {
		t.Errorf("unexpected header: %q", lines[0])
	}
}

func TestWriteTransferIncomeTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	empty := domain.OverseasTransferIncome{Year: 2024, TaxRate: 20}
	if err := WriteOverseasTransferIncome(&buf, FormatTable, empty); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), "매도 종목 없음") {
		t.Errorf("expected empty-state, got %q", buf.String())
	}
}
