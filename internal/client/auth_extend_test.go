package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestRequestExtensionReturnsDocID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/wts-login-extend/doc/request" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("X-XSRF-TOKEN"); got != "xsrf-test" {
			t.Fatalf("missing XSRF header: %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"txId":"abc-123"}}`))
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "s"},
			Headers: map[string]string{"X-XSRF-TOKEN": "xsrf-test"},
		},
	})

	uuid, err := c.RequestExtension(context.Background())
	if err != nil {
		t.Fatalf("RequestExtension: %v", err)
	}
	if uuid != "abc-123" {
		t.Fatalf("uuid = %q, want abc-123", uuid)
	}
}

func TestGetExtensionStatusReportsApproval(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/wts-login-extend/doc/abc-123/status") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		calls++
		if calls < 2 {
			_, _ = w.Write([]byte(`{"result":"REQUESTED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"COMPLETED"}`))
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	st1, err := c.GetExtensionStatus(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("GetExtensionStatus #1: %v", err)
	}
	if st1.Approved() || st1.Rejected() {
		t.Fatalf("expected pending, got %+v", st1)
	}

	st2, err := c.GetExtensionStatus(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("GetExtensionStatus #2: %v", err)
	}
	if !st2.Approved() {
		t.Fatalf("expected approved, got %+v", st2)
	}
}

func TestGetServerExpiredAtParsesKST(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/session/expired-at" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "2026-05-13T07:03:20.24+09:00"})
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	got, err := c.GetServerExpiredAt(context.Background())
	if err != nil {
		t.Fatalf("GetServerExpiredAt: %v", err)
	}
	want := time.Date(2026, 5, 13, 7, 3, 20, 240_000_000, time.FixedZone("KST", 9*3600))
	if !got.Equal(want) {
		t.Fatalf("expiry = %s, want %s", got, want)
	}
}

func TestRequestExtensionWithoutSessionReturnsAuthError(t *testing.T) {
	t.Parallel()

	c := New(Config{})
	_, err := c.RequestExtension(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestFinalizeExtensionPostsState(t *testing.T) {
	t.Parallel()

	var captured struct {
		path   string
		method string
		ctype  string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.method = r.Method
		captured.ctype = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	if err := c.FinalizeExtension(context.Background(), "abc-123"); err != nil {
		t.Fatalf("FinalizeExtension: %v", err)
	}
	if captured.path != "/api/v1/wts-login-extend/abc-123/state" {
		t.Fatalf("path = %s", captured.path)
	}
	if captured.method != http.MethodPost {
		t.Fatalf("method = %s", captured.method)
	}
	if captured.ctype != "application/json" {
		t.Fatalf("content-type = %s", captured.ctype)
	}
}

func TestFinalizeExtensionWithoutSessionReturnsAuthError(t *testing.T) {
	t.Parallel()

	c := New(Config{})
	if err := c.FinalizeExtension(context.Background(), "abc-123"); !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestExtensionUUIDValidation(t *testing.T) {
	t.Parallel()

	c := New(Config{
		HTTPClient: nil,
		APIBaseURL: "http://127.0.0.1",
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	for _, uuid := range []string{"", "   "} {
		if _, err := c.GetExtensionStatus(context.Background(), uuid); err == nil {
			t.Fatalf("GetExtensionStatus(%q): expected error, got nil", uuid)
		}
		if err := c.FinalizeExtension(context.Background(), uuid); err == nil {
			t.Fatalf("FinalizeExtension(%q): expected error, got nil", uuid)
		}
	}
}

func TestGetExtensionStatusEscapesUUID(t *testing.T) {
	t.Parallel()

	var gotPath, gotEscaped string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotEscaped = r.URL.Path
		_, _ = w.Write([]byte(`{"result":"REQUESTED"}`))
	}))
	defer server.Close()

	c := New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	if _, err := c.GetExtensionStatus(context.Background(), "a/b c"); err != nil {
		t.Fatalf("GetExtensionStatus: %v", err)
	}
	if gotPath != "/api/v1/wts-login-extend/doc/a%2Fb%20c/status" {
		t.Fatalf("escaped path = %q / %q", gotPath, gotEscaped)
	}
}
