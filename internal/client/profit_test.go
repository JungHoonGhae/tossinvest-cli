package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetProfitOverview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profit/overview" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		// dummy data — never real account values
		_, _ = w.Write([]byte(`{"result":{
			"totalAssetAmount":{"krw":1000000,"usd":700.5},
			"earningAmount":{"krw":50000,"usd":35.0},
			"sales":{"amount":{"krw":-1234,"usd":-0.9},"earningRate":{"krw":-3.2,"usd":-1.1},"purchaseAmount":{"krw":40000,"usd":28.0}},
			"lending":{"amount":{"krw":0,"usd":0},"earningRate":{"krw":0,"usd":0},"purchaseAmount":null},
			"dividend":{"amount":{"krw":9999,"usd":7.1},"earningRate":{"krw":0,"usd":0},"purchaseAmount":null},
			"maturity":{"amount":{"krw":0,"usd":null},"earningRate":{"krw":0,"usd":null},"purchaseAmount":null},
			"interest":536}}`))
	}))
	defer srv.Close()

	c := New(Config{
		CertBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	got, err := c.GetProfitOverview(context.Background())
	if err != nil {
		t.Fatalf("GetProfitOverview: %v", err)
	}
	if got.TotalAssetAmount.KRW != 1000000 || got.TotalAssetAmount.USD == nil || *got.TotalAssetAmount.USD != 700.5 {
		t.Errorf("total asset: %+v", got.TotalAssetAmount)
	}
	if got.Sales.Amount.KRW != -1234 || got.Sales.EarningRate.KRW != -3.2 {
		t.Errorf("sales: %+v", got.Sales)
	}
	if got.Dividend.Amount.KRW != 9999 {
		t.Errorf("dividend: %+v", got.Dividend)
	}
	// maturity USD is null → pointer must be nil
	if got.Maturity.Amount.USD != nil {
		t.Errorf("maturity USD should be nil, got %v", *got.Maturity.Amount.USD)
	}
	if got.Interest != 536 {
		t.Errorf("interest: %v", got.Interest)
	}
}

func TestGetProfitOverviewNoSession(t *testing.T) {
	c := New(Config{})
	if _, err := c.GetProfitOverview(context.Background()); err == nil {
		t.Fatal("want error without a session")
	}
}
