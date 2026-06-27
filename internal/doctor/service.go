package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
)

type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
	CheckInfo CheckStatus = "info"
)

type Check struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Summary string      `json:"summary"`
	Detail  string      `json:"detail,omitempty"`
}

type AuthReport struct {
	PythonBinary string      `json:"python_binary"`
	HelperDir    string      `json:"helper_dir"`
	Session      auth.Status `json:"session"`
	Checks       []Check     `json:"checks"`
}

type Report struct {
	Version     version.Info  `json:"version"`
	GoVersion   string        `json:"go_version"`
	OS          string        `json:"os"`
	Arch        string        `json:"arch"`
	Paths       config.Paths  `json:"paths"`
	Config      config.Status `json:"config"`
	Auth        AuthReport    `json:"auth"`
	Checks      []Check       `json:"checks"`
	Diagnostics *Diagnostics  `json:"diagnostics,omitempty"`

	// OpenAPISummary is a one-line hybrid/Open API status note set by the
	// doctor command after Run() returns. It is omitted when empty (e.g. when
	// this field is not populated in --report mode).
	OpenAPISummary string `json:"open_api_summary,omitempty"`
}

// Diagnostics captures extra signals that are only useful for bug reports /
// maintenance — surfaced via `tossctl doctor --report`. Kept out of the
// default Run() so that a plain `tossctl doctor` stays fast and does not hit
// the network.
type Diagnostics struct {
	Probes       []tossclient.ProbeResult `json:"probes,omitempty"`
	ProbeSkipped string                   `json:"probe_skipped,omitempty"`
	FileModes    []FileModeCheck          `json:"file_modes"`
	OrphanFiles  []string                 `json:"orphan_files"`
}

type FileModeCheck struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	IsDir    bool   `json:"is_dir,omitempty"`
	Mode     string `json:"mode,omitempty"`     // "0600" / "0755"
	Expected string `json:"expected,omitempty"` // "0600" / "0700"
	OK       bool   `json:"ok"`
}

type authStatusReader interface {
	Status(context.Context) (auth.Status, error)
}

type sessionProber interface {
	Probe(context.Context) []tossclient.ProbeResult
}

type Service struct {
	paths       config.Paths
	configState config.Status
	loginConfig auth.LoginConfig
	authService authStatusReader
}

func NewService(paths config.Paths, configState config.Status, loginConfig auth.LoginConfig, authService authStatusReader) *Service {
	return &Service{
		paths:       paths,
		configState: configState,
		loginConfig: loginConfig,
		authService: authService,
	}
}

func (s *Service) Run(ctx context.Context) (Report, error) {
	authReport, err := s.RunAuth(ctx)
	if err != nil {
		return Report{}, err
	}

	checks := []Check{
		checkPath("config_dir", s.paths.ConfigDir),
		checkPath("cache_dir", s.paths.CacheDir),
		checkFile("config_file", s.paths.ConfigFile),
		checkFile("session_file", s.paths.SessionFile),
		checkFile("lineage_file", s.paths.LineageFile),
		checkTradingConfig(s.configState),
		checkLiveOrderActions(s.configState),
		checkDangerousAutomation(s.configState),
		checkLegacyConfig(s.configState),
		{
			Name:    "trading_scope",
			Status:  CheckInfo,
			Summary: "trading support is intentionally narrow and still beta",
			Detail:  "Currently validated for US/KR buy/sell limit + US fractional (market) orders in KRW, plus same-day pending cancel. US and KR are treated symmetrically (only `trading.place` is needed). Sell requires `trading.sell=true`, fractional requires `trading.fractional=true`. Amend still needs more live verification.",
		},
	}

	return Report{
		Version:   version.Current(),
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Paths:     s.paths,
		Config:    s.configState,
		Auth:      authReport,
		Checks:    checks,
	}, nil
}

