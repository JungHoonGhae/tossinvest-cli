package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

type fakeLoginRunner struct {
	result *LoginResult
	err    error
}

type fakeSessionValidator struct {
	err error
}

func (r fakeLoginRunner) Login(context.Context, LoginConfig) (*LoginResult, error) {
	return r.result, r.err
}

func (v fakeSessionValidator) ValidateSession(context.Context) error {
	return v.err
}

func (v fakeSessionValidator) GetServerExpiredAt(context.Context) (time.Time, error) {
	return time.Time{}, errors.New("not implemented in fake")
}

func TestLoginImportsHelperStorageState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "session.json")
	storageStatePath := filepath.Join(tmpDir, "playwright-state.json")

	state := map[string]any{
		"cookies": []map[string]string{
			{"name": "SESSION", "value": "session-token"},
			{"name": "XSRF-TOKEN", "value": "xsrf-token"},
		},
		"origins": []map[string]any{
			{
				"origin": "https://www.tossinvest.com",
				"localStorage": []map[string]string{
					{"name": "WTS-DEVICE-ID", "value": "device-123"},
					{"name": "qr-tabId", "value": "browser-tab-login"},
				},
			},
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(storageStatePath, data, 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	svc := NewService(
		session.NewFileStore(sessionPath),
		sessionPath,
		Options{
			Runner: fakeLoginRunner{
				result: &LoginResult{
					Status:           "ok",
					StorageStatePath: storageStatePath,
				},
			},
		},
	)

	sess, err := svc.Login(context.Background())
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if sess.Cookies["SESSION"] != "session-token" {
		t.Fatalf("unexpected session cookie: %q", sess.Cookies["SESSION"])
	}
	if sess.Headers["X-XSRF-TOKEN"] != "xsrf-token" {
		t.Fatalf("unexpected xsrf header: %q", sess.Headers["X-XSRF-TOKEN"])
	}
	if sess.Headers["Browser-Tab-Id"] != "browser-tab-login" {
		t.Fatalf("unexpected browser-tab-id header: %q", sess.Headers["Browser-Tab-Id"])
	}
	if sess.Storage["localStorage:WTS-DEVICE-ID"] != "device-123" {
		t.Fatalf("unexpected storage value: %q", sess.Storage["localStorage:WTS-DEVICE-ID"])
	}

	stored, err := session.NewFileStore(sessionPath).Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if stored.Provider != "playwright-storage-state" {
		t.Fatalf("unexpected provider: %s", stored.Provider)
	}
}

func TestStatusIncludesValidationResult(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "session.json")
	sess := &session.Session{
		Provider:    "playwright-storage-state",
		Cookies:     map[string]string{"SESSION": "session-token"},
		Headers:     map[string]string{"X-XSRF-TOKEN": "xsrf-token"},
		Storage:     map[string]string{"localStorage:WTS-DEVICE-ID": "device-123"},
		RetrievedAt: mustTime(t, "2026-03-11T05:00:00Z"),
	}
	if err := session.NewFileStore(sessionPath).Save(context.Background(), sess); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	svc := NewService(
		session.NewFileStore(sessionPath),
		sessionPath,
		Options{Validator: fakeSessionValidator{}},
	)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !status.Validated {
		t.Fatal("expected validated status")
	}
	if !status.Valid {
		t.Fatal("expected valid session")
	}
	if status.CheckedAt == nil {
		t.Fatal("expected checked timestamp")
	}
}

func TestStatusCapturesValidationError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "session.json")
	sess := &session.Session{
		Provider:    "playwright-storage-state",
		Cookies:     map[string]string{"SESSION": "session-token"},
		RetrievedAt: mustTime(t, "2026-03-11T05:00:00Z"),
	}
	if err := session.NewFileStore(sessionPath).Save(context.Background(), sess); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	svc := NewService(
		session.NewFileStore(sessionPath),
		sessionPath,
		Options{Validator: fakeSessionValidator{err: errors.New("session rejected")}},
	)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !status.Validated {
		t.Fatal("expected validated status")
	}
	if status.Valid {
		t.Fatal("expected invalid session")
	}
	if status.ValidationError != "session rejected" {
		t.Fatalf("unexpected validation error: %q", status.ValidationError)
	}
}

type fakeServerExpiredAtValidator struct {
	validateErr error
	expiredAt   time.Time
	expiredErr  error
}

func (v fakeServerExpiredAtValidator) ValidateSession(context.Context) error {
	return v.validateErr
}

func (v fakeServerExpiredAtValidator) GetServerExpiredAt(context.Context) (time.Time, error) {
	return v.expiredAt, v.expiredErr
}

func TestStatusRefreshesServerExpiresAt(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	if err := session.NewFileStore(path).Save(context.Background(), &session.Session{
		Provider:    "playwright-storage-state",
		Cookies:     map[string]string{"SESSION": "s"},
		RetrievedAt: mustTime(t, "2026-05-04T13:00:00Z"),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := mustTime(t, "2026-05-13T07:03:20+09:00")
	svc := NewService(
		session.NewFileStore(path),
		path,
		Options{Validator: fakeServerExpiredAtValidator{expiredAt: want}},
	)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ServerExpiresAt == nil || !status.ServerExpiresAt.Equal(want) {
		t.Fatalf("ServerExpiresAt = %v, want %v", status.ServerExpiresAt, &want)
	}

	// And the value should be persisted back to disk.
	stored, err := session.NewFileStore(path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.ServerExpiresAt == nil || !stored.ServerExpiresAt.Equal(want) {
		t.Fatalf("disk ServerExpiresAt not persisted")
	}
}

func TestDefaultLoginConfigHonorsExplicitPythonOverride(t *testing.T) {
	t.Setenv("TOSSCTL_AUTH_HELPER_PYTHON", "/custom/path/to/python")
	t.Setenv("UV_TOOL_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")

	cfg := DefaultLoginConfig(t.TempDir())
	if cfg.PythonBin != "/custom/path/to/python" {
		t.Fatalf("expected explicit override to win, got %q", cfg.PythonBin)
	}
}

func TestDefaultLoginConfigPrefersUvToolPython(t *testing.T) {
	toolDir := t.TempDir()
	// On Windows exec.LookPath only treats files with a PATHEXT extension as
	// runnable, and uv installs the interpreter under Scripts\python.exe rather
	// than bin/python, so mirror that layout to match resolveDefaultPythonBin.
	rel := filepath.Join("tossctl-auth-helper", "bin", "python")
	if runtime.GOOS == "windows" {
		rel = filepath.Join("tossctl-auth-helper", "Scripts", "python.exe")
	}
	pythonPath := filepath.Join(toolDir, rel)
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write python stub: %v", err)
	}

	t.Setenv("TOSSCTL_AUTH_HELPER_PYTHON", "")
	t.Setenv("UV_TOOL_DIR", toolDir)

	cfg := DefaultLoginConfig(t.TempDir())
	if cfg.PythonBin != pythonPath {
		t.Fatalf("expected uv tool python %q, got %q", pythonPath, cfg.PythonBin)
	}
}

func TestDefaultLoginConfigFallsBackToPython3(t *testing.T) {
	t.Setenv("TOSSCTL_AUTH_HELPER_PYTHON", "")
	t.Setenv("UV_TOOL_DIR", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("APPDATA", "")
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultLoginConfig(t.TempDir())

	want := "python3"
	if runtime.GOOS == "windows" {
		want = "python"
	}
	if cfg.PythonBin != want {
		t.Fatalf("expected %q fallback, got %q", want, cfg.PythonBin)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	return parsed
}
