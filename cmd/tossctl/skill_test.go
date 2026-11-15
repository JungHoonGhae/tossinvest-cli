package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/skill"
)

func TestSkillInstallCommandWritesFiles(t *testing.T) {
	dir := t.TempDir()
	cmd := newSkillInstallCmd(&rootOptions{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dir", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, rel := range []string{"SKILL.md", filepath.Join("references", "operations.md")} {
		if _, err := os.Stat(filepath.Join(dir, "tossctl", rel)); err != nil {
			t.Fatalf("%s not installed: %v", rel, err)
		}
	}
	if !strings.Contains(out.String(), "Installed the tossctl skill") {
		t.Fatalf("missing confirmation line, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Restart your Claude client") {
		t.Fatalf("missing restart hint, got: %q", out.String())
	}
}

// The doctor nudge must never corrupt machine-readable output, must appear when
// the skill is absent, and must stay quiet once it is installed.

func TestSkillHintSuppressedForJSON(t *testing.T) {
	var buf bytes.Buffer
	writeSkillInstallHintIfNeeded(&buf, output.FormatJSON)
	if buf.Len() != 0 {
		t.Fatalf("hint leaked into non-table output: %q", buf.String())
	}
}

func TestSkillHintShownWhenNotInstalled(t *testing.T) {
	t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir()) // empty → not installed
	var buf bytes.Buffer
	writeSkillInstallHintIfNeeded(&buf, output.FormatTable)
	if !strings.Contains(buf.String(), "tossctl skill install") {
		t.Fatalf("expected install hint, got: %q", buf.String())
	}
}

func TestSkillHintSilentWhenInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_SKILLS_DIR", dir)
	if _, err := skill.Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	var buf bytes.Buffer
	writeSkillInstallHintIfNeeded(&buf, output.FormatTable)
	if buf.Len() != 0 {
		t.Fatalf("hint shown though skill is installed: %q", buf.String())
	}
}
