package main

import (
	"strings"
	"testing"
)

func TestAssetSnapshotCommandsAreReadOnlySecuritiesWTS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path      []string
		wantFlags []string
	}{
		{path: []string{"portfolio", "performance"}, wantFlags: []string{"account"}},
		{path: []string{"portfolio", "snapshots"}, wantFlags: []string{"account", "cursor", "limit"}},
		{path: []string{"portfolio", "snapshot"}, wantFlags: []string{"account"}},
	}
	for _, tc := range tests {
		cmd, _, err := newRootCmd().Find(tc.path)
		if err != nil {
			t.Fatalf("%v: %v", tc.path, err)
		}
		if cmd.Name() != tc.path[len(tc.path)-1] {
			t.Fatalf("%v resolved to %q", tc.path, cmd.Name())
		}
		if cmd.Annotations["source"] != "wts" || cmd.Annotations["domain"] != "securities" || cmd.Annotations["mutating"] != "" {
			t.Fatalf("%v annotations = %#v", tc.path, cmd.Annotations)
		}
		for _, flag := range tc.wantFlags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%v missing --%s", tc.path, flag)
			}
		}
	}
}

func TestAssetSnapshotCommandsRejectInvalidInputsBeforeAuthentication(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "negative snapshot limit",
			args:    []string{"--config-dir", t.TempDir(), "portfolio", "snapshots", "--limit", "-1"},
			wantErr: "--limit must be zero or greater",
		},
		{
			name:    "malformed snapshot date",
			args:    []string{"--config-dir", t.TempDir(), "portfolio", "snapshot", "2026-9-3"},
			wantErr: "date must be YYYY-MM-DD",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Execute() error = %v, want substring %q", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), "auth") || strings.Contains(err.Error(), "session") {
				t.Fatalf("input validation happened after authentication: %v", err)
			}
		})
	}
}
