package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// ---------------------------------------------------------------------------
// resolveBackend tests
// ---------------------------------------------------------------------------

func TestResolveBackendEmptyFlagUsesConfig(t *testing.T) {
	t.Parallel()
	for _, prefer := range []string{"auto", "wts", "official"} {
		cfg := config.OpenAPI{Enabled: true, Prefer: prefer, Fallback: true}
		got, err := resolveBackend(cfg, "")
		if err != nil {
			t.Fatalf("prefer=%q: unexpected error: %v", prefer, err)
		}
		if got != prefer {
			t.Fatalf("prefer=%q: want %q, got %q", prefer, prefer, got)
		}
	}
}

func TestResolveBackendFlagOverridesConfig(t *testing.T) {
	t.Parallel()
	cfg := config.OpenAPI{Enabled: true, Prefer: "auto", Fallback: true}
	for _, flag := range []string{"auto", "wts", "official"} {
		got, err := resolveBackend(cfg, flag)
		if err != nil {
			t.Fatalf("flag=%q: unexpected error: %v", flag, err)
		}
		if got != flag {
			t.Fatalf("flag=%q: want %q, got %q", flag, flag, got)
		}
	}
}

func TestResolveBackendInvalidFlagReturnsError(t *testing.T) {
	t.Parallel()
	cfg := config.OpenAPI{Enabled: true, Prefer: "auto", Fallback: true}
	_, err := resolveBackend(cfg, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid --backend value, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --backend") {
		t.Fatalf("error should mention 'invalid --backend', got: %v", err)
	}
}

func TestResolveBackendWTSFlagOverridesOfficialConfig(t *testing.T) {
	t.Parallel()
	// Config prefers official, but --backend wts should override.
	cfg := config.OpenAPI{Enabled: true, Prefer: "official", Fallback: false}
	got, err := resolveBackend(cfg, "wts")
	if err != nil {
		t.Fatal(err)
	}
	if got != "wts" {
		t.Fatalf("want %q, got %q", "wts", got)
	}
}

func TestExpiryWarningWithin24Hours(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(18 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	var buf bytes.Buffer
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatTable, time.Now(), nil, nil)
	got := buf.String()
	if !strings.Contains(got, "session expires") {
		t.Fatalf("expected warning, got %q", got)
	}
	if !strings.Contains(got, "tossctl auth extend") {
		t.Fatalf("expected hint about auth extend, got %q", got)
	}
}

func TestExpiryWarningSilentBeyond24Hours(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(48 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	var buf bytes.Buffer
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatTable, time.Now(), nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected silence, got %q", buf.String())
	}
}

func TestExpiryWarningSilentInJSONMode(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(2 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	var buf bytes.Buffer
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatJSON, time.Now(), nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected silence in JSON mode, got %q", buf.String())
	}
}

func TestExpiryWarningSilentForExtendCommand(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(2 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	for _, name := range []string{"extend", "login", "logout", "status", "import-playwright-state", "version", "help"} {
		var buf bytes.Buffer
		writeExpiryWarningIfNeeded(&buf, sess, name, output.FormatTable, time.Now(), nil, nil)
		if buf.Len() != 0 {
			t.Fatalf("expected silence for %q, got %q", name, buf.String())
		}
	}
}

func TestExpiryWarningRespectsBackoffGate(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(2 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	// Gate denies → no output, mark must not run.
	var buf bytes.Buffer
	markCalled := false
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatTable, time.Now(),
		func() bool { return false },
		func() { markCalled = true },
	)
	if buf.Len() != 0 {
		t.Fatalf("expected silence when gate denies, got %q", buf.String())
	}
	if markCalled {
		t.Fatal("expected mark to be skipped when gate denies")
	}

	// Gate permits → output emitted, mark runs once.
	buf.Reset()
	markCount := 0
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatTable, time.Now(),
		func() bool { return true },
		func() { markCount++ },
	)
	if !strings.Contains(buf.String(), "session expires") {
		t.Fatalf("expected warning, got %q", buf.String())
	}
	if markCount != 1 {
		t.Fatalf("expected mark to be called once, got %d", markCount)
	}
}

