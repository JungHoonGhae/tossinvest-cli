package main

import "testing"

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
