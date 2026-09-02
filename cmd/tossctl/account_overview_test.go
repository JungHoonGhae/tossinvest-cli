package main

import "testing"

func TestAccountOverviewCommandContract(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"account", "overview"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "overview" {
		t.Fatalf("command = %q", cmd.Name())
	}
	if got := cmd.Annotations["source"]; got != "wts" {
		t.Fatalf("source = %q, want wts", got)
	}
	if cmd.Flags().Lookup("full") == nil {
		t.Fatal("--full flag missing")
	}
	if cmd.Annotations["mutating"] != "" {
		t.Fatal("read-only overview must not be marked mutating")
	}
}
