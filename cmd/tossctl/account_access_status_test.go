package main

import "testing"

func TestAccountAccessStatusCommandIsReadOnlySecuritiesWTS(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"account", "access-status"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "access-status" {
		t.Fatalf("resolved command = %q", cmd.Name())
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" || cmd.Annotations["mutating"] != "" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
	if cmd.Flags().Lookup("account") == nil {
		t.Fatal("access-status missing --account")
	}
}
