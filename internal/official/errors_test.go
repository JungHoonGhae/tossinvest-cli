package official

import (
	"errors"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	if !errors.Is(classifyStatus(403, []byte(`{"message":"ip not allowed"}`)), ErrIPNotAllowed) {
		t.Fatal("403 ip")
	}
	if !errors.Is(classifyStatus(401, nil), ErrAuth) {
		t.Fatal("401")
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