// RunReport extends Run() with extra diagnostics suitable for bug reports:
// endpoint-family probes, file permission audit, and orphan cache file
// detection. The caller passes a session-bound client (or nil to skip probe).
// All paths in the returned Report are redacted to `~` — this output is meant
// to be pasted into GitHub issues, so we must not leak the user's home dir
// (which on macOS includes the account username).
func (s *Service) RunReport(ctx context.Context, prober sessionProber) (Report, error) {
	report, err := s.Run(ctx)
	if err != nil {
		return report, err
	}

	diag := &Diagnostics{
		FileModes:   s.checkFileModes(),
		OrphanFiles: s.checkOrphanFiles(),
	}

	if prober != nil && report.Auth.Session.Active {
		diag.Probes = prober.Probe(ctx)
	} else {
		diag.ProbeSkipped = "no active session"
	}

	report.Diagnostics = diag
	redactReport(&report)
	return report, nil
}

// redactReport replaces the user's home directory prefix with `~` in every
// path field surfaced by --report. Does not touch URLs, UA strings, or error
// messages from network probes (those don't contain local PII).
func redactReport(r *Report) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}

	redact := func(s string) string {
		if s == "" {
			return s
		}
		if strings.HasPrefix(s, home) {
			return "~" + s[len(home):]
		}
		return s
	}

	r.Paths.ConfigDir = redact(r.Paths.ConfigDir)
	r.Paths.CacheDir = redact(r.Paths.CacheDir)
	r.Paths.ConfigFile = redact(r.Paths.ConfigFile)
	r.Paths.SessionFile = redact(r.Paths.SessionFile)
	r.Paths.LineageFile = redact(r.Paths.LineageFile)

	r.Config.ConfigFile = redact(r.Config.ConfigFile)

	r.Auth.PythonBinary = redact(r.Auth.PythonBinary)
	r.Auth.HelperDir = redact(r.Auth.HelperDir)
	r.Auth.Session.SessionFile = redact(r.Auth.Session.SessionFile)

	for i := range r.Checks {
		r.Checks[i].Summary = redact(r.Checks[i].Summary)
		r.Checks[i].Detail = redact(r.Checks[i].Detail)
	}
	for i := range r.Auth.Checks {
		r.Auth.Checks[i].Summary = redact(r.Auth.Checks[i].Summary)
		r.Auth.Checks[i].Detail = redact(r.Auth.Checks[i].Detail)
	}

	if r.Diagnostics != nil {
		for i := range r.Diagnostics.FileModes {
			r.Diagnostics.FileModes[i].Path = redact(r.Diagnostics.FileModes[i].Path)
		}
		for i := range r.Diagnostics.OrphanFiles {
			r.Diagnostics.OrphanFiles[i] = redact(r.Diagnostics.OrphanFiles[i])
		}
	}
}

// checkFileModes verifies tossctl's state files are 0600 and state dirs 0700.
// Reports existing-only (missing files are not a failure; checkFile covers that).
func (s *Service) checkFileModes() []FileModeCheck {
	dirs := []string{s.paths.ConfigDir}
	// CacheDir may equal ConfigDir on Linux; dedup.
	if s.paths.CacheDir != "" && s.paths.CacheDir != s.paths.ConfigDir {
		dirs = append(dirs, s.paths.CacheDir)
	}
	files := []string{
		s.paths.ConfigFile,
		s.paths.SessionFile,
		s.paths.LineageFile,
	}

	checks := make([]FileModeCheck, 0, len(dirs)+len(files))
	for _, d := range dirs {
		checks = append(checks, inspectMode(d, true, 0o700))
	}
	for _, f := range files {
		checks = append(checks, inspectMode(f, false, 0o600))
	}
	return checks
}

func inspectMode(path string, isDir bool, expected os.FileMode) FileModeCheck {
	check := FileModeCheck{Path: path, IsDir: isDir, Expected: fmt.Sprintf("%#o", expected)}
	info, err := os.Stat(path)
	if err != nil {
		// Non-existence is not a failure here — the regular Run() Checks
		// already report file presence. Report as OK=true so --report
		// doesn't flag fresh installs.
		check.OK = true
		return check
	}
	check.Exists = true
	mode := info.Mode().Perm()
	check.Mode = fmt.Sprintf("%#o", mode)
	check.OK = mode == expected
	return check
}

// checkOrphanFiles looks for intermediate state files that should have been
// cleaned up after a successful login (see LoginWith in internal/auth). Their
// presence means either a login mid-failure or a pre-0.4.1 leftover.
func (s *Service) checkOrphanFiles() []string {
	candidates := []string{
		filepath.Join(s.paths.CacheDir, "auth", "playwright-storage-state.json"),
	}
	var orphans []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			orphans = append(orphans, p)
		}
	}
	return orphans
}

