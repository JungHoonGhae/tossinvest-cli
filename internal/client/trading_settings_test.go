package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetTradingSettingsUsesVerifiedReadContracts(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/account/list" {
			t.Fatalf("unexpected API-host request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"primary-test"}],"primaryKey":"primary-test"}}`))
	}))
	t.Cleanup(api.Close)

	cert := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		wantAccountKey := ""
		switch r.URL.Path {
		case "/api/v1/trading/settings/simple-trade":
			wantAccountKey = "primary-test"
			_, _ = w.Write([]byte(`{"result":false}`))
		case "/api/v2/trading/settings/investor-exchange-choice-type":
			_, _ = w.Write([]byte(`{"result":"integrated"}`))
		case "/api/v1/users/settings/me/ats-notification":
			_, _ = w.Write([]byte(`{"result":true}`))
		case "/api/v1/member-subscriptions/get-option-real-time-tick":
			_, _ = w.Write([]byte(`{"result":{"requested":true,"serviced":false,"shouldCharged":true}}`))
		default:
			t.Fatalf("unexpected cert-host request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("accountKey"); got != wantAccountKey {
			t.Fatalf("%s accountKey = %q, want %q", r.URL.Path, got, wantAccountKey)
		}
	}))
	t.Cleanup(cert.Close)

	c := New(Config{
		HTTPClient:  api.Client(),
		APIBaseURL:  api.URL,
		InfoBaseURL: "http://info.invalid",
		CertBaseURL: cert.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "synthetic"}},
	})
	got, err := c.GetTradingSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.SimpleTradeEnabled || got.InvestorExchangeChoice != "integrated" || !got.ATSNotificationEnabled {
		t.Fatalf("settings = %#v", got)
	}
	if !got.OptionRealTimeTick.Requested || got.OptionRealTimeTick.Serviced || !got.OptionRealTimeTick.ShouldCharged {
		t.Fatalf("option real-time tick = %#v", got.OptionRealTimeTick)
	}
}
