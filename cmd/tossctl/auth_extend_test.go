package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

func TestAuthLoginExposesLinkFlag(t *testing.T) {
	t.Parallel()

	cmd := newAuthCmd(&rootOptions{})
	loginCmd, _, err := cmd.Find([]string{"login"})
	if err != nil {
		t.Fatalf("find auth login: %v", err)
	}
	flag := loginCmd.Flags().Lookup("link")
	if flag == nil {
		t.Fatal("auth login must expose --link")
	}
	if flag.DefValue != "false" {
		t.Fatalf("--link must be opt-in, got default %q", flag.DefValue)
	}
}

func TestWriteAuthStatusIncludesServerExpiryInKST(t *testing.T) {
	t.Parallel()

	parsed, err := time.Parse(time.RFC3339Nano, "2026-05-12T10:47:37.24+09:00")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	retrieved := time.Date(2026, 4, 28, 22, 4, 7, 0, time.UTC)
	status := auth.Status{
		Active:          true,
		Provider:        "playwright-storage-state",
		SessionFile:     "/tmp/session.json",
		RetrievedAt:     &retrieved,
		ServerExpiresAt: &parsed,
	}

	var buf bytes.Buffer
	if err := writeAuthStatus(&buf, output.FormatTable, status); err != nil {
		t.Fatalf("writeAuthStatus: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, "Server Expiry: 2026-05-12 10:47 KST") {
		t.Fatalf("expected KST server expiry, got %q", got)
	}
	persistenceIdx := strings.Index(got, "Persistence:")
	serverIdx := strings.Index(got, "Server Expiry:")
	liveIdx := strings.Index(got, "Live Check:")
	if persistenceIdx < 0 || serverIdx < 0 || liveIdx < 0 {
		t.Fatalf("missing expected lines: %q", got)
	}
	if !(persistenceIdx < serverIdx && serverIdx < liveIdx) {
		t.Fatalf("expected Persistence < Server Expiry < Live Check, got %d / %d / %d", persistenceIdx, serverIdx, liveIdx)
	}
}

func TestWriteAuthStatusOmitsServerExpiryWhenNil(t *testing.T) {
	t.Parallel()

	retrieved := time.Date(2026, 4, 28, 22, 4, 7, 0, time.UTC)
	status := auth.Status{
		Active:      true,
		Provider:    "playwright-storage-state",
		SessionFile: "/tmp/session.json",
		RetrievedAt: &retrieved,
	}

	var buf bytes.Buffer
	if err := writeAuthStatus(&buf, output.FormatTable, status); err != nil {
		t.Fatalf("writeAuthStatus: %v", err)
	}
	if strings.Contains(buf.String(), "Server Expiry") {
		t.Fatalf("expected no server expiry line, got %q", buf.String())
	}
}

func TestRunAuthExtendSucceeds(t *testing.T) {
	t.Parallel()

	expiry := time.Date(2026, 5, 13, 7, 3, 20, 0, time.FixedZone("KST", 9*3600))
	result := &auth.ExtendResult{
		UUID:            "abc",
		ServerExpiresAt: expiry,
		Elapsed:         3 * time.Second,
	}

	var out bytes.Buffer
	if err := writeExtendResult(&out, result); err != nil {
		t.Fatalf("writeExtendResult: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Extension complete") {
		t.Fatalf("expected success line, got %q", got)
	}
	if !strings.Contains(got, "2026-05-13 07:03 KST") {
		t.Fatalf("expected formatted KST expiry, got %q", got)
	}
}

func TestRunAuthExtendTimeoutMessage(t *testing.T) {
	t.Parallel()

	got := userFacingCommandError(auth.ErrExtensionTimeout).Error()
	if !strings.Contains(got, "rerun") {
		t.Fatalf("expected retry hint, got %q", got)
	}
	if !strings.Contains(got, "tossctl auth extend") {
		t.Fatalf("expected extend command hint, got %q", got)
	}
}

func TestRunAuthExtendRejectionMessage(t *testing.T) {
	t.Parallel()

	got := userFacingCommandError(auth.ErrExtensionRejected).Error()
	if !strings.Contains(got, "denied") && !strings.Contains(got, "canceled") {
		t.Fatalf("expected rejection wording, got %q", got)
	}
}

func TestWriteExtendResultRendersKSTForRFC3339ParsedInput(t *testing.T) {
	t.Parallel()

	// Simulate the production path: server returns RFC3339Nano with numeric
	// offset, parsed via time.Parse — the resulting Location is an anonymous
	// FixedZone with empty name. writeExtendResult must still display "KST".
	parsed, err := time.Parse(time.RFC3339Nano, "2026-05-13T07:03:20.24+09:00")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result := &auth.ExtendResult{
		UUID:            "abc",
		ServerExpiresAt: parsed,
		Elapsed:         3 * time.Second,
	}

	var out bytes.Buffer
	if err := writeExtendResult(&out, result); err != nil {
		t.Fatalf("writeExtendResult: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "2026-05-13 07:03 KST") {
		t.Fatalf("expected KST label even for RFC3339-parsed input, got %q", got)
	}
}

// Sanity check: spinner runs without crashing when stdout is not a TTY.
func TestSpinnerSilentInNonTTY(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	stop := startSpinner(&out, "test", false)
	time.Sleep(20 * time.Millisecond)
	stop()
	if out.Len() != 0 {
		t.Fatalf("expected no output in non-TTY mode, got %q", out.String())
	}
}
