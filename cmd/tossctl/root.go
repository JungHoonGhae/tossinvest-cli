package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/onboarding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/selfupdate"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	outputFormat string
	configDir    string
	sessionFile  string
	backend      string // --backend flag: overrides cfg.OpenAPI.Prefer for this run
}

type appContext struct {
	format         output.Format
	paths          config.Paths
	config         config.File
	configService  *config.Service
	loginConfig    auth.LoginConfig
	authService    *auth.Service
	client         *hybrid.Client
	session        *session.Session
	lineageService *orderlineage.Service
	tradingService *trading.Service
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "tossctl",
		Short:        i18n.T("root.short"),
		Long:         i18n.T("root.long"),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseFormat(opts.outputFormat)
			if err != nil {
				return err
			}
			store := session.NewFileStore(resolveSessionFile(opts))
			sess, _ := store.Load(cmd.Context())
			var gate func() bool
			var mark func()
			var configGate func() bool
			var configMark func()
			if cachePath, err := resolveUpdateCachePath(opts); err == nil {
				checker := updatecheck.New(cachePath)
				gate = checker.ShouldNotifyExpiry
				mark = checker.MarkExpiryNotified
				configGate = checker.ShouldNotifyConfig
				configMark = checker.MarkConfigNotified
			}
			writeExpiryWarningIfNeeded(cmd.ErrOrStderr(), sess, cmd.Name(), format, time.Now(), gate, mark)
			if status, err := loadConfigStatus(opts); err == nil {
				writeConfigLegacyWarningIfNeeded(cmd.ErrOrStderr(), status, cmd.Name(), format, configGate, configMark)
			}

			// First-run hint: nudge users with no auth at all toward `tossctl init`.
			// Never blocks; suppressed in non-interactive and JSON/CSV modes.
			if format == output.FormatTable {
				hasOfficialCreds := false
				if credFile, _, err := resolveOpenAPIPaths(opts); err == nil {
					if creds, err := official.LoadCredentials(os.Getenv, credFile); err == nil && creds != nil {
						hasOfficialCreds = true
					}
				}
				state := onboarding.State{
					HasSession:       sess != nil,
					HasOfficialCreds: hasOfficialCreds,
				}
				if shouldHintOnboarding(state, tui.IsInteractive(os.Stdin, os.Stdout), cmd.Name()) {
					fmt.Fprintln(cmd.ErrOrStderr(), "First time here? Run `tossctl init` to set up authentication.")
				}
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			writeUpdateNoticeIfNeeded(cmd.Context(), cmd.ErrOrStderr(), opts)
		},
	}

	cmd.PersistentFlags().StringVar(
		&opts.outputFormat,
		"output",
		string(output.FormatTable),
		"Output format: table, json, csv",
	)
	cmd.PersistentFlags().StringVar(
		&opts.configDir,
		"config-dir",
		"",
		"Override the config directory",
	)
	cmd.PersistentFlags().StringVar(
		&opts.sessionFile,
		"session-file",
		"",
		"Override the session file path",
	)
	cmd.PersistentFlags().StringVar(
		&opts.backend,
		"backend",
		"",
		"Override routing backend for this run: auto|wts|openapi",
	)
	cmd.PersistentFlags().String("lang", "", "UI language for help, prompts, and table output: en|ko (also TOSSCTL_LANG / LANG)")

	cmd.AddCommand(
		newInitCmd(opts),
		newVersionCmd(opts),
		newUpdateCmd(opts),
		newDoctorCmd(opts),
		newConfigCmd(opts),
		newAuthCmd(opts),
		newOpenAPICmd(opts),
		newAccountCmd(opts),
		newPortfolioCmd(opts),
		newLendingCmd(opts),
		newAccumulateCmd(opts),
		newOrdersCmd(opts),
		newTransactionsCmd(opts),
		newWatchlistCmd(opts),
		newQuoteCmd(opts),
		newMarketCmd(opts),
		newCommunityCmd(opts),
		newOrderCmd(opts),
		newExportCmd(opts),
		newPushCmd(opts),
		newMonitorCmd(opts),
		newMCPCmd(opts),
	)

	return cmd
}

// resolveSessionFile mirrors the resolution done in newAppContext but without
// requiring the full app context — PersistentPreRunE runs before subcommands
// have built theirs.
func resolveSessionFile(opts *rootOptions) string {
	if opts.sessionFile != "" {
		return opts.sessionFile
	}
	if opts.configDir != "" {
		return filepath.Join(opts.configDir, "session.json")
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		return ""
	}
	return paths.SessionFile
}

var expiryWarningSkipCommands = map[string]struct{}{
	"extend":                  {},
	"login":                   {},
	"logout":                  {},
	"status":                  {},
	"import-playwright-state": {},
	"version":                 {},
	"help":                    {},
}

// hintOnboardingSkipCommands lists commands where the first-run onboarding
// hint is noise or would be recursive (init itself, meta commands).
var hintOnboardingSkipCommands = map[string]struct{}{
	"init":       {},
	"help":       {},
	"completion": {},
	"version":    {},
}

