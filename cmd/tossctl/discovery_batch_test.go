package main

import (
	"strings"
	"testing"
)

func TestDiscoveryBatchCommandContracts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     []string
		wantName string
		wantFull bool
	}{
		{path: []string{"market", "key-events"}, wantName: "key-events"},
		{path: []string{"banking", "status"}, wantName: "status", wantFull: true},
		{path: []string{"notifications", "list"}, wantName: "list"},
	}
	for _, tc := range tests {
		cmd, _, err := newRootCmd().Find(tc.path)
		if err != nil {
			t.Fatalf("%v: %v", tc.path, err)
		}
		if cmd.Name() != tc.wantName {
			t.Fatalf("%v: command = %q", tc.path, cmd.Name())
		}
		if cmd.Annotations["source"] != "wts" || cmd.Annotations["mutating"] != "" {
			t.Fatalf("%v: annotations = %#v", tc.path, cmd.Annotations)
		}
		if tc.wantFull && cmd.Flags().Lookup("full") == nil {
			t.Fatalf("%v: --full missing", tc.path)
		}
	}
}

func TestMarketBriefingScopeContract(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"market", "briefing"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Flags().Lookup("scope") == nil {
		t.Fatal("market briefing --scope missing")
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}

func TestMarketBriefingRejectsInvalidScopeBeforeAuthentication(t *testing.T) {
	cmd := newMarketCmd(&rootOptions{})
	cmd.SetArgs([]string{"briefing", "--scope", "jp"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid --scope") {
		t.Fatalf("error = %v", err)
	}
}

func TestMarketSectorDetailCommandContract(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"market", "sector"})
	if err != nil || cmd.Name() != "sector" {
		t.Fatalf("market sector command missing: cmd=%q err=%v", cmd.Name(), err)
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" || cmd.Annotations["mutating"] != "" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}
