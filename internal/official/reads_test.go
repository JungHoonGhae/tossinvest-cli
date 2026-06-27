package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptAccountsUnit is a pure-function unit test of adaptAccounts.
// It verifies the official→domain field mapping without any HTTP I/O.
func TestAdaptAccountsUnit(t *testing.T) {
	raw := []apiAccount{
		{AccountNo: "123-45", AccountSeq: 7, AccountType: "BROKERAGE"},
	}
	got := adaptAccounts(raw)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	// accountSeq (int) → ID (string): the stable key used in subsequent API calls
	if got[0].ID != "7" {
		t.Fatalf("ID: want %q, got %q", "7", got[0].ID)
	}
	// accountNo (human number) → DisplayName
	if got[0].DisplayName != "123-45" {
		t.Fatalf("DisplayName: want %q, got %q", "123-45", got[0].DisplayName)
	}
	// accountType → Type (raw enum value)
	if got[0].Type != "BROKERAGE" {
		t.Fatalf("Type: want %q, got %q", "BROKERAGE", got[0].Type)
	}
	// Fields not available from official API must be zero/empty
	if got[0].Name != "" {
		t.Fatalf("Name: expected empty, got %q", got[0].Name)
	}
	if len(got[0].Markets) != 0 {
		t.Fatalf("Markets: expected nil, got %v", got[0].Markets)
	}
	if got[0].Primary {
		t.Fatal("Primary: expected false (not available in official API)")
	}
	if got[0].Currency != "" {
		t.Fatalf("Currency: expected empty, got %q", got[0].Currency)
	}
}

// TestAdaptAccountsMultiple verifies mapping of multiple accounts and int→string conversion.
func TestAdaptAccountsMultiple(t *testing.T) {
	raw := []apiAccount{
		{AccountNo: "111-11", AccountSeq: 1, AccountType: "BROKERAGE"},
		{AccountNo: "222-22", AccountSeq: 99, AccountType: "ISA"},
	}
	got := adaptAccounts(raw)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].ID != "1" || got[0].DisplayName != "111-11" || got[0].Type != "BROKERAGE" {
		t.Fatalf("first account: %+v", got[0])
	}
	if got[1].ID != "99" || got[1].DisplayName != "222-22" || got[1].Type != "ISA" {
		t.Fatalf("second account: %+v", got[1])
	}
}

// TestAdaptAccountsEmpty verifies that an empty slice returns an empty (non-nil) slice.
func TestAdaptAccountsEmpty(t *testing.T) {
	got := adaptAccounts(nil)
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// TestAccountsIntegration tests Accounts() against an httptest server that
// serves the token endpoint and the accounts endpoint with a dummy envelope.
func TestAccountsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/accounts":
			if r.Header.Get("Authorization") != "Bearer AT" {
				t.Errorf("Authorization = %q, want Bearer AT", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"result":[{"accountNo":"123-45","accountSeq":7,"accountType":"BROKERAGE"}]}`))
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

	got, err := c.Accounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 account, got %d", len(got))
	}
	if got[0].ID != "7" {
		t.Fatalf("ID: want %q, got %q", "7", got[0].ID)
	}
	if got[0].DisplayName != "123-45" {
		t.Fatalf("DisplayName: want %q, got %q", "123-45", got[0].DisplayName)
	}
	if got[0].Type != "BROKERAGE" {
		t.Fatalf("Type: want %q, got %q", "BROKERAGE", got[0].Type)
	}
}

// TestAccountsIntegrationEmpty verifies that an empty result list is handled cleanly.
func TestAccountsIntegrationEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/accounts":
			_, _ = w.Write([]byte(`{"result":[]}`))
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

	got, err := c.Accounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 accounts, got %d", len(got))
	}
}