func (s *Service) RunAuth(ctx context.Context) (AuthReport, error) {
	sessionStatus, err := s.authService.Status(ctx)
	if err != nil {
		return AuthReport{}, err
	}

	checks := []Check{
		checkPythonBinary(s.loginConfig.PythonBin),
		checkPath("auth_helper_dir", s.loginConfig.HelperDir),
		checkPythonModule(s.loginConfig, "tossctl_auth_helper", "auth helper module is importable", "auth helper module is not importable"),
		checkPythonModule(s.loginConfig, "playwright", "python playwright package is installed", "python playwright package is not installed"),
		checkChrome(s.loginConfig),
		checkSession(sessionStatus),
	}

	return AuthReport{
		PythonBinary: s.loginConfig.PythonBin,
		HelperDir:    s.loginConfig.HelperDir,
		Session:      sessionStatus,
		Checks:       checks,
	}, nil
}

func checkPath(name, path string) Check {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return Check{Name: name, Status: CheckOK, Summary: fmt.Sprintf("%s exists", path)}
	case err == nil:
		return Check{Name: name, Status: CheckWarn, Summary: fmt.Sprintf("%s exists but is not a directory", path)}
	case errors.Is(err, os.ErrNotExist):
		return Check{Name: name, Status: CheckWarn, Summary: fmt.Sprintf("%s does not exist yet", path)}
	default:
		return Check{Name: name, Status: CheckWarn, Summary: fmt.Sprintf("could not inspect %s", path), Detail: err.Error()}
	}
}

func checkFile(name, path string) Check {
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return Check{Name: name, Status: CheckOK, Summary: fmt.Sprintf("%s exists", path)}
	case err == nil:
		return Check{Name: name, Status: CheckWarn, Summary: fmt.Sprintf("%s exists but is a directory", path)}
	case errors.Is(err, os.ErrNotExist):
		return Check{Name: name, Status: CheckInfo, Summary: fmt.Sprintf("%s does not exist yet", path)}
	default:
		return Check{Name: name, Status: CheckWarn, Summary: fmt.Sprintf("could not inspect %s", path), Detail: err.Error()}
	}
}

func checkPythonBinary(pythonBin string) Check {
	path, err := exec.LookPath(pythonBin)
	if err != nil {
		return Check{
			Name:    "python_binary",
			Status:  CheckFail,
			Summary: fmt.Sprintf("%s was not found in PATH", pythonBin),
			Detail:  "Install Python 3.11+ or set TOSSCTL_AUTH_HELPER_PYTHON.",
		}
	}

	return Check{
		Name:    "python_binary",
		Status:  CheckOK,
		Summary: fmt.Sprintf("using %s", path),
	}
}

func checkPythonModule(cfg auth.LoginConfig, module, successSummary, failSummary string) Check {
	path, err := exec.LookPath(cfg.PythonBin)
	if err != nil {
		return Check{
			Name:    module,
			Status:  CheckFail,
			Summary: failSummary,
			Detail:  "Python is not available.",
		}
	}

	cmd := exec.Command(path, "-c", "import "+module)
	cmd.Dir = cfg.HelperDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Check{
			Name:    module,
			Status:  CheckWarn,
			Summary: failSummary,
			Detail:  string(output),
		}
	}

	return Check{
		Name:    module,
		Status:  CheckOK,
		Summary: successSummary,
	}
}

func checkChrome(cfg auth.LoginConfig) Check {
	path, err := exec.LookPath(cfg.PythonBin)
	if err != nil {
		return Check{
			Name:    "chrome",
			Status:  CheckFail,
			Summary: "chrome check skipped because python is unavailable",
		}
	}

	script := `import json, subprocess, sys
from playwright.sync_api import sync_playwright
p = sync_playwright().start()
try:
    b = p.chromium.launch(headless=True, channel="chrome")
    ua = b.new_page().evaluate("navigator.userAgent")
    b.close()
    print(ua)
except Exception as e:
    print("ERROR:" + str(e), file=sys.stderr)
    sys.exit(1)
finally:
    p.stop()
`
	cmd := exec.Command(path, "-c", script)
	cmd.Dir = cfg.HelperDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if strings.Contains(detail, "Executable doesn't exist") || strings.Contains(detail, "channel") {
			detail = "Google Chrome is not installed. Install from https://www.google.com/chrome/"
		}
		return Check{
			Name:    "chrome",
			Status:  CheckWarn,
			Summary: "Google Chrome is not available via Playwright",
			Detail:  firstLine(detail),
		}
	}

	ua := strings.TrimSpace(string(output))
	return Check{
		Name:    "chrome",
		Status:  CheckOK,
		Summary: "Google Chrome is available",
		Detail:  ua,
	}
}