// shouldHintOnboarding is a pure helper that returns true when all three
// conditions hold: the user needs onboarding, the terminal is interactive,
// and the command is not in the exclusion set. It never performs I/O.
func shouldHintOnboarding(state onboarding.State, interactive bool, cmdName string) bool {
	if !onboarding.NeedsOnboarding(state) {
		return false
	}
	if !interactive {
		return false
	}
	_, skip := hintOnboardingSkipCommands[cmdName]
	return !skip
}

// writeExpiryWarningIfNeeded prints a session-expiry hint to stderr when the
// session is within 24h of expiry. The optional `gate` and `mark` callbacks
// implement a 1-hour backoff so agents calling tossctl repeatedly don't see
// the same warning on every invocation; pass nil for both to disable the
// backoff (used by unit tests).
func writeExpiryWarningIfNeeded(w io.Writer, sess *session.Session, cmdName string, format output.Format, now time.Time, gate func() bool, mark func()) {
	if sess == nil || sess.ServerExpiresAt == nil {
		return
	}
	if format == output.FormatJSON {
		return
	}
	if _, skip := expiryWarningSkipCommands[cmdName]; skip {
		return
	}
	remaining := sess.ServerExpiresAt.Sub(now)
	if remaining <= 0 || remaining >= 24*time.Hour {
		return
	}
	if gate != nil && !gate() {
		return
	}
	fmt.Fprintf(w, "⚠ session expires in ~%s; run `tossctl auth extend` to renew\n", humanizeDuration(remaining))
	if mark != nil {
		mark()
	}
}

// configWarningSkipCommands lists commands where a config-legacy nudge is
// noise: the config command already prints full status (so the warning would
// be redundant right above it), and version/help don't touch config behavior.
var configWarningSkipCommands = map[string]struct{}{
	"config":  {},
	"doctor":  {},
	"version": {},
	"help":    {},
}

// writeConfigLegacyWarningIfNeeded prints a one-line stderr hint when the
// user's config carries fields that are no longer wired (LegacyFields) or was
// written by an older schema than this binary expects. Previously this drift
// was only surfaced by `tossctl config status`/`doctor`, so a user who never
// re-ran those could keep a stale config silently. The optional gate/mark
// callbacks apply the same 24h backoff as the update notice; pass nil for both
// to disable the backoff (used by unit tests).
func writeConfigLegacyWarningIfNeeded(w io.Writer, status config.Status, cmdName string, format output.Format, gate func() bool, mark func()) {
	if !status.Exists || format == output.FormatJSON {
		return
	}
	if _, skip := configWarningSkipCommands[cmdName]; skip {
		return
	}
	stale := status.SourceSchemaVersion != 0 && status.SourceSchemaVersion < config.SchemaVersion
	if len(status.LegacyFields) == 0 && !stale {
		return
	}
	if gate != nil && !gate() {
		return
	}
	switch {
	case len(status.LegacyFields) > 0:
		fmt.Fprintf(w, "⚠ config has unused legacy field(s): %s — run `tossctl config status` and remove them from %s\n",
			strings.Join(status.LegacyFields, ", "), status.ConfigFile)
	default:
		fmt.Fprintf(w, "⚠ config schema is outdated (v%d, current v%d) — run `tossctl config status` to review %s\n",
			status.SourceSchemaVersion, config.SchemaVersion, status.ConfigFile)
	}
	if mark != nil {
		mark()
	}
}

// loadConfigStatus resolves the config file path and returns its Status without
// requiring the full app context (PersistentPreRunE runs before subcommands
// build theirs).
func loadConfigStatus(opts *rootOptions) (config.Status, error) {
	configFile, err := configFilePath(opts)
	if err != nil {
		return config.Status{}, err
	}
	return config.NewService(configFile).Status(context.Background())
}

