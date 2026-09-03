package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetSecuritiesTransferAccountsUsesVerifiedReadContracts(t *testing.T) {
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
		if got := r.Header.Get("accountKey"); got != "primary-test" {
			t.Fatalf("%s accountKey = %q, want primary-test", r.URL.Path, got)
		}
		switch r.URL.Path {
		case "/api/v1/securities-transfer/my-accounts":
			_, _ = w.Write([]byte(`{"result":[{"bankCode":"092","accountNo":"123-456-789","accountId":"own-1"}]}`))
		case "/api/v1/securities-transfer/recent-accounts":
			_, _ = w.Write([]byte(`{"result":[{"bankCode":"088","accountNo":"987-654-321"}]}`))
		default:
			t.Fatalf("unexpected cert-host request: %s %s", r.Method, r.URL.Path)
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
	got, err := c.GetSecuritiesTransferAccounts(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.OwnAccounts) != 1 || got.OwnAccounts[0].BankCode != "092" || got.OwnAccounts[0].AccountID != "own-1" || got.OwnAccounts[0].AccountNo != "123-456-789" {
		t.Fatalf("own accounts = %#v", got.OwnAccounts)
	}
	if len(got.RecentAccounts) != 1 || got.RecentAccounts[0].BankCode != "088" || got.RecentAccounts[0].AccountNo != "987-654-321" {
		t.Fatalf("recent accounts = %#v", got.RecentAccounts)
	}
	if got.AccountScope == "" || got.AccountScope == "primary-test" {
		t.Fatalf("account scope is not opaque: %q", got.AccountScope)
	}
}

func TestGetSecuritiesTransferAccountsUsesExplicitAccountWithoutLookup(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/account/list" {
			t.Fatal("explicit account must not trigger primary-account lookup")
		}
		if r.Header.Get("accountKey") != "selected-test" {
			t.Fatalf("%s accountKey = %q", r.URL.Path, r.Header.Get("accountKey"))
		}
		switch r.URL.Path {
		case securitiesTransferMyAccountsPath, securitiesTransferRecentAccountsPath:
			_, _ = w.Write([]byte(`{"result":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	c := New(Config{HTTPClient: server.Client(), APIBaseURL: server.URL, CertBaseURL: server.URL, Session: &session.Session{Cookies: map[string]string{"SESSION": "synthetic"}}})
	got, err := c.GetSecuritiesTransferAccounts(context.Background(), " selected-test ")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountScope == "" || got.AccountScope == "selected-test" {
		t.Fatalf("account scope is not opaque: %q", got.AccountScope)
	}
}
