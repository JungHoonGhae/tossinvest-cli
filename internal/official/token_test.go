package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestTokenColdLoadFromDiskCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("network exchange should not happen when disk cache is valid")
	}))
	defer srv.Close()

	cacheFile := filepath.Join(t.TempDir(), "t.json")
	stamper := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, cacheFile, srv.Client())
	ct := cachedToken{AccessToken: "DISK", ExpiresAt: time.Now().Add(time.Hour), KeyFingerprint: stamper.fingerprint()}
	data, _ := json.Marshal(ct)
	if err := os.WriteFile(cacheFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Cold manager: nothing in memory, must read from disk.
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, cacheFile, srv.Client())
	tok, err := m.token(context.Background())
	if err != nil || tok != "DISK" {
		t.Fatalf("got %q,%v", tok, err)
	}
}

// TestTokenRejectsForeignCache guards issue #10: a disk entry stamped for
// other credentials (rotated key, env/file switching) must not be reused —
// the manager exchanges fresh instead.
func TestTokenRejectsForeignCache(t *testing.T) {
	exchanges := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges++
		_, _ = w.Write([]byte(`{"access_token":"NEW","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	cacheFile := filepath.Join(t.TempDir(), "t.json")
	other := newTokenManager(Credentials{APIKey: "other", SecretKey: "s"}, srv.URL, cacheFile, srv.Client())
	ct := cachedToken{AccessToken: "OLD-KEY-TOKEN", ExpiresAt: time.Now().Add(time.Hour), KeyFingerprint: other.fingerprint()}
	data, _ := json.Marshal(ct)
	if err := os.WriteFile(cacheFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, cacheFile, srv.Client())
	tok, err := m.token(context.Background())
	if err != nil || tok != "NEW" {
		t.Fatalf("foreign cache must be discarded, got %q,%v", tok, err)
	}
	if exchanges != 1 {
		t.Fatalf("expected a fresh exchange, got %d", exchanges)
	}
	// The re-exchange must stamp the new credentials on disk.
	saved, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	var stamped cachedToken
	if err := json.Unmarshal(saved, &stamped); err != nil {
		t.Fatal(err)
	}
	if stamped.KeyFingerprint != m.fingerprint() {
		t.Fatalf("saved cache must carry the current fingerprint, got %+v", stamped)
	}
}

// TestTokenOldCacheFormatTriggersExchange: entries written before
// fingerprinting carry no fingerprint and never match — one re-exchange
// self-heals the format.
func TestTokenOldCacheFormatTriggersExchange(t *testing.T) {
	exchanges := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges++
		_, _ = w.Write([]byte(`{"access_token":"NEW","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	cacheFile := filepath.Join(t.TempDir(), "t.json")
	ct := cachedToken{AccessToken: "LEGACY", ExpiresAt: time.Now().Add(time.Hour)}
	data, _ := json.Marshal(ct)
	if err := os.WriteFile(cacheFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, cacheFile, srv.Client())
	tok, err := m.token(context.Background())
	if err != nil || tok != "NEW" || exchanges != 1 {
		t.Fatalf("legacy cache must trigger one exchange, got %q,%v (%d exchanges)", tok, err, exchanges)
	}
}

func TestTokenEmptyAccessTokenErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, filepath.Join(t.TempDir(), "t.json"), srv.Client())
	_, err := m.token(context.Background())
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
	if !ShouldFallback(err) {
		t.Fatalf("empty token should fallback (ErrServer), err=%v", err)
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
