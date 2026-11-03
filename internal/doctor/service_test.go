package doctor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
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

// TestRedactValueBoundary guards issue #18: only the exact home dir or a
// separator-terminated prefix is replaced — /Users/lee must never mangle
// /Users/leejongyoung — while mid-string paths are still caught.
func TestRedactValueBoundary(t *testing.T) {
	home := "/Users/lee"
	cases := []struct {
		in, want string
	}{
		{"/Users/lee", "~"},
		{"/Users/lee/.config/tossinvest-cli", "~/.config/tossinvest-cli"},
		{`C:\Users\lee\cfg`, `C:\Users\lee\cfg`}, // backslash home needs backslash base
		{"/Users/leejongyoung/.config", "/Users/leejongyoung/.config"},
		{"using /Users/lee/x for y", "using ~/x for y"},
		{"no home here", "no home here"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := redactValue(tc.in, home); got != tc.want {
			t.Errorf("redactValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := redactValue(`C:\Users\lee\cfg`, `C:\Users\lee`); got != `~\cfg` {
		t.Errorf("backslash home must redact, got %q", got)
	}
	nested := map[string]any{
		"paths": []any{"/Users/lee/a", map[string]any{"d": "see /Users/lee/b"}},
		"other": "/Users/leejongyoung/c",
	}
	out := redactValue(nested, home).(map[string]any)
	arr := out["paths"].([]any)
	if arr[0] != "~/a" {
		t.Errorf("nested slice not redacted: %v", arr)
	}
	if arr[1].(map[string]any)["d"] != "see ~/b" {
		t.Errorf("nested map not redacted: %v", arr)
	}
	if out["other"] != "/Users/leejongyoung/c" {
		t.Errorf("boundary violated: %v", out)
	}
}

// TestRedactReportCoversCredentialPaths guards issue #18: the credential and
// token files must not leak the username when the report is pasted into an
// issue.
func TestRedactReportCoversCredentialPaths(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	r := Report{
		Paths: config.Paths{
			CredentialsFile: filepath.Join(fakeHome, ".config", "tossinvest-cli", "openapi-credentials.json"),
			TokenFile:       filepath.Join(fakeHome, ".cache", "tossinvest-cli", "openapi-token.json"),
		},
		Checks: []Check{{Name: "x", Summary: "using " + filepath.Join(fakeHome, "y")}},
	}
	redactReport(&r)
	for _, got := range []string{r.Paths.CredentialsFile, r.Paths.TokenFile} {
		if !strings.HasPrefix(got, "~"+string(filepath.Separator)) {
			t.Errorf("path not redacted: %q", got)
		}
	}
	if r.Checks[0].Summary != "using "+filepath.Join("~", "y") {
		t.Errorf("mid-string path not redacted: %q", r.Checks[0].Summary)
	}
}

// TestCheckFileModesIncludesCredentials guards issue #18: API credential and
// token files are audited like every other secret.
func TestCheckFileModesIncludesCredentials(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "openapi-credentials.json")
	tokenFile := filepath.Join(dir, "openapi-token.json")
	if err := os.WriteFile(credFile, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Chmod (not WriteFile mode bits) so the wide permission survives any umask.
	if err := os.Chmod(credFile, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewService(config.Paths{ConfigDir: dir, CacheDir: dir, CredentialsFile: credFile, TokenFile: tokenFile}, config.Status{}, auth.LoginConfig{}, nil)
	var foundCred, foundToken bool
	for _, c := range s.checkFileModes() {
		if strings.HasSuffix(c.Path, "openapi-credentials.json") {
			foundCred = true
			if c.OK {
				t.Error("0644 credentials file must fail the 0600 audit")
			}
		}
		if strings.HasSuffix(c.Path, "openapi-token.json") {
			foundToken = true
		}
	}
	if !foundCred || !foundToken {
		t.Fatal("credential and token files must be in the mode audit")
	}
}

// TestExternalCheckHonorsContextCancel: a cancelled context must fail the
// check fast instead of hanging on a wedged subprocess.
func TestExternalCheckHonorsContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sleep as a hanging stand-in")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := checkPythonModule(ctx, auth.LoginConfig{PythonBin: "sleep", HelperDir: t.TempDir()}, "sleep-probe", "ok", "unavailable")
	if got.Status != CheckWarn {
		t.Fatalf("cancelled check must warn, got %+v", got)
	}
}