func TestConfigLegacyWarningOnLegacyFields(t *testing.T) {
	t.Parallel()

	status := config.Status{
		Exists:       true,
		ConfigFile:   "/tmp/config.json",
		LegacyFields: []string{"trading.grant", "trading.allow_dangerous_execute"},
	}
	var buf bytes.Buffer
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable, nil, nil)
	got := buf.String()
	if !strings.Contains(got, "legacy field") {
		t.Fatalf("expected legacy-field warning, got %q", got)
	}
	if !strings.Contains(got, "trading.grant") {
		t.Fatalf("expected offending field listed, got %q", got)
	}
}

func TestConfigLegacyWarningOnStaleSchema(t *testing.T) {
	t.Parallel()

	status := config.Status{
		Exists:              true,
		ConfigFile:          "/tmp/config.json",
		SourceSchemaVersion: config.SchemaVersion - 1,
	}
	var buf bytes.Buffer
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable, nil, nil)
	if !strings.Contains(buf.String(), "schema is outdated") {
		t.Fatalf("expected stale-schema warning, got %q", buf.String())
	}
}

func TestConfigLegacyWarningSilentWhenClean(t *testing.T) {
	t.Parallel()

	status := config.Status{
		Exists:              true,
		ConfigFile:          "/tmp/config.json",
		SourceSchemaVersion: config.SchemaVersion,
	}
	var buf bytes.Buffer
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable, nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected silence for clean config, got %q", buf.String())
	}
}

func TestConfigLegacyWarningSilentInJSONAndSkipCommands(t *testing.T) {
	t.Parallel()

	status := config.Status{
		Exists:       true,
		ConfigFile:   "/tmp/config.json",
		LegacyFields: []string{"trading.grant"},
	}

	var buf bytes.Buffer
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatJSON, nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected silence in JSON mode, got %q", buf.String())
	}

	for _, name := range []string{"config", "doctor", "version", "help"} {
		buf.Reset()
		writeConfigLegacyWarningIfNeeded(&buf, status, name, output.FormatTable, nil, nil)
		if buf.Len() != 0 {
			t.Fatalf("expected silence for %q, got %q", name, buf.String())
		}
	}
}

func TestConfigLegacyWarningSilentWhenNoConfig(t *testing.T) {
	t.Parallel()

	status := config.Status{Exists: false, LegacyFields: []string{"trading.grant"}}
	var buf bytes.Buffer
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable, nil, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected silence when config absent, got %q", buf.String())
	}
}

func TestConfigLegacyWarningRespectsBackoffGate(t *testing.T) {
	t.Parallel()

	status := config.Status{
		Exists:       true,
		ConfigFile:   "/tmp/config.json",
		LegacyFields: []string{"trading.grant"},
	}

	var buf bytes.Buffer
	markCalled := false
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable,
		func() bool { return false },
		func() { markCalled = true },
	)
	if buf.Len() != 0 {
		t.Fatalf("expected silence when gate denies, got %q", buf.String())
	}
	if markCalled {
		t.Fatal("expected mark skipped when gate denies")
	}

	buf.Reset()
	markCount := 0
	writeConfigLegacyWarningIfNeeded(&buf, status, "portfolio", output.FormatTable,
		func() bool { return true },
		func() { markCount++ },
	)
	if !strings.Contains(buf.String(), "legacy field") {
		t.Fatalf("expected warning when gate permits, got %q", buf.String())
	}
	if markCount != 1 {
		t.Fatalf("expected mark called once, got %d", markCount)
	}
}

func TestExpiryWarningSilentWhenAlreadyExpired(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(-1 * time.Hour)
	sess := &session.Session{ServerExpiresAt: &exp}

	var buf bytes.Buffer
	writeExpiryWarningIfNeeded(&buf, sess, "portfolio", output.FormatTable, time.Now(), nil, nil)
	// Already expired — let the 401 path handle it; don't add noise.
	if buf.Len() != 0 {
		t.Fatalf("expected silence when already expired, got %q", buf.String())
	}
}
