package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junghoonkye/tossinvest-cli/internal/client"
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

func TestStatusMarksHalfSessionInvalid(t *testing.T) {
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"1","name":"기본계좌","displayName":"기본계좌","type":"BROKERAGE","markets":["US"]}],"primaryKey":"1"}}`))
		case "/api/v3/my-assets/summaries/markets/all/overview":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	validator := client.New(client.Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     sess,
	})

	svc := NewService(
		session.NewFileStore(sessionPath),
		sessionPath,
		Options{Validator: validator},
	)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !status.Validated {
		t.Fatal("expected validated status")
	}
	if status.Valid {
		t.Fatal("expected invalid half-session status")
	}
	if status.ValidationError == "" {
		t.Fatal("expected validation error detail")
	}
	if want := "/api/v3/my-assets/summaries/markets/all/overview"; !strings.Contains(status.ValidationError, want) {
		t.Fatalf("expected validation error to mention %s, got %q", want, status.ValidationError)
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
