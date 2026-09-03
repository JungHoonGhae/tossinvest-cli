package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccountAccessStatusMapsVerifiedAccountSignals(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/api/v1/user/last-login-info":
			if got := r.Header.Get("accountKey"); got != "" {
				t.Fatalf("last-login accountKey = %q, want empty", got)
			}
			_, _ = w.Write([]byte(`{"result":{"channel":"W","osName":"macOS","agentName":"Chrome","timestamp":"2026-09-03T12:34:56+09:00"}}`))
		case "/api/v1/margin/cert/frozen-account":
			if got := r.Header.Get("accountKey"); got != "account-secret" {
				t.Fatalf("frozen accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"isFrozen":true,"startDate":"2026-09-01","endDate":"2026-09-30"}}`))
		case "/api/v2/account/unlock/accident-account/count":
			if got := r.Header.Get("accountKey"); got != "account-secret" {
				t.Fatalf("accident count accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAccountAccessStatus(context.Background(), " account-secret ")
	if err != nil {
		t.Fatalf("GetAccountAccessStatus: %v", err)
	}
	if got.AccountScope == "" || got.AccountScope == "account-secret" {
		t.Fatalf("account scope is not opaque: %q", got.AccountScope)
	}
	if got.LastLogin.Channel != "W" || got.LastLogin.OSName != "macOS" || got.LastLogin.AgentName != "Chrome" || got.LastLogin.Timestamp == "" {
		t.Fatalf("last login = %#v", got.LastLogin)
	}
	if !got.Margin.Frozen || got.Margin.StartDate != "2026-09-01" || got.Margin.EndDate != "2026-09-30" {
		t.Fatalf("margin status = %#v", got.Margin)
	}
	if got.AccidentAccountCount != 2 || got.FetchedAt.IsZero() {
		t.Fatalf("access status = %#v", got)
	}
}

func TestGetAccountAccessStatusResolvesPrimaryAccount(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/list":
			_, _ = w.Write([]byte(`{"result":{"primaryKey":"primary-secret","accountList":[{"key":"primary-secret"}]}}`))
		case "/api/v1/user/last-login-info":
			_, _ = w.Write([]byte(`{"result":{"channel":"M","osName":"iOS","agentName":"Toss","timestamp":"2026-09-03T00:00:00Z"}}`))
		case "/api/v1/margin/cert/frozen-account":
			if got := r.Header.Get("accountKey"); got != "primary-secret" {
				t.Fatalf("frozen accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"isFrozen":false,"startDate":null,"endDate":null}}`))
		case "/api/v2/account/unlock/accident-account/count":
			if got := r.Header.Get("accountKey"); got != "primary-secret" {
				t.Fatalf("accident count accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAccountAccessStatus(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountScope == "" || got.Margin.Frozen || got.Margin.StartDate != "" || got.Margin.EndDate != "" {
		t.Fatalf("status = %#v", got)
	}
}
