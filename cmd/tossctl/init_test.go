package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/onboarding"
)

// TestInitNonInteractivePrintsGuidance verifies that running `tossctl init`
// in a non-TTY environment (as in CI or AI agents) prints flag-based guidance
// and returns nil — it must never block waiting for input.
func TestInitNonInteractivePrintsGuidance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &rootOptions{configDir: dir}

	cmd := newInitCmd(opts)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// In test environments os.Stdin is not a character device → non-interactive.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error in non-interactive mode, got %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "tossctl auth login") {
		t.Errorf("expected web login guidance, got %q", out)
	}
	if !strings.Contains(out, "tossctl openapi login") {
		t.Errorf("expected openapi login guidance in output, got %q", out)
	}
	if !strings.Contains(out, "--key") || !strings.Contains(out, "--secret") {
		t.Errorf("expected --key and --secret flags mentioned, got %q", out)
	}
}

// TestShouldHintOnboarding is a pure table test over shouldHintOnboarding.
// It must return true ONLY when all three conditions hold:
//   - onboarding.NeedsOnboarding(state) is true
//   - interactive is true
//   - cmdName is not in the exclusion set {init, help, completion, version}
func TestShouldHintOnboarding(t *testing.T) {
	t.Parallel()

	needsOnboarding := onboarding.State{HasSession: false, HasOfficialCreds: false}
	hasSession := onboarding.State{HasSession: true, HasOfficialCreds: false}
	hasOfficial := onboarding.State{HasSession: false, HasOfficialCreds: true}
	hasBoth := onboarding.State{HasSession: true, HasOfficialCreds: true}

	cases := []struct {
		name        string
		state       onboarding.State
		interactive bool
		cmdName     string
		want        bool
	}{
		// ── should hint ────────────────────────────────────────────────────────
		{
			name:  "needs onboarding + interactive + eligible cmd (portfolio)",
			state: needsOnboarding, interactive: true, cmdName: "portfolio", want: true,
		},
		{
			name:  "needs onboarding + interactive + eligible cmd (account)",
			state: needsOnboarding, interactive: true, cmdName: "account", want: true,
		},
		{
			name:  "needs onboarding + interactive + eligible cmd (quote)",
			state: needsOnboarding, interactive: true, cmdName: "quote", want: true,
		},

		// ── excluded by cmdName ────────────────────────────────────────────────
		{
			name:  "needs onboarding + interactive + init",
			state: needsOnboarding, interactive: true, cmdName: "init", want: false,
		},
		{
			name:  "needs onboarding + interactive + help",
			state: needsOnboarding, interactive: true, cmdName: "help", want: false,
		},
		{
			name:  "needs onboarding + interactive + completion",
			state: needsOnboarding, interactive: true, cmdName: "completion", want: false,
		},
		{
			name:  "needs onboarding + interactive + version",
			state: needsOnboarding, interactive: true, cmdName: "version", want: false,
		},

		// ── excluded by non-interactive ────────────────────────────────────────
		{
			name:  "needs onboarding + non-interactive + eligible cmd",
			state: needsOnboarding, interactive: false, cmdName: "portfolio", want: false,
		},

		// ── excluded because no onboarding needed ─────────────────────────────
		{
			name:  "has session only — no hint",
			state: hasSession, interactive: true, cmdName: "portfolio", want: false,
		},
		{
			name:  "has official only — no hint",
			state: hasOfficial, interactive: true, cmdName: "portfolio", want: false,
		},
		{
			name:  "has both — no hint",
			state: hasBoth, interactive: true, cmdName: "portfolio", want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shouldHintOnboarding(tc.state, tc.interactive, tc.cmdName)
			if got != tc.want {
				t.Errorf("shouldHintOnboarding(%+v, interactive=%v, cmd=%q) = %v, want %v",
					tc.state, tc.interactive, tc.cmdName, got, tc.want)
			}
		})
	}
}
