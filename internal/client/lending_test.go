package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetLendingExpected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/lending/revenue/account/expected" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		// dummy data — never real account values
		_, _ = w.Write([]byte(`{"result":{"expectedAmountUsdOneMonth":1.23,"expectedAmountUsdOneYear":14.76,"items":[{"guid":"AMX0000000001","stockName":"DUMMY","amount":1.23}]}}`))
	}))
	defer srv.Close()

	c := New(Config{
		CertBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	got, err := c.GetLendingExpected(context.Background())
	if err != nil {
		t.Fatalf("GetLendingExpected: %v", err)
	}
	if got.OneMonthUSD != 1.23 || got.OneYearUSD != 14.76 {
		t.Errorf("totals: got 1M=%v 1Y=%v", got.OneMonthUSD, got.OneYearUSD)
	}
	if len(got.Stocks) != 1 || got.Stocks[0].ProductCode != "AMX0000000001" || got.Stocks[0].Name != "DUMMY" || got.Stocks[0].AmountUSD != 1.23 {
		t.Errorf("stocks: %+v", got.Stocks)
	}
}

func TestGetLendingExpectedNoSession(t *testing.T) {
	c := New(Config{})
	if _, err := c.GetLendingExpected(context.Background()); err == nil {
		t.Fatal("want error without a session")
	}
}

func TestGetTopLendingRevenuePreservesServerRanking(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/lending/revenue/account/top-revenue" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{"items":[` +
			`{"userName":"user-a","revenue":12.34,"revenueKrw":16900},` +
			`{"userName":"user-b","revenue":9.87,"revenueKrw":13500}` +
			`]}}`))
	}))
	defer srv.Close()

	c := New(Config{CertBaseURL: srv.URL, Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	got, err := c.GetTopLendingRevenue(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTopLendingRevenue: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Rank != 1 || got.Items[0].UserName != "user-a" || got.Items[0].Revenue != 12.34 || got.Items[0].RevenueKRW != 16900 {
		t.Fatalf("ranking = %#v", got)
	}
}
