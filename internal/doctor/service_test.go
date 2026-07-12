package doctor

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func TestCheckLiveOrderActionsDisabled(t *testing.T) {
	check := checkLiveOrderActions(config.Status{})
	if check.Status != CheckInfo {
		t.Fatalf("expected info status, got %s", check.Status)
	}
	if check.Name != "live_order_actions" {
		t.Fatalf("unexpected check name: %s", check.Name)
	}
}

func TestCheckDangerousAutomationEnabled(t *testing.T) {
	check := checkDangerousAutomation(config.Status{
		Trading: config.Trading{
			DangerousAutomation: config.DangerousAutomation{
				AcceptFXConsent: true,
			},
		},
	})
	if check.Status != CheckWarn {
		t.Fatalf("expected warn status, got %s", check.Status)
	}
	if !strings.Contains(check.Detail, "accept_fx_consent") {
		t.Fatalf("unexpected dangerous automation detail: %s", check.Detail)
	}
}

func TestCheckLegacyConfig(t *testing.T) {
	check := checkLegacyConfig(config.Status{
		Exists:       true,
		LegacyFields: []string{"trading.allow_dangerous_execute"},
	})
	if check.Status != CheckWarn {
		t.Fatalf("expected warn status, got %s", check.Status)
	}
	if check.Name != "legacy_config" {
		t.Fatalf("unexpected check name: %s", check.Name)
	}
}

func hasHangul(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hangul) {
			return true
		}
	}
	return false
}

// TestCheckMessagesLocalized guards against check messages being hardcoded in
// English again: under `ko`, both a static summary and a `%s`-formatted summary
// must resolve through the i18n catalog (i.e. contain Hangul).
func TestCheckMessagesLocalized(t *testing.T) {
	orig := i18n.Lang()
	i18n.SetLang("ko")
	t.Cleanup(func() { i18n.SetLang(orig) })

	live := checkLiveOrderActions(config.Status{})
	if !hasHangul(live.Summary) {
		t.Errorf("live_order_actions summary not localized under ko: %q", live.Summary)
	}
	if !hasHangul(live.Detail) {
		t.Errorf("live_order_actions detail not localized under ko: %q", live.Detail)
	}

	missing := checkPath("config_dir", filepath.Join(t.TempDir(), "does-not-exist"))
	if !hasHangul(missing.Summary) {
		t.Errorf("path summary not localized under ko: %q", missing.Summary)
	}
}
