package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestGetUnwrapsEnvelopeAndRetriesOn401(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/ping":
			calls++
			if calls == 1 {
				w.WriteHeader(401)
				return
			} // 첫 호출 401 → 갱신 후 재시도
			if r.Header.Get("Authorization") != "Bearer AT" {
				t.Fatalf("auth %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
		}
	}))
	defer srv.Close()
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.get(context.Background(), "/api/v1/ping", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Fatal("envelope not unwrapped")
	}
	if calls != 2 {
		t.Fatalf("expected retry, calls=%d", calls)
	}
}

func TestPostSendsJSONAndUnwrapsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/echo":
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if r.Header.Get("Authorization") != "Bearer AT" {
				t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"result":{"val":42}}`))
		}
	}))
	defer srv.Close()
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	var out struct {
		Val int `json:"val"`
	}
	if err := c.post(context.Background(), "/api/v1/echo", map[string]string{"x": "y"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Val != 42 {
		t.Fatalf("expected val=42, got %d", out.Val)
	}
}

func TestClientBaseURL(t *testing.T) {
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, "/tmp/cache.json", WithBaseURL("https://example.com"))
	if got := c.BaseURL(); got != "https://example.com" {
		t.Fatalf("BaseURL() = %q, want https://example.com", got)
	}
}

func TestClientDefaultBaseURL(t *testing.T) {
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, "/tmp/cache.json")
	if got := c.BaseURL(); got != "https://openapi.tossinvest.com" {
		t.Fatalf("BaseURL() = %q, want https://openapi.tossinvest.com", got)
	}
}

func TestGetNon2xxReturnsClassifiedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/fail":
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`server gone`))
		}
	}))
	defer srv.Close()
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	err := c.get(context.Background(), "/api/v1/fail", nil, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !ShouldFallback(err) {
		t.Fatalf("expected ShouldFallback=true for server error, got false; err=%v", err)
	}
}
