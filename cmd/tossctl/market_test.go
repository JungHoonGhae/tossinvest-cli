package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// findMarketResearchCmd locates `market research` in the real command tree
// built by newRootCmd(), the same tree the convention tests walk.
func findMarketResearchCmd(t *testing.T) *cobra.Command {
	t.Helper()
	root := newRootCmd()
	for _, top := range root.Commands() {
		if top.Name() != "market" {
			continue
		}
		for _, sub := range top.Commands() {
			if sub.Name() == "research" {
				return sub
			}
		}
	}
	t.Fatal("market research command not found in command tree")
	return nil
}

func TestMarketResearchEffortFlagDefault(t *testing.T) {
	cmd := findMarketResearchCmd(t)
	f := cmd.Flags().Lookup("effort")
	if f == nil {
		t.Fatal("expected --effort flag on `market research`")
	}
	if f.DefValue != "lite" {
		t.Errorf("--effort default = %q, want lite", f.DefValue)
	}
}

func TestMarketResearchRequiresAtLeastOneArg(t *testing.T) {
	cmd := findMarketResearchCmd(t)
	if cmd.Args == nil {
		t.Fatal("expected an Args validator on `market research`")
	}
	if err := cmd.Args(cmd, nil); err == nil {
		t.Error("expected an error with zero args (query is required)")
	}
	if err := cmd.Args(cmd, []string{"삼성전자"}); err != nil {
		t.Errorf("expected one arg to be accepted, got error: %v", err)
	}
	if err := cmd.Args(cmd, []string{"삼성전자", "실적"}); err != nil {
		t.Errorf("expected multiple args to be accepted (joined as query), got error: %v", err)
	}
}

func TestMarketResearchSourceAnnotation(t *testing.T) {
	cmd := findMarketResearchCmd(t)
	if got := cmd.Annotations["source"]; got != "external" {
		t.Errorf("source annotation = %q, want external", got)
	}
}
