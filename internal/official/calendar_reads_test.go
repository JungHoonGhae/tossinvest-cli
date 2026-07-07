package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestMarketCalendarIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/market-calendar/KR":
			if got := r.URL.Query().Get("date"); got != "2026-03-25" {
				t.Errorf("date query = %q, want 2026-03-25", got)
			}
			_, _ = w.Write([]byte(`{"result":{"today":{"date":"2026-03-25"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)

	got, err := c.MarketCalendar(context.Background(), "kr", "2026-03-25")
	if err != nil {
		t.Fatal(err)
	}
	today, ok := got["today"].(map[string]any)
	if !ok || today["date"] != "2026-03-25" {
		t.Fatalf("unexpected calendar payload: %v", got)
	}
}

func TestMarketCalendarRejectsBadCountry(t *testing.T) {
	c := New(Credentials{APIKey: "k"}, filepath.Join(t.TempDir(), "t.json"))
	if _, err := c.MarketCalendar(context.Background(), "JP", ""); err == nil {
		t.Fatal("expected error for unsupported country")
	}
}
