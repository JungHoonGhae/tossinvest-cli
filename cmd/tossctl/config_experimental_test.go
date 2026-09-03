package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
)

func TestConfigExperimentalPaperTradingRequiresOneChoiceAndPersists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	missing := newRootCmd()
	missing.SetOut(&bytes.Buffer{})
	missing.SetErr(&bytes.Buffer{})
	missing.SetArgs([]string{"--config-dir", dir, "config", "experimental", "paper-trading"})
	if err := missing.Execute(); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("missing choice error = %v", err)
	}

	enable := newRootCmd()
	enable.SetOut(&bytes.Buffer{})
	enable.SetErr(&bytes.Buffer{})
	enable.SetArgs([]string{"--config-dir", dir, "--output", "json", "config", "experimental", "paper-trading", "--enable"})
	if err := enable.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.NewService(filepath.Join(dir, "config.json")).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Experimental.PaperTrading || cfg.SchemaVersion != config.SchemaVersion {
		t.Fatalf("config = %#v", cfg)
	}

	both := newRootCmd()
	both.SetOut(&bytes.Buffer{})
	both.SetErr(&bytes.Buffer{})
	both.SetArgs([]string{"--config-dir", dir, "config", "experimental", "paper-trading", "--enable", "--disable"})
	if err := both.Execute(); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("both choices error = %v", err)
	}
}
