package main

import (
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/spf13/cobra"
)

// TestMain pins the i18n locale to English before running this package's
// tests so TestShortDescriptionsAreEnglish sees English Short strings
// regardless of the host environment's TOSSCTL_LANG/LANG settings.
func TestMain(m *testing.M) {
	i18n.SetLang("en")
	os.Exit(m.Run())
}

// leafCommands walks the tree and returns commands that actually run (RunE/Run set).
func leafCommands(root *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.RunE != nil || c.Run != nil {
			out = append(out, c)
		}
		for _, ch := range c.Commands() {
			walk(ch)
		}
	}
	walk(root)
	return out
}

// allCommands walks the whole tree including grouping parents.
func allCommands(root *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		out = append(out, c)
		for _, ch := range c.Commands() {
			walk(ch)
		}
	}
	walk(root)
	return out
}

func hasHangul(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hangul) {
			return true
		}
	}
	return false
}

func TestShortDescriptionsAreEnglish(t *testing.T) {
	for _, c := range allCommands(newRootCmd()) {
		path := c.CommandPath()
		if c.Short == "" {
			continue // grouping parent without Short is allowed
		}
		if hasHangul(c.Short) {
			t.Errorf("%s: Short contains Hangul: %q", path, c.Short)
		}
		if strings.HasSuffix(c.Short, ".") {
			t.Errorf("%s: Short should not end with a period: %q", path, c.Short)
		}
		for _, banned := range []string{"공식 API", "TODO", "WTS-only"} {
			if strings.Contains(c.Short, banned) {
				t.Errorf("%s: Short contains banned token %q: %q", path, banned, c.Short)
			}
		}
	}
}

func TestLeafCommandsHaveSourceAnnotation(t *testing.T) {
	// local = command hits no remote market/account API (version, config, doctor,
	// local session ops). official/wts/both = data source for remote commands.
	valid := map[string]bool{"official": true, "wts": true, "both": true, "local": true}
	for _, c := range leafCommands(newRootCmd()) {
		src, ok := c.Annotations["source"]
		if !ok {
			t.Errorf("%s: missing 'source' annotation", c.CommandPath())
			continue
		}
		if !valid[src] {
			t.Errorf("%s: invalid source %q (want official|wts|both|local)", c.CommandPath(), src)
		}
	}
}

func TestMutatingAnnotationOnTradeCommands(t *testing.T) {
	wantMutating := map[string]bool{
		// `ops call` reaches the same write operations `order` does — it dispatches
		// through the same trading.Service gate — so it must declare itself too.
		// Everything else here is a typed trade action.
		"tossctl ops call":                 true,
		"tossctl order place":              true,
		"tossctl order cancel":             true,
		"tossctl order amend":              true,
		"tossctl order conditional place":  true,
		"tossctl order conditional cancel": true,
		"tossctl order conditional modify": true,
	}
	for _, c := range leafCommands(newRootCmd()) {
		path := c.CommandPath()
		isMut := c.Annotations["mutating"] == "true"
		if wantMutating[path] && !isMut {
			t.Errorf("%s: expected mutating=true annotation", path)
		}
		if !wantMutating[path] && isMut {
			t.Errorf("%s: unexpected mutating=true (only trade actions should mutate): %q", path, c.Annotations["mutating"])
		}
	}
}

func TestStateChangingCommandsDeclareRiskAndReversibility(t *testing.T) {
	want := map[string][2]string{
		"tossctl openapi ip replace-current":        {"preference", "compensating"},
		"tossctl quote alert add":                   {"preference", "reversible"},
		"tossctl quote alert remove":                {"preference", "reversible"},
		"tossctl portfolio hidden hide":             {"preference", "reversible"},
		"tossctl portfolio hidden show":             {"preference", "reversible"},
		"tossctl watchlist group create":            {"preference", "reversible"},
		"tossctl watchlist group rename":            {"preference", "reversible"},
		"tossctl watchlist group delete":            {"destructive", "irreversible"},
		"tossctl watchlist add":                     {"preference", "reversible"},
		"tossctl watchlist remove":                  {"preference", "reversible"},
		"tossctl config experimental paper-trading": {"preference", "reversible"},
		"tossctl order place":                       {"financial", "irreversible"},
		"tossctl order cancel":                      {"financial", "irreversible"},
		"tossctl order amend":                       {"financial", "irreversible"},
		"tossctl order conditional place":           {"financial", "irreversible"},
		"tossctl order conditional cancel":          {"financial", "irreversible"},
		"tossctl order conditional modify":          {"financial", "irreversible"},
		"tossctl paper init":                        {"simulation", "unknown"},
		"tossctl paper deposit":                     {"simulation", "unknown"},
		"tossctl paper order place":                 {"simulation", "irreversible-in-simulation"},
		"tossctl paper order cancel":                {"simulation", "irreversible-in-simulation"},
		"tossctl paper orders cancel-all":           {"simulation", "irreversible-in-simulation"},
	}
	seen := map[string]bool{}
	for _, cmd := range leafCommands(newRootCmd()) {
		path := cmd.CommandPath()
		policy, expected := want[path]
		writes := cmd.Annotations["writes_state"] == "true"
		if expected {
			seen[path] = true
			if !writes || cmd.Annotations["mutation_risk"] != policy[0] || cmd.Annotations["reversibility"] != policy[1] {
				t.Errorf("%s: incomplete mutation annotations: %#v", path, cmd.Annotations)
			}
		} else if writes {
			t.Errorf("%s: unexpected writes_state=true annotation", path)
		}
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("expected mutation command %s not found", path)
		}
	}
	call, _, err := newRootCmd().Find([]string{"ops", "call"})
	if err != nil || call.Annotations["writes_state"] != "possible" || call.Annotations["mutation_risk"] != "operation-defined" {
		t.Errorf("ops call must declare dynamic mutation policy: cmd=%v err=%v annotations=%#v", call, err, call.Annotations)
	}
}
