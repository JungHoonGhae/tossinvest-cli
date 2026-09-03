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

func TestPriceAlertContract(t *testing.T) {
	t.Parallel()

	type observedRequest struct {
		method string
		path   string
		xsrf   string
		body   map[string]any
	}
	var observed []observedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := observedRequest{method: r.Method, path: r.URL.Path, xsrf: r.Header.Get("X-XSRF-TOKEN")}
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			if len(data) > 0 {
				_ = json.Unmarshal(data, &entry.body)
			}
		}
		observed = append(observed, entry)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":[{"targetPrice":70000,"currency":"KRW"}]}`))
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

	alerts, err := c.ListPriceAlerts(context.Background(), "A005930")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if alerts.ProductCode != "A005930" || len(alerts.Alerts) != 1 || alerts.Alerts[0].TargetPrice != 70000 || alerts.Alerts[0].Currency != "KRW" {
		t.Fatalf("unexpected alerts: %#v", alerts)
	}
	if err := c.AddPriceAlert(context.Background(), "A005930", 71000, "KRW"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := c.DeletePriceAlert(context.Background(), "A005930", 71000, "KRW"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(observed) != 3 {
		t.Fatalf("requests = %d, want 3: %#v", len(observed), observed)
	}
	if got := observed[0]; got.method != http.MethodGet || got.path != "/api/v1/user-price-alimy/A005930" {
		t.Fatalf("list route = %s %s", got.method, got.path)
	}
	if got := observed[1]; got.method != http.MethodPost || got.path != "/api/v1/user-price-alimy/A005930" || got.body["targetPrice"] != float64(71000) || got.body["currency"] != "KRW" {
		t.Fatalf("add request = %#v", got)
	}
	if got := observed[2]; got.method != http.MethodDelete || got.path != "/api/v1/user-price-alimy/A005930/KRW/71000" {
		t.Fatalf("delete route = %s %s", got.method, got.path)
	}
	for _, got := range observed[1:] {
		if got.xsrf != "token" {
			t.Fatalf("%s %s X-XSRF-TOKEN = %q", got.method, got.path, got.xsrf)
		}
	}
}
