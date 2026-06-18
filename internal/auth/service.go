package auth

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/junghoonkye/tossinvest-cli/internal/client"
	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

type LoginConfig struct {
	PythonBin        string
	HelperDir        string
	StorageStatePath string
	Headless         bool
	QROutputPath     string
}

type Options struct {
	LoginConfig     LoginConfig
	Runner          LoginRunner
	Validator       SessionValidator
	ExtensionRunner ExtensionRunner
	PollInterval    time.Duration // defaults to 1s when zero
}

type Service struct {
	store           session.Store
	sessionFile     string
	loginConfig     LoginConfig
	runner          LoginRunner
	validator       SessionValidator
	extensionRunner ExtensionRunner
	pollInterval    time.Duration
}

type Status struct {
	Active          bool       `json:"active"`
	Expired         bool       `json:"expired"`
	Provider        string     `json:"provider,omitempty"`
	CookieCount     int        `json:"cookie_count,omitempty"`
	StorageKeys     int        `json:"storage_keys,omitempty"`
	RetrievedAt     *time.Time `json:"retrieved_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	ServerExpiresAt *time.Time `json:"server_expires_at,omitempty"`
	SessionFile     string     `json:"session_file"`
	Validated       bool       `json:"validated"`
	Valid           bool       `json:"valid"`
	ValidationError string     `json:"validation_error,omitempty"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
}

type SessionValidator interface {
	ValidateSession(context.Context) error
	GetServerExpiredAt(context.Context) (time.Time, error)
}

type ExtensionRunner interface {
	RequestExtension(context.Context) (string, error)
	GetExtensionStatus(context.Context, string) (client.ExtensionStatus, error)
	FinalizeExtension(context.Context, string) error
	GetServerExpiredAt(context.Context) (time.Time, error)
}

func DefaultLoginConfig(cacheDir string) LoginConfig {
	pythonBin := os.Getenv("TOSSCTL_AUTH_HELPER_PYTHON")
	if pythonBin == "" {
		pythonBin = resolveDefaultPythonBin()
	}

	helperDir := os.Getenv("TOSSCTL_AUTH_HELPER_DIR")
	if helperDir == "" {
		helperDir = resolveDefaultHelperDir()
	}

	storageStatePath := os.Getenv("TOSSCTL_AUTH_STORAGE_STATE")
	if storageStatePath == "" {
		storageStatePath = filepath.Join(cacheDir, "auth", "playwright-storage-state.json")
	}

	return LoginConfig{
		PythonBin:        pythonBin,
		HelperDir:        helperDir,
		StorageStatePath: storageStatePath,
	}
}

func resolveDefaultPythonBin() string {
	for _, candidate := range defaultPythonCandidates() {
		if candidate == "" {
			continue
		}
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func defaultPythonCandidates() []string {
	var candidates []string
	for _, toolDir := range uvToolDirs() {
		for _, tool := range uvToolNames {
			candidates = append(candidates,
				filepath.Join(toolDir, tool, "bin", "python"),
				filepath.Join(toolDir, tool, "bin", "python3"),
				filepath.Join(toolDir, tool, "Scripts", "python.exe"),
			)
		}
	}
	if runtime.GOOS == "windows" {
		return append(candidates, "python", "python.exe", "python3")
	}
	return append(candidates, "python3")
}

// `playwright` works as a candidate even though it lacks the helper module —
// `python -m tossctl_auth_helper` resolves from cfg.HelperDir (cmd.Dir).
var uvToolNames = []string{"tossctl-auth-helper", "playwright"}

func uvToolDirs() []string {
	var dirs []string

	if dir := os.Getenv("UV_TOOL_DIR"); dir != "" {
		dirs = append(dirs, dir)
	}

	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "uv", "tools"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "uv", "tools"))
	}

	if appdata := os.Getenv("APPDATA"); appdata != "" {
		dirs = append(dirs, filepath.Join(appdata, "uv", "tools"))
	}

	return dirs
}

func resolveDefaultHelperDir() string {
	candidates := []string{"auth-helper"}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "auth-helper"),
			filepath.Join(exeDir, "..", "libexec", "auth-helper"),
			filepath.Join(exeDir, "..", "share", "tossctl", "auth-helper"),
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return candidates[0]
}

func NewService(store session.Store, sessionFile string, opts Options) *Service {
	runner := opts.Runner
	if runner == nil {
		runner = PythonLoginRunner{}
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}

	return &Service{
		store:           store,
		sessionFile:     sessionFile,
		loginConfig:     opts.LoginConfig,
		runner:          runner,
		validator:       opts.Validator,
		extensionRunner: opts.ExtensionRunner,
		pollInterval:    pollInterval,
	}
}

func (s *Service) Login(ctx context.Context) (*session.Session, error) {
	return s.LoginWith(ctx, s.loginConfig)
}

func (s *Service) LoginWith(ctx context.Context, cfg LoginConfig) (*session.Session, error) {
	result, err := s.runner.Login(ctx, cfg)
	if err != nil {
		return nil, err
	}

	sess, err := s.ImportPlaywrightState(ctx, result.StorageStatePath)
	if err != nil {
		return nil, err
	}

	// The helper-managed storage-state file holds a redundant copy of the full
	// cookie set (SESSION/UTK/LTK/FTK/XSRF-TOKEN) that we have just persisted
	// into session.json. Remove it so `auth logout` (which only clears
	// session.json) doesn't leave live credentials in the cache directory.
	// User-invoked `import-playwright-state <path>` does NOT pass through here,
	// so externally-supplied files are never deleted.
	_ = os.Remove(result.StorageStatePath)

	return sess, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	sess, err := s.store.Load(ctx)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return Status{
				Active:      false,
				Expired:     false,
				SessionFile: s.sessionFile,
			}, nil
		}

		return Status{}, err
	}

	status := Status{
		Active:          true,
		Expired:         sess.IsExpired(time.Now()),
		Provider:        sess.Provider,
		CookieCount:     len(sess.Cookies),
		StorageKeys:     len(sess.Storage),
		RetrievedAt:     &sess.RetrievedAt,
		ExpiresAt:       sess.ExpiresAt,
		ServerExpiresAt: sess.ServerExpiresAt,
		SessionFile:     s.sessionFile,
	}

	if s.validator != nil {
		checkedAt := time.Now().UTC()
		status.Validated = true
		status.CheckedAt = &checkedAt
		if err := s.validator.ValidateSession(ctx); err != nil {
			status.Valid = false
			status.ValidationError = err.Error()
			return status, nil
		}
		status.Valid = true

		if expiredAt, err := s.validator.GetServerExpiredAt(ctx); err == nil {
			status.ServerExpiresAt = &expiredAt
			sess.ServerExpiresAt = &expiredAt
			_ = s.store.Save(ctx, sess) // best-effort persist; non-fatal on save error
		}
	}

	return status, nil
}

func (s *Service) Logout(ctx context.Context) (bool, error) {
	if _, err := s.store.Load(ctx); err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return false, nil
		}

		return false, err
	}

	if err := s.store.Clear(ctx); err != nil {
		return false, err
	}

	return true, nil
}
