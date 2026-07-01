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
		"tossctl order place":  true,
		"tossctl order cancel": true,
		"tossctl order amend":  true,
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
