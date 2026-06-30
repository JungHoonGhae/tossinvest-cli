package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

// TestApplySessionSetsBrowserUserAgent verifies that every request sent
// through the client carries a browser-like User-Agent instead of Go's
// default "Go-http-client/1.1", which the Toss Securities API blocks (403).
func TestApplySessionSetsBrowserUserAgent(t *testing.T) {
	t.Parallel()

	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"accountList":[]}}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
		},
	})

	_, _, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts returned error: %v", err)
	}

	if capturedUA == "" {
		t.Fatal("expected non-empty User-Agent header")
	}
	if capturedUA == "Go-http-client/1.1" {
		t.Fatal("User-Agent was not overridden from Go default")
	}
	if !strings.Contains(capturedUA, "Mozilla/5.0") {
		t.Fatalf("expected browser-like User-Agent, got: %s", capturedUA)
	}
}
