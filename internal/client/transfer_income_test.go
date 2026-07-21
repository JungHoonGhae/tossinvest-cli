package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetOverseasTransferIncomePaging(t *testing.T) {
	// Two pages: page 1 (lastPage=false, 2 items) + page 2 (lastPage=true, 1 item).
	// All dummy data — never real account values.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/my-assets/transfer-income/overseas") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("year") != "2025" {
			t.Errorf("want year=2025, got %q", r.URL.Query().Get("year"))
		}
		num := r.URL.Query().Get("number")
		summary := `"transferIncomeSummary":{"transferIncomeTaxRate":20,"localIncomeTaxRate":10,"baseDeduction":2500000,"totalProfitLossAmount":-100,"transferIncomeTax":0,"localIncomeTax":0,"totalTax":0}`
		if num == "1" {
			fmt.Fprintf(w, `{"result":{"pagingParam":{"number":2,"size":10},"body":{%s,"items":[
				{"stockCode":"A000001","stockSymbol":"DUMA","stockName":"Dummy A","sellQuantity":10,"sellAmount":1000,"buyAmount":1200,"expense":5,"profitLossAmount":-205,"finalSettlementKorDate":"2025-05-01","settled":true},
				{"stockCode":"A000002","stockSymbol":"DUMB","stockName":"Dummy B","sellQuantity":3,"sellAmount":900,"buyAmount":800,"expense":2,"profitLossAmount":98,"finalSettlementKorDate":"2025-06-01","settled":true}
			]},"lastPage":false}}`, summary)
		} else {
			fmt.Fprintf(w, `{"result":{"pagingParam":{"number":3,"size":10},"body":{%s,"items":[
				{"stockCode":"A000003","stockSymbol":"DUMC","stockName":"Dummy C","sellQuantity":1,"sellAmount":500,"buyAmount":450,"expense":1,"profitLossAmount":49,"finalSettlementKorDate":"2025-07-01","settled":false}
			]},"lastPage":true}}`, summary)
		}
	}))
	defer srv.Close()

	c := New(Config{
		APIBaseURL: srv.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	got, err := c.GetOverseasTransferIncome(context.Background(), 2025)
	if err != nil {
		t.Fatalf("GetOverseasTransferIncome: %v", err)
	}
	if got.Year != 2025 || got.TaxRate != 20 || got.BaseDeduction != 2500000 {
		t.Errorf("summary: %+v", got)
	}
	if len(got.Stocks) != 3 {
		t.Fatalf("want 3 stocks across 2 pages, got %d", len(got.Stocks))
	}
	if got.Stocks[0].Symbol != "DUMA" || got.Stocks[2].Symbol != "DUMC" {
		t.Errorf("stock order/paging wrong: %+v", got.Stocks)
	}
	if got.Stocks[2].Settled {
		t.Errorf("last stock should be unsettled")
	}
}

func TestGetOverseasTransferIncomeNoSession(t *testing.T) {
	c := New(Config{})
	if _, err := c.GetOverseasTransferIncome(context.Background(), 2025); err == nil {
		t.Fatal("want error without a session")
	}
}
