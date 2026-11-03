package official

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClassifyStatus(t *testing.T) {
	if !errors.Is(classifyStatus(403, []byte(`{"message":"ip not allowed"}`)), ErrIPNotAllowed) {
		t.Fatal("403 ip")
	}
	if !errors.Is(classifyStatus(401, nil), ErrAuth) {
		t.Fatal("401")
	}
	if !errors.Is(classifyStatus(401, []byte(`{"message":"ip not allowed"}`)), ErrIPNotAllowed) {
		t.Fatal("401 ip")
	}
	// Negative: "ip" appearing only inside another word must NOT trigger ErrIPNotAllowed.
	if !errors.Is(classifyStatus(403, []byte(`{"message":"invalid client description"}`)), ErrAuth) {
		t.Fatal("403 with 'ip' inside 'description' must be ErrAuth, not ErrIPNotAllowed")
	}
	if !errors.Is(classifyStatus(401, []byte(`{"error":"forbidden"}`)), ErrAuth) {
		t.Fatal("401 plain forbidden must be ErrAuth")
	}
	if !errors.Is(classifyStatus(429, nil), ErrRateLimited) {
		t.Fatal("429")
	}
	if !errors.Is(classifyStatus(503, nil), ErrServer) {
		t.Fatal("5xx")
	}
	if ShouldFallback(classifyStatus(404, nil)) {
		t.Fatal("404 must not fallback")
	}
}

func TestShouldFallback(t *testing.T) {
	if !ShouldFallback(ErrTransport) {
		t.Fatal("ErrTransport should fallback")
	}
	if !ShouldFallback(ErrAuth) {
		t.Fatal("ErrAuth should fallback")
	}
	if !ShouldFallback(ErrIPNotAllowed) {
		t.Fatal("ErrIPNotAllowed should fallback")
	}
	if !ShouldFallback(ErrRateLimited) {
		t.Fatal("ErrRateLimited should fallback")
	}
	if !ShouldFallback(ErrServer) {
		t.Fatal("ErrServer should fallback")
	}
	// APIError must not fallback
	if ShouldFallback(&APIError{Code: 400, Body: "bad request"}) {
		t.Fatal("APIError must not fallback")
	}
}

// An ambiguous account is refused before any request, so reads may fall back
// to the web session instead of failing outright for multi-account users.
func TestShouldFallbackOnAmbiguousAccount(t *testing.T) {
	err := fmt.Errorf("%w: 2 accounts found (1, 2)", ErrAccountAmbiguous)
	if !ShouldFallback(err) {
		t.Fatal("ambiguous account must allow read fallback")
	}
}

// A hung official server surfaces as an http.Client.Timeout error, which
// errors.Is-matches context.DeadlineExceeded once wrapped with %w. That is an
// availability failure and must still fall back to WTS; only caller
// cancellation must not.
func TestShouldFallbackOnClientTimeoutButNotCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := &Client{hc: &http.Client{Timeout: 20 * time.Millisecond}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	_, _, err := c.doRequest(req)
	if err == nil || !ShouldFallback(err) {
		t.Fatalf("client timeout must fall back, got %v", err)
	}
	if ShouldFallback(fmt.Errorf("%w: %w", ErrTransport, context.Canceled)) {
		t.Fatal("caller cancellation must not fall back")
	}
}
