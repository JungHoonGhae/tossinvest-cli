package client

import (
	"context"
	"strings"
	"testing"
)

func TestInvestModeFor(t *testing.T) {
	cases := []struct {
		code             string
		wantView, wantIM string
	}{
		{"A005930", "krx_all", "krx"},           // KR stock
		{"A114800", "krx_all", "krx"},           // KR ETF
		{"US20220809012", "unified", "unified"}, // US stock code
		{"AAPL", "unified", "unified"},          // US ticker
	}
	for _, c := range cases {
		gv, gi := investModeFor(c.code)
		if gv != c.wantView || gi != c.wantIM {
			t.Errorf("investModeFor(%q) = (%q,%q), want (%q,%q)", c.code, gv, gi, c.wantView, c.wantIM)
		}
	}
}

// US symbols have no daily price band — GetPriceLimits must reject them with a
// clear message before any network call (a US product code looks-like a code so
// resolveProductCode returns it directly without hitting search).
func TestGetPriceLimitsRejectsUSSymbol(t *testing.T) {
	c := New(Config{})
	_, err := c.GetPriceLimits(context.Background(), "US19801212001")
	if err == nil {
		t.Fatal("expected error for US symbol, got nil")
	}
	if !strings.Contains(err.Error(), "KRX") {
		t.Errorf("expected KRX-only message, got: %v", err)
	}
}

func TestRunScreenerRawRejectsInvalidJSON(t *testing.T) {
	c := New(Config{})
	_, err := c.RunScreenerRaw(context.Background(), "not json", "kr", 5)
	if err == nil {
		t.Fatal("expected error for invalid JSON filter")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("expected JSON validation message, got: %v", err)
	}
}

func TestGetTradingFlowsRejectsUSSymbol(t *testing.T) {
	c := New(Config{})
	_, err := c.GetTradingFlows(context.Background(), "US19801212001", 5)
	if err == nil {
		t.Fatal("expected error for US symbol, got nil")
	}
	if !strings.Contains(err.Error(), "KRX") {
		t.Errorf("expected KRX-only message, got: %v", err)
	}
}
