package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestHiddenHoldingsContract(t *testing.T) {
	t.Parallel()

	type observedRequest struct {
		method     string
		path       string
		accountKey string
		xsrf       string
		body       map[string]any
	}
	var observed []observedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/account/list" {
			_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"primary-test","displayName":"test"}],"primaryKey":"primary-test"}}`))
			return
		}
		entry := observedRequest{method: r.Method, path: r.URL.Path, accountKey: r.Header.Get("accountKey"), xsrf: r.Header.Get("X-XSRF-TOKEN")}
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			if len(data) > 0 {
				_ = json.Unmarshal(data, &entry.body)
			}
		}
		observed = append(observed, entry)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":{"hiddenStocks":[{"stockCode":"A005930","stockName":"삼성전자","type":"STOCK","tradableQuantity":2}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	c := New(Config{
		HTTPClient:  srv.Client(),
		APIBaseURL:  srv.URL,
		InfoBaseURL: srv.URL,
		CertBaseURL: srv.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "x"},
			Headers: map[string]string{"X-XSRF-TOKEN": "token"},
		},
	})

	holdings, err := c.ListHiddenHoldings(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if holdings.AccountKey != "primary-test" || len(holdings.Holdings) != 1 || holdings.Holdings[0].ProductCode != "A005930" || holdings.Holdings[0].TradableQuantity != 2 {
		t.Fatalf("unexpected holdings: %#v", holdings)
	}
	if err := c.HideHolding(context.Background(), "primary-test", "A005930"); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if err := c.ShowHolding(context.Background(), "primary-test", "A005930"); err != nil {
		t.Fatalf("show: %v", err)
	}

	if len(observed) != 3 {
		t.Fatalf("requests = %d, want 3: %#v", len(observed), observed)
	}
	for _, got := range observed {
		if got.accountKey != "primary-test" {
			t.Fatalf("%s %s accountKey = %q", got.method, got.path, got.accountKey)
		}
	}
	if got := observed[0]; got.method != http.MethodGet || got.path != "/api/v2/hidden-stocks" {
		t.Fatalf("list route = %s %s", got.method, got.path)
	}
	if got := observed[1]; got.method != http.MethodPost || got.path != "/api/v1/my-assets/hidden-stocks/hide" || got.body["stockCode"] != "A005930" {
		t.Fatalf("hide request = %#v", got)
	}
	if got := observed[2]; got.method != http.MethodPost || got.path != "/api/v1/my-assets/hidden-stocks/show" || got.body["stockCode"] != "A005930" {
		t.Fatalf("show request = %#v", got)
	}
	for _, got := range observed[1:] {
		if got.xsrf != "token" {
			t.Fatalf("%s %s X-XSRF-TOKEN = %q", got.method, got.path, got.xsrf)
		}
	}
}
