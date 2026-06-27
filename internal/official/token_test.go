package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestTokenExchangeAndCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "k" {
			t.Fatal("bad form")
		}
		hits++
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, filepath.Join(t.TempDir(), "t.json"), srv.Client())
	tok, err := m.token(context.Background())
	if err != nil || tok != "AT" {
		t.Fatalf("got %q,%v", tok, err)
	}
	_, _ = m.token(context.Background()) // cache reuse
	if hits != 1 {
		t.Fatalf("expected 1 exchange, got %d", hits)
	}
}

func TestTokenRefresh(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"access_token":"AT2","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, filepath.Join(t.TempDir(), "t.json"), srv.Client())
	// First exchange
	_, _ = m.token(context.Background())
	// Force refresh
	tok, err := m.refresh(context.Background())
	if err != nil || tok != "AT2" {
		t.Fatalf("refresh got %q,%v", tok, err)
	}
	if hits != 2 {
		t.Fatalf("expected 2 exchanges, got %d", hits)
	}
}

func TestTokenExchangeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, filepath.Join(t.TempDir(), "t.json"), srv.Client())
	_, err := m.token(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !ShouldFallback(err) {
		t.Fatalf("401 should fallback, err=%v", err)
	}
}
