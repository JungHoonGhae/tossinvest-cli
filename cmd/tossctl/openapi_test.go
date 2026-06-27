package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

// ── login: flags-only error paths ──────────────────────────────────────────

func TestOpenAPILoginFlagsOnlyErrorsWhenMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &rootOptions{configDir: dir}

	cases := []struct {
		name string
		args []string
	}{
		{"both missing", []string{"login"}},
		{"key only", []string{"login", "--key", "K"}},
		{"secret only", []string{"login", "--secret", "S"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newOpenAPICmd(opts)
			cmd.SetArgs(tc.args)
			var errBuf bytes.Buffer
			cmd.SetErr(&errBuf)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expected error for args %v", tc.args)
			}
		})
	}
}

// ── login: success — masked output, no secret in output ────────────────────

func TestOpenAPILoginSavesCredentialsAndMasksKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &rootOptions{configDir: dir}

	cmd := newOpenAPICmd(opts)
	cmd.SetArgs([]string{"login", "--key", "tsck_live_9I24L3TIMVgiFfakZJaVLA", "--secret", "super-secret-123"})
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := outBuf.String()

	// Secret must never appear in output.
	if strings.Contains(out, "super-secret-123") {
		t.Fatal("secret must not appear in output")
	}

	// Masked key should appear.
	if !strings.Contains(out, "tsck_live_…aVLA") {
		t.Fatalf("expected masked key in output, got %q", out)
	}

	// Credentials file must exist with 0600 permissions.
	credFile := filepath.Join(dir, "openapi-credentials.json")
	fi, err := os.Stat(credFile)
	if err != nil {
		t.Fatalf("credentials file not created: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("want 0600, got %v", fi.Mode().Perm())
	}
}

// ── logout: removes credential and token files ──────────────────────────────

func TestOpenAPILogoutRemovesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	credFile := filepath.Join(dir, "openapi-credentials.json")
	tokenFile := filepath.Join(dir, "openapi-token.json")

	if err := official.SaveCredentials(credFile, official.Credentials{APIKey: "k", SecretKey: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenFile, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := &rootOptions{configDir: dir}
	cmd := newOpenAPICmd(opts)
	cmd.SetArgs([]string{"logout"})
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(credFile); !os.IsNotExist(err) {
		t.Fatal("credentials file should be deleted after logout")
	}
	if _, err := os.Stat(tokenFile); !os.IsNotExist(err) {
		t.Fatal("token file should be deleted after logout")
	}
}

func TestOpenAPILogoutWhenNoFilesIsHarmless(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &rootOptions{configDir: dir}
	cmd := newOpenAPICmd(opts)
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout with no files should not error, got %v", err)
	}
}

// ── saveOpenAPICredentials seam ─────────────────────────────────────────────

func TestSaveOpenAPICredentials(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")

	if err := saveOpenAPICredentials(path, "apikey-dummy-123456789012", "secret-dummy"); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("want 0600, got %v", fi.Mode().Perm())
	}

	loaded, err := official.LoadCredentials(func(string) string { return "" }, path)
	if err != nil || loaded == nil {
		t.Fatalf("failed to reload saved credentials: %v", err)
	}
	if loaded.APIKey != "apikey-dummy-123456789012" {
		t.Fatalf("wrong key, got %q", loaded.APIKey)
	}
	// SavedAt should be populated.
	if loaded.SavedAt == "" {
		t.Fatal("SavedAt should be set")
	}
}

// ── validateOpenAPICredentials seam (via httptest) ──────────────────────────

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, filepath.Join(t.TempDir(), "token.json")
}

func TestValidateOpenAPICredentialsSuccess(t *testing.T) {
	t.Parallel()
	srv, tokenFile := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/accounts":
			_, _ = w.Write([]byte(`{"result":[]}`))
		default:
			http.NotFound(w, r)
		}
	})

	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("expected OK=true, got %+v", result)
	}
	if result.Message != "ok" {
		t.Fatalf("expected message 'ok', got %q", result.Message)
	}
}

func TestValidateOpenAPICredentialsIPNotAllowed(t *testing.T) {
	t.Parallel()
	srv, tokenFile := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"ip not allowed"}`))
			return
		}
		http.NotFound(w, r)
	})

	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.ErrorKind != "ip_not_allowed" {
		t.Fatalf("expected ip_not_allowed, got %q", result.ErrorKind)
	}
}

func TestValidateOpenAPICredentialsAuthError(t *testing.T) {
	t.Parallel()
	srv, tokenFile := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
			return
		}
		http.NotFound(w, r)
	})

	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.ErrorKind != "auth" {
		t.Fatalf("expected auth, got %q", result.ErrorKind)
	}
}

func TestValidateOpenAPICredentialsRateLimited(t *testing.T) {
	t.Parallel()
	srv, tokenFile := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	})

	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.ErrorKind != "rate_limited" {
		t.Fatalf("expected rate_limited, got %q", result.ErrorKind)
	}
}

func TestValidateOpenAPICredentialsServerError(t *testing.T) {
	t.Parallel()
	srv, tokenFile := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	})

	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.ErrorKind != "server_error" {
		t.Fatalf("expected server_error, got %q", result.ErrorKind)
	}
}

// ── writeProbeResult ─────────────────────────────────────────────────────────

func TestWriteProbeResultJSONSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeProbeResult(&buf, output.FormatJSON, probeResult{OK: true, Message: "ok"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, `"ok": true`) {
		t.Fatalf("expected ok:true in JSON, got %q", got)
	}
	if strings.Contains(got, "error_kind") {
		t.Fatalf("error_kind should be omitted when empty, got %q", got)
	}
}

func TestWriteProbeResultJSONFailure(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeProbeResult(&buf, output.FormatJSON, probeResult{
		OK:        false,
		ErrorKind: "auth",
		Message:   "인증 실패",
	}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, `"ok": false`) {
		t.Fatalf("expected ok:false in JSON, got %q", got)
	}
	if !strings.Contains(got, `"error_kind": "auth"`) {
		t.Fatalf("expected error_kind in JSON, got %q", got)
	}
}

func TestWriteProbeResultTableSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeProbeResult(&buf, output.FormatTable, probeResult{OK: true, Message: "ok"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "✓") {
		t.Fatalf("expected checkmark in table output, got %q", buf.String())
	}
}

func TestWriteProbeResultTableFailure(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writeProbeResult(&buf, output.FormatTable, probeResult{OK: false, Message: "실패"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "✗") {
		t.Fatalf("expected cross in table output, got %q", buf.String())
	}
}
