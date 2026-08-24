package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func f64(v float64) *float64 { return &v }

func dummyProfit() domain.ProfitOverview {
	return domain.ProfitOverview{
		TotalAssetAmount: domain.DualCurrency{KRW: 1000000, USD: f64(700.5)},
		EarningAmount:    domain.DualCurrency{KRW: 50000, USD: f64(35)},
		Sales:            domain.ProfitByType{Amount: domain.DualCurrency{KRW: -1234, USD: f64(-0.9)}, EarningRate: domain.DualCurrency{KRW: -3.2}},
		Dividend:         domain.ProfitByType{Amount: domain.DualCurrency{KRW: 9999, USD: f64(7.1)}},
		Interest:         536,
	}
}

func TestWriteProfitOverviewJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteProfitOverview(&buf, FormatJSON, dummyProfit()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), `"interest": 536`) {
		t.Errorf("missing interest in JSON: %s", buf.String())
	}
}

func TestWriteProfitOverviewTable(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	i18n.SetLang("ko")
	var koBuf bytes.Buffer
	if err := WriteProfitOverview(&koBuf, FormatTable, dummyProfit()); err != nil {
		t.Fatalf("err (ko): %v", err)
	}
	koOut := koBuf.String()
	for _, want := range []string{"매매손익", "배당", "예탁금이자", "총 자산"} {
		if !strings.Contains(koOut, want) {
			t.Errorf("table (ko) missing %q:\n%s", want, koOut)
		}
	}

	i18n.SetLang("en")
	var enBuf bytes.Buffer
	if err := WriteProfitOverview(&enBuf, FormatTable, dummyProfit()); err != nil {
		t.Fatalf("err (en): %v", err)
	}
	enOut := enBuf.String()
	for _, want := range []string{"Trading", "Dividend", "Deposit Interest", "Total assets"} {
		if !strings.Contains(enOut, want) {
			t.Errorf("table (en) missing %q:\n%s", want, enOut)
		}
	}
}
