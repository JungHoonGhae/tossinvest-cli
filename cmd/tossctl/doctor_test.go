package main

import (
	"strings"
	"testing"
	"time"
)

// ── pure unit tests for doctorOpenAPISummary ───────────────────────────────

func TestDoctorOpenAPISummaryAbsent(t *testing.T) {
	t.Parallel()
	got := doctorOpenAPISummary(doctorOpenAPIInputs{credsPresent: false})
	if !strings.Contains(got, "not configured") {
		t.Fatalf("expected not-configured hint for absent creds, got %q", got)
	}
	if !strings.Contains(got, "tossctl openapi login") {
		t.Fatalf("expected login command in hint, got %q", got)
	}
	// Absent creds must NOT show error markers.
	if strings.Contains(got, "❌") {
		t.Fatalf("absent creds should not show error marker, got %q", got)
	}
}

func TestDoctorOpenAPISummaryPresentOK(t *testing.T) {
	t.Parallel()
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeOK:      true,
	})
	if !strings.Contains(got, "connection OK") {
		t.Fatalf("expected connection OK, got %q", got)
	}
	if !strings.Contains(got, "current IP allowed") {
		t.Fatalf("expected IP allowed text, got %q", got)
	}
	// No error markers for a successful probe.
	if strings.Contains(got, "❌") {
		t.Fatalf("OK probe should not show error marker, got %q", got)
	}
}

func TestDoctorOpenAPISummaryIPNotAllowed(t *testing.T) {
	t.Parallel()
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeOK:      false,
		probeErrKind: "ip_not_allowed",
	})
	if !strings.Contains(got, "IP not allowed") {
		t.Fatalf("expected IP not allowed message, got %q", got)
	}
	if !strings.Contains(got, "tossctl openapi status") {
		t.Fatalf("expected pointer to openapi status, got %q", got)
	}
	if !strings.Contains(got, "❌") {
		t.Fatalf("expected error marker for IP not allowed, got %q", got)
	}
}

func TestDoctorOpenAPISummaryAuthError(t *testing.T) {
	t.Parallel()
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeOK:      false,
		probeErrKind: "auth",
	})
	if !strings.Contains(got, "auth failed") {
		t.Fatalf("expected auth error message, got %q", got)
	}
	if !strings.Contains(got, "❌") {
		t.Fatalf("expected error marker for auth error, got %q", got)
	}
}

func TestDoctorOpenAPISummaryExpiringKey(t *testing.T) {
	t.Parallel()
	// Key expires in 5 days — within the 30-day warning threshold.
	expiresAt := time.Now().Add(5 * 24 * time.Hour)
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeOK:      true,
		hasKeyInfo:   true,
		keyActive:    true,
		expiresAt:    &expiresAt,
	})
	if !strings.Contains(got, "D-") {
		t.Fatalf("expected D-NN expiry notation in summary, got %q", got)
	}
	if !strings.Contains(got, "⚠") {
		t.Fatalf("expected warning symbol for expiring key, got %q", got)
	}
	if !strings.Contains(got, "expiring") {
		t.Fatalf("expected expiring in summary, got %q", got)
	}
}

func TestDoctorOpenAPISummaryNonExpiringKey(t *testing.T) {
	t.Parallel()
	// Key expires in 60 days — beyond the 30-day warning threshold.
	expiresAt := time.Now().Add(60 * 24 * time.Hour)
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeOK:      true,
		hasKeyInfo:   true,
		keyActive:    true,
		expiresAt:    &expiresAt,
	})
	if strings.Contains(got, "expiring soon") {
		t.Fatalf("should NOT warn for non-expiring key (60 days), got %q", got)
	}
	if !strings.Contains(got, "connection OK") {
		t.Fatalf("expected connection OK, got %q", got)
	}
}

func TestDoctorOpenAPISummaryProbeCallError(t *testing.T) {
	t.Parallel()
	// If the probe call itself errors (e.g. timeout), doctor must not fail —
	// the summary should still be non-empty and point to openapi status.
	got := doctorOpenAPISummary(doctorOpenAPIInputs{
		credsPresent: true,
		probeCallErr: "context deadline exceeded",
	})
	if got == "" {
		t.Fatal("expected non-empty summary even when probe call errors")
	}
	if !strings.Contains(got, "tossctl openapi status") {
		t.Fatalf("expected pointer to openapi status on probe error, got %q", got)
	}
}

// ── graceful: summary is always non-empty and starts with "Open API: " ────

func TestDoctorOpenAPISummaryNeverEmpty(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(10 * 24 * time.Hour)

	cases := []doctorOpenAPIInputs{
		{credsPresent: false},
		{credsPresent: true, probeOK: true},
		{credsPresent: true, probeOK: false, probeErrKind: "ip_not_allowed"},
		{credsPresent: true, probeOK: false, probeErrKind: "auth"},
		{credsPresent: true, probeOK: false, probeErrKind: "rate_limited"},
		{credsPresent: true, probeOK: false, probeErrKind: "server_error"},
		{credsPresent: true, probeOK: false, probeErrKind: "transport_error"},
		{credsPresent: true, probeOK: false, probeErrKind: "unknown"},
		{credsPresent: true, probeCallErr: "timeout"},
		{credsPresent: true, probeOK: true, hasKeyInfo: true, keyActive: true},
		{credsPresent: true, probeOK: true, hasKeyInfo: true, keyActive: false},
		{credsPresent: true, probeOK: true, hasKeyInfo: true, keyActive: true, expiresAt: &expiresAt},
	}

	for _, in := range cases {
		got := doctorOpenAPISummary(in)
		if got == "" {
			t.Errorf("empty summary for input %+v", in)
		}
		if !strings.HasPrefix(got, "Open API: ") {
			t.Errorf("summary should start with 'Open API: ', got %q for input %+v", got, in)
		}
	}
}

// ── absent creds: hint line must not mark doctor as failed ─────────────────

func TestDoctorOpenAPISummaryAbsentIsHintNotError(t *testing.T) {
	t.Parallel()
	// Verifies that the "no credentials" path returns an informational hint,
	// not an error. Doctor exit status must not be affected by this line.
	got := doctorOpenAPISummary(doctorOpenAPIInputs{credsPresent: false})

	// Must not contain error indicators.
	for _, bad := range []string{"❌", "FAIL", "error"} {
		if strings.Contains(got, bad) {
			t.Fatalf("absent-creds hint must not contain %q, got %q", bad, got)
		}
	}
	// Must be a gentle hint toward the setup command.
	if !strings.Contains(got, "openapi login") {
		t.Fatalf("expected openapi login hint, got %q", got)
	}
}