// writeUpdateNoticeIfNeeded prints a single stderr line when a newer stable
// tossctl release is available. Output is gated by a 24h backoff in the
// updatecheck cache so cron jobs and AI-agent loops that invoke tossctl many
// times don't see the same notice on every call; JSON/CSV output is still
// skipped so structured pipelines stay clean. Network and config errors are
// silent — the notice is a courtesy, not a correctness signal.
func writeUpdateNoticeIfNeeded(ctx context.Context, stderr io.Writer, opts *rootOptions) {
	if version.Version == "dev" {
		return
	}
	format, err := output.ParseFormat(opts.outputFormat)
	if err != nil || format != output.FormatTable {
		return
	}

	checker := newUpdateChecker(opts)
	if checker == nil {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	latest, ok := checker.ShouldNotifyUpdate(checkCtx, version.Version)
	if !ok {
		return
	}

	configFile, _ := configFilePath(opts)
	fmt.Fprintf(
		stderr,
		"\n✨ tossctl %s available (current %s) — %s\n   Disable: set update_check.enabled=false in %s\n",
		latest,
		version.Version,
		updateActionHint(version.Version),
		configFile,
	)
	checker.MarkUpdateNotified()
}

// updateActionHint returns the right upgrade instruction for the running
// binary's install method, so the "update available" nudge doesn't tell a
// curl-installed user to run `brew upgrade` (or a brew user to run
// `tossctl update`, which would just re-delegate to brew anyway but is a
// confusing detour).
func updateActionHint(currentVersion string) string {
	execPath, err := os.Executable()
	if err != nil {
		return "`tossctl update` or " + version.ReleasesLatestURL
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	if selfupdate.DetectInstallMethod(execPath, currentVersion) == selfupdate.MethodHomebrew {
		return "`brew upgrade tossctl-cli` or " + version.ReleasesLatestURL
	}
	return "`tossctl update` or " + version.ReleasesLatestURL
}

// newUpdateChecker constructs an updatecheck.Checker honoring update_check
// settings in the user's config. Returns nil when the feature is disabled or
// paths cannot be resolved — callers should treat nil as "skip the notice."
//
// The expiry-warning backoff uses the same cache file but is wired through
// resolveUpdateCachePath directly, so disabling update_check does not turn
// off the expiry-warning rate-limit.
func newUpdateChecker(opts *rootOptions) *updatecheck.Checker {
	cfg, err := loadConfig(opts)
	if err != nil || !cfg.UpdateCheck.Enabled {
		return nil
	}
	cachePath, err := resolveUpdateCachePath(opts)
	if err != nil {
		return nil
	}
	return updatecheck.New(cachePath)
}

func resolveUpdateCachePath(opts *rootOptions) (string, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", err
	}
	cacheDir := paths.CacheDir
	if opts.configDir != "" {
		cacheDir = opts.configDir
	}
	return filepath.Join(cacheDir, "update-check.json"), nil
}

func loadConfig(opts *rootOptions) (config.File, error) {
	configFile, err := configFilePath(opts)
	if err != nil {
		return config.File{}, err
	}
	return config.NewService(configFile).Load(context.Background())
}

func configFilePath(opts *rootOptions) (string, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", err
	}
	if opts.configDir != "" {
		return filepath.Join(opts.configDir, "config.json"), nil
	}
	return paths.ConfigFile, nil
}

// resolveBackend returns the effective routing backend preference.
// The --backend flag takes precedence over cfg.Prefer.
// An empty flag means "use config". Invalid flag values are rejected.
func resolveBackend(cfg config.OpenAPI, flag string) (string, error) {
	if flag == "" {
		return cfg.Prefer, nil
	}
	if norm, ok := config.NormalizeBackend(flag); ok {
		return norm, nil
	}
	return "", fmt.Errorf("invalid --backend value %q: must be one of auto, wts, openapi", flag)
}

func humanizeDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	hours := int(d.Hours())
	if hours >= 1 {
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	minutes := int(d.Minutes())
	if minutes >= 1 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func newAppContext(opts *rootOptions) (*appContext, error) {
	format, err := output.ParseFormat(opts.outputFormat)
	if err != nil {
		return nil, err
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}

	if opts.configDir != "" {
		paths.ConfigDir = opts.configDir
		paths.ConfigFile = filepath.Join(opts.configDir, "config.json")
		paths.SessionFile = filepath.Join(opts.configDir, "session.json")
		paths.LineageFile = filepath.Join(opts.configDir, "trading-lineage.json")
	}

	if opts.sessionFile != "" {
		paths.SessionFile = opts.sessionFile
	}

	store := session.NewFileStore(paths.SessionFile)
	sess, err := store.Load(context.Background())
	if err != nil && !errors.Is(err, session.ErrNoSession) {
		return nil, err
	}

	loginConfig := auth.DefaultLoginConfig(paths.CacheDir)
	configService := config.NewService(paths.ConfigFile)
	cfg, err := configService.Load(context.Background())
	if err != nil {
		return nil, err
	}

	wtsClient := tossclient.New(tossclient.Config{
		Session:       sess,
		TradingPolicy: cfg.Trading,
	})

	prefer, err := resolveBackend(cfg.OpenAPI, opts.backend)
	if err != nil {
		return nil, err
	}

	credFile, tokenFile, err := resolveOpenAPIPaths(opts)
	if err != nil {
		return nil, err
	}
	creds, err := official.LoadCredentials(os.Getenv, credFile)
	if err != nil {
		return nil, fmt.Errorf("loading official credentials: %w", err)
	}

	var off *official.Client
	if creds != nil && cfg.OpenAPI.Enabled && prefer != "wts" {
		off = official.New(*creds, tokenFile)
	}

	h := hybrid.New(wtsClient, off, hybrid.Policy{Prefer: prefer, Fallback: cfg.OpenAPI.Fallback}, os.Stderr)

	return &appContext{
		format:        format,
		paths:         paths,
		config:        cfg,
		configService: configService,
		loginConfig:   loginConfig,
		authService: auth.NewService(store, paths.SessionFile, auth.Options{
			LoginConfig:     loginConfig,
			Validator:       wtsClient,
			ExtensionRunner: wtsClient,
		}),
		client:         h,
		session:        sess,
		lineageService: orderlineage.NewService(paths.LineageFile),
		tradingService: trading.NewService(cfg.Trading, h.Broker()),
	}, nil
}