func firstLine(value string) string {
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return value[:idx]
	}
	return value
}

func checkSession(status auth.Status) Check {
	switch {
	case !status.Active:
		return Check{
			Name:    "session",
			Status:  CheckInfo,
			Summary: "no stored session",
			Detail:  "Run `tossctl auth login` after your local auth environment is ready.",
		}
	case status.Validated && status.Valid:
		return Check{
			Name:    "session",
			Status:  CheckOK,
			Summary: "stored session is valid",
		}
	case status.Validated && !status.Valid:
		return Check{
			Name:    "session",
			Status:  CheckWarn,
			Summary: "stored session is no longer valid",
			Detail:  status.ValidationError,
		}
	case status.Expired:
		return Check{
			Name:    "session",
			Status:  CheckWarn,
			Summary: "stored session looks expired",
		}
	default:
		return Check{
			Name:    "session",
			Status:  CheckOK,
			Summary: "stored session exists",
		}
	}
}

func checkTradingConfig(status config.Status) Check {
	if !status.Exists {
		return Check{
			Name:    "trading_config",
			Status:  CheckInfo,
			Summary: "config file does not exist yet; trading actions default to disabled",
			Detail:  "Run `tossctl config init` to create config.json and enable only the actions you want.",
		}
	}

	enabled := status.Trading.EnabledActions()
	if len(enabled) == 0 {
		return Check{
			Name:    "trading_config",
			Status:  CheckInfo,
			Summary: "config file exists, but all trading actions are disabled",
			Detail:  "Edit config.json to explicitly allow the actions you want to use.",
		}
	}

	return Check{
		Name:    "trading_config",
		Status:  CheckOK,
		Summary: "one or more trading actions are enabled in config",
		Detail:  strings.Join(enabled, ", "),
	}
}

func checkLiveOrderActions(status config.Status) Check {
	if !status.Exists || !status.Trading.AllowLiveOrderActions {
		return Check{
			Name:    "live_order_actions",
			Status:  CheckInfo,
			Summary: "real account-changing order actions are blocked",
			Detail:  "Set `trading.allow_live_order_actions=true` only if you intend to let `place`, `cancel`, or `amend` reach the broker.",
		}
	}

	return Check{
		Name:    "live_order_actions",
		Status:  CheckWarn,
		Summary: "real account-changing order actions are enabled",
		Detail:  "Live `place`, `cancel`, and `amend` can execute after --execute + confirm token gates pass.",
	}
}

func checkDangerousAutomation(status config.Status) Check {
	enabled := status.Trading.DangerousAutomation.EnabledActions()
	if len(enabled) == 0 {
		return Check{
			Name:    "dangerous_automation",
			Status:  CheckInfo,
			Summary: "no risky broker branches will be auto-continued",
		}
	}

	return Check{
		Name:    "dangerous_automation",
		Status:  CheckWarn,
		Summary: "risky broker branch automation is enabled",
		Detail:  strings.Join(enabled, ", ") + " (only has effect when matching branch handlers exist in the current build)",
	}
}

func checkLegacyConfig(status config.Status) Check {
	if !status.Exists {
		return Check{
			Name:    "legacy_config",
			Status:  CheckInfo,
			Summary: "no config file is present, so no legacy translation is needed",
		}
	}

	if len(status.LegacyFields) == 0 {
		return Check{
			Name:    "legacy_config",
			Status:  CheckInfo,
			Summary: "config is already using the current trading policy keys",
		}
	}

	return Check{
		Name:    "legacy_config",
		Status:  CheckWarn,
		Summary: "legacy trading config keys detected (values ignored, safe to remove from config.json)",
		Detail:  strings.Join(status.LegacyFields, ", "),
	}
}
