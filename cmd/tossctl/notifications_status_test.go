package main

import "testing"

func TestNotificationsStatusCommandIsReadOnlySecuritiesWTS(t *testing.T) {
	t.Parallel()
	cmd, _, err := newRootCmd().Find([]string{"notifications", "status"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "status" {
		t.Fatalf("resolved command = %q", cmd.Name())
	}
	if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" || cmd.Annotations["mutating"] != "" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}
