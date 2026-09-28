package main

import (
	"strings"
	"testing"
)

func TestResearchContentCommandsRejectInvalidIDsBeforeAuthentication(t *testing.T) {
	for _, args := range [][]string{{"market", "earnings", "transcript", "0"}, {"market", "earnings", "report", "bad"}, {"watchlist", "news", "0"}} {
		root := newRootCmd()
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "positive integer") {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
func TestResearchContentCommandsAreReadOnly(t *testing.T) {
	for _, args := range [][]string{{"market", "earnings", "transcript"}, {"market", "earnings", "report"}, {"watchlist", "news"}} {
		cmd, _, err := newRootCmd().Find(args)
		if err != nil || cmd.Annotations["source"] != "wts" || cmd.Annotations["mutating"] != "" || cmd.Annotations["writes_state"] != "" {
			t.Fatalf("command %v: %v", args, err)
		}
	}
}
