package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestGetAccountOverviewUsesVerifiedAndroidContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/dashboard/all-accounts" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), `{"sections":["SUMMARY_WITH_MINOR"]}`; got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"result":[{"data":{"accountOverviews":[{"accountName":"일반계좌","accountNo":"1000000001","pendingOrderCount":2,"totalAssetAmount":1234000}],"minorAccountOverviews":[{"accountName":"미성년계좌","accountNo":"2000000002","pendingOrderCount":0,"totalAssetAmount":5000}],"totalAssetAmount":1239000}}]}`)
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  "http://api.invalid",
		InfoBaseURL: server.URL,
		CertBaseURL: "http://cert.invalid",
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "synthetic"},
		},
	})

	got, err := c.GetAccountOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalAssetAmount != 1239000 || len(got.Accounts) != 1 || len(got.MinorAccounts) != 1 {
		t.Fatalf("unexpected overview: %#v", got)
	}
	if got.Accounts[0].AccountNo != "1000000001" || got.Accounts[0].PendingOrderCount != 2 {
		t.Fatalf("unexpected regular account: %#v", got.Accounts[0])
	}
}

func TestGetAccountOverviewRequiresSession(t *testing.T) {
	t.Parallel()

	_, err := New(Config{}).GetAccountOverview(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestGetAccountOverviewRejectsMissingResult(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"result":[]}`)
	}))
	defer server.Close()
	c := New(Config{
		HTTPClient:  server.Client(),
		InfoBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "synthetic"}},
	})

	if _, err := c.GetAccountOverview(context.Background()); err == nil {
		t.Fatal("expected empty result to fail")
	}
}

func TestGetAccountOverviewRejectsNullData(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"result":[{"data":null}]}`)
	}))
	t.Cleanup(server.Close)
	c := New(Config{
		HTTPClient:  server.Client(),
		InfoBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "synthetic"}},
	})
	if _, err := c.GetAccountOverview(context.Background()); err == nil || !strings.Contains(err.Error(), "data is missing") {
		t.Fatalf("null data error = %v", err)
	}
}
