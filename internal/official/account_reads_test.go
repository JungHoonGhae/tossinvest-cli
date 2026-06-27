package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptBuyingPowerUnit verifies the pure adapter function for BuyingPower.
func TestAdaptBuyingPowerUnit(t *testing.T) {
	raw := apiBuyingPower{CashBuyingPower: "5000000", Currency: "KRW"}
	got := adaptBuyingPower(raw)
	if got.Currency != "KRW" {
		t.Fatalf("Currency: want KRW, got %q", got.Currency)
	}
	if got.CashBuyingPower != 5000000 {
		t.Fatalf("CashBuyingPower: want 5000000, got %v", got.CashBuyingPower)
	}
}

// TestAdaptBuyingPowerUSD verifies USD decimal parsing.
func TestAdaptBuyingPowerUSD(t *testing.T) {
	raw := apiBuyingPower{CashBuyingPower: "3500.5", Currency: "USD"}
	got := adaptBuyingPower(raw)
	if got.Currency != "USD" {
		t.Fatalf("Currency: want USD, got %q", got.Currency)
	}
	if got.CashBuyingPower != 3500.5 {
		t.Fatalf("CashBuyingPower: want 3500.5, got %v", got.CashBuyingPower)
	}
}

// TestAdaptBuyingPowerZero verifies zero/empty string handling.
func TestAdaptBuyingPowerZero(t *testing.T) {
	raw := apiBuyingPower{CashBuyingPower: "", Currency: "KRW"}
	got := adaptBuyingPower(raw)
	if got.CashBuyingPower != 0 {
		t.Fatalf("CashBuyingPower: want 0, got %v", got.CashBuyingPower)
	}
}

// TestBuyingPowerIntegration tests BuyingPower() against an httptest server.
func TestBuyingPowerIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/buying-power":
			if r.URL.Query().Get("currency") != "KRW" {
				t.Errorf("currency param: want KRW, got %q", r.URL.Query().Get("currency"))
			}
			_, _ = w.Write([]byte(`{"result":{"cashBuyingPower":"5000000","currency":"KRW"}}`))
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

	got, err := c.BuyingPower(context.Background(), "KRW")
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "KRW" {
		t.Fatalf("Currency: want KRW, got %q", got.Currency)
	}
	if got.CashBuyingPower != 5000000 {
		t.Fatalf("CashBuyingPower: want 5000000, got %v", got.CashBuyingPower)
	}
}

// TestBuyingPowerSendsAccountHeader verifies that WithAccountSeq injects the
// X-Tossinvest-Account header on account-scoped requests.
func TestBuyingPowerSendsAccountHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/buying-power":
			gotHeader = r.Header.Get("X-Tossinvest-Account")
			_, _ = w.Write([]byte(`{"result":{"cashBuyingPower":"100","currency":"KRW"}}`))
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
		WithAccountSeq(42),
	)

	if _, err := c.BuyingPower(context.Background(), "KRW"); err != nil {
		t.Fatal(err)
	}
	if gotHeader != "42" {
		t.Fatalf("X-Tossinvest-Account: want 42, got %q", gotHeader)
	}
}

// TestBuyingPowerOmitsAccountHeaderWhenUnset verifies the header is absent when
// no account seq is configured (accountSeq == 0).
func TestBuyingPowerOmitsAccountHeaderWhenUnset(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/buying-power":
			_, present = r.Header["X-Tossinvest-Account"]
			_, _ = w.Write([]byte(`{"result":{"cashBuyingPower":"100","currency":"KRW"}}`))
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

	if _, err := c.BuyingPower(context.Background(), "KRW"); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("X-Tossinvest-Account header should be absent when accountSeq unset")
	}
}
