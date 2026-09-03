package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
)

func TestPaperCommandTaxonomyAndSafetyAnnotations(t *testing.T) {
	root := newRootCmd()
	reads := [][]string{{"paper", "status"}, {"paper", "orders", "pending"}, {"paper", "orders", "completed"}}
	for _, path := range reads {
		cmd, _, err := root.Find(path)
		if err != nil || cmd.Annotations["source"] != "wts" || cmd.Annotations["environment"] != "paper" || cmd.Annotations["writes_state"] == "true" {
			t.Fatalf("read %v: cmd=%v err=%v annotations=%#v", path, cmd, err, cmd.Annotations)
		}
	}
	writes := [][]string{
		{"paper", "init"}, {"paper", "deposit"}, {"paper", "order", "place"},
		{"paper", "order", "cancel"}, {"paper", "orders", "cancel-all"},
	}
	for _, path := range writes {
		cmd, _, err := root.Find(path)
		if err != nil || cmd.Annotations["source"] != "wts" || cmd.Annotations["environment"] != "paper" ||
			cmd.Annotations["writes_state"] != "true" || cmd.Annotations["mutation_risk"] != "simulation" || cmd.Annotations["mutating"] == "true" {
			t.Fatalf("write %v: cmd=%v err=%v annotations=%#v", path, cmd, err, cmd.Annotations)
		}
	}
}

func TestPaperCommandIsHiddenAndBlockedUntilOptedIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	hidden := newRootCmd()
	var hiddenOut bytes.Buffer
	hidden.SetOut(&hiddenOut)
	hidden.SetErr(&hiddenOut)
	hidden.SetArgs([]string{"--config-dir", dir, "--help"})
	if err := hidden.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hiddenOut.String(), "\n  paper ") {
		t.Fatalf("paper command leaked into opted-out help:\n%s", hiddenOut.String())
	}

	directHelp := newRootCmd()
	var directHelpOut bytes.Buffer
	directHelp.SetOut(&directHelpOut)
	directHelp.SetErr(&directHelpOut)
	directHelp.SetArgs([]string{"--config-dir", dir, "paper", "--help"})
	if err := directHelp.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(directHelpOut.String(), "config experimental paper-trading --enable") || strings.Contains(directHelpOut.String(), "Available Commands") {
		t.Fatalf("disabled direct help exposed the experiment:\n%s", directHelpOut.String())
	}

	blocked := newRootCmd()
	blocked.SetOut(&bytes.Buffer{})
	blocked.SetErr(&bytes.Buffer{})
	blocked.SetArgs([]string{"--config-dir", dir, "paper", "status"})
	if err := blocked.Execute(); err == nil || !strings.Contains(err.Error(), "config experimental paper-trading --enable") {
		t.Fatalf("blocked error = %v", err)
	}
}

func TestPaperCommandAppearsAfterExplicitOptIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	service := config.NewService(filepath.Join(dir, "config.json"))
	if err := service.SetExperimentalPaperTrading(context.Background(), true); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--config-dir", dir, "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\n  paper ") {
		t.Fatalf("enabled paper command missing from help:\n%s", out.String())
	}
}

func TestPaperLivePreviewIsAnExplicitNonMutatingTransition(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"paper", "order", "live-preview"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Annotations["source"] != "local" || cmd.Annotations["environment"] != "live" || cmd.Annotations["transition_from"] != "paper" || cmd.Annotations["writes_state"] == "true" || cmd.Annotations["mutating"] == "true" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}
