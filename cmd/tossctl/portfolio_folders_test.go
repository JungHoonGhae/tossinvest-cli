package main

import "testing"

func TestPortfolioFoldersCommandIsAccountScopedReadOnlyWTS(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"portfolio", "folders"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "folders" {
		t.Fatalf("resolved command = %q", cmd.Name())
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
	if cmd.Annotations["mutating"] != "" {
		t.Fatalf("read command marked mutating: %#v", cmd.Annotations)
	}
	if cmd.Flags().Lookup("account") == nil {
		t.Fatal("portfolio folders missing --account")
	}
}
