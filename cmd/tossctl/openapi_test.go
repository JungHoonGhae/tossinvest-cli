package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
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

func TestValidateOpenAPICredentialsTransportError(t *testing.T) {
	t.Parallel()
	// Spin up a server only to capture a reachable URL + client, then close it
	// before the request so the token-exchange dial is refused → ErrTransport.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	client := srv.Client()
	srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "token.json")
	creds := official.Credentials{APIKey: "k", SecretKey: "s"}
	result, err := validateOpenAPICredentials(context.Background(), creds, tokenFile,
		official.WithBaseURL(url),
		official.WithHTTPClient(client),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if result.ErrorKind != "transport_error" {
		t.Fatalf("expected transport_error, got %q", result.ErrorKind)
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

// ── openapi status: no credentials → guidance + exit 0 ──────────────────────

func TestOpenAPIStatusNoCredentials(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &rootOptions{configDir: dir, outputFormat: "table"}

	cmd := newOpenAPICmd(opts)
	cmd.SetArgs([]string{"status"})
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Should succeed (exit 0) even with no credentials.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error with no credentials, got %v", err)
	}

	out := outBuf.String()
	// Should mention setup guidance.
	if !strings.Contains(out, "미설정") && !strings.Contains(out, "login") && !strings.Contains(out, "init") {
		t.Fatalf("expected setup guidance in output, got %q", out)
	}
}

// ── buildStatusReport: active key, expiry in 10 days → D-10 warning ─────────

func TestBuildStatusReportExpiryWarning(t *testing.T) {
	t.Parallel()
	now := time.Now()
	expiresAt := now.Add(10 * 24 * time.Hour)
	issuedAt := now.Add(-30 * 24 * time.Hour)

	keyInfo := tossclient.OpenAPIClientInfo{
		Status:    "ACTIVE",
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	in := statusInputs{
		creds:       &official.Credentials{APIKey: "tsck_live_9I24L3TIMVtestZJaVLA"},
		credsSource: "file",
		keyInfo:     &keyInfo,
		allowedIPs:  []string{"203.0.113.1", "203.0.113.2"},
		probe:       probeResult{OK: true, Message: "ok"},
		prefer:      "auto",
		fallback:    true,
	}

	r := buildStatusReport(in)

	if !r.CredentialsConfigured {
		t.Error("expected CredentialsConfigured=true")
	}
	if r.KeyExpiryWarning == "" {
		t.Errorf("expected expiry warning (D-10), got empty")
	}
	if !strings.Contains(r.KeyExpiryWarning, "D-") {
		t.Errorf("expected D-NN in warning, got %q", r.KeyExpiryWarning)
	}
	if !r.KeyActive {
		t.Error("expected KeyActive=true")
	}
	if r.ConnectionOK != true {
		t.Error("expected ConnectionOK=true")
	}
	if r.CurrentIPStatus != "현재 IP 허용됨" {
		t.Errorf("expected '현재 IP 허용됨', got %q", r.CurrentIPStatus)
	}
}

// ── buildStatusReport: ip_not_allowed → "add IP" instruction appears ─────────

func TestBuildStatusReportIPNotAllowed(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	issuedAt := time.Now().Add(-30 * 24 * time.Hour)

	keyInfo := tossclient.OpenAPIClientInfo{
		Status:    "ACTIVE",
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	in := statusInputs{
		creds:       &official.Credentials{APIKey: "tsck_live_9I24L3TIMVtestZJaVLA"},
		credsSource: "file",
		keyInfo:     &keyInfo,
		allowedIPs:  []string{"203.0.113.10"},
		probe: probeResult{
			OK:        false,
			ErrorKind: "ip_not_allowed",
			Message:   "이 IP에서 API 접근이 허용되지 않습니다.",
		},
		prefer:   "auto",
		fallback: true,
	}

	r := buildStatusReport(in)

	// Current IP status should contain "add IP" guidance.
	if !strings.Contains(r.CurrentIPStatus, "허용") && !strings.Contains(r.CurrentIPStatus, "추가") {
		t.Errorf("expected IP add instruction in CurrentIPStatus, got %q", r.CurrentIPStatus)
	}
	if r.ConnectionOK {
		t.Error("expected ConnectionOK=false")
	}
	if !strings.Contains(r.ConnectionStatus, "IP") {
		t.Errorf("expected IP mention in ConnectionStatus, got %q", r.ConnectionStatus)
	}
}

// ── buildStatusReport: WTS metadata error → graceful degrade, probe shown ────

func TestBuildStatusReportGracefulDegrade(t *testing.T) {
	t.Parallel()

	in := statusInputs{
		creds:         &official.Credentials{APIKey: "tsck_live_9I24L3TIMVtestZJaVLA"},
		credsSource:   "file",
		keyInfo:       nil, // WTS call failed
		keyInfoErr:    context.DeadlineExceeded,
		allowedIPs:    nil,
		allowedIPsErr: context.DeadlineExceeded,
		probe: probeResult{
			OK:        false,
			ErrorKind: "auth",
			Message:   "인증 실패",
		},
		prefer:   "auto",
		fallback: true,
	}

	r := buildStatusReport(in)

	// WTS metadata error should be surfaced.
	if r.KeyMetaError == "" {
		t.Error("expected KeyMetaError to be set on WTS failure")
	}
	// Probe result must still be shown.
	if r.ConnectionOK {
		t.Error("expected ConnectionOK=false")
	}
	if r.ConnectionStatus == "" {
		t.Error("expected ConnectionStatus to be populated even on WTS degrade")
	}
	if r.ConnectionDetail == "" {
		t.Error("expected ConnectionDetail to be populated")
	}
}

// ── buildStatusReport: --output json has expected fields ─────────────────────

func TestBuildStatusReportJSONFields(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	issuedAt := time.Now().Add(-10 * 24 * time.Hour)

	keyInfo := tossclient.OpenAPIClientInfo{
		Status:    "ACTIVE",
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	in := statusInputs{
		creds:       &official.Credentials{APIKey: "tsck_live_9I24L3TIMVtestZJaVLA"},
		credsSource: "env",
		keyInfo:     &keyInfo,
		allowedIPs:  []string{"203.0.113.5"},
		probe:       probeResult{OK: true, Message: "ok"},
		prefer:      "official",
		fallback:    false,
	}

	r := buildStatusReport(in)

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	got := string(data)

	requiredKeys := []string{
		"credentials_configured",
		"credentials_source",
		"masked_key",
		"connection_ok",
		"connection_status",
		"routing_prefer",
		"routing_fallback",
		"eligible_ops_count",
		"current_ip_status",
		"token_status",
	}
	for _, key := range requiredKeys {
		if !strings.Contains(got, `"`+key+`"`) {
			t.Errorf("expected JSON key %q in output, got: %s", key, got)
		}
	}

	// credentials_source should be "env"
	if !strings.Contains(got, `"env"`) {
		t.Errorf("expected env source in JSON, got %s", got)
	}
	// Masked key must not contain the full key
	if strings.Contains(got, "9I24L3TIMVtestZJaVLA") {
		t.Error("full API key must not appear in JSON output")
	}
}
