package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

// probeResult is the structured outcome of validating Open API credentials.
// It is serialised as JSON by `openapi test --output json`.
type probeResult struct {
	OK        bool   `json:"ok"`
	ErrorKind string `json:"error_kind,omitempty"`
	Message   string `json:"message"`
}

// saveOpenAPICredentials is a reusable seam (consumed by Task 18 wizard).
// It writes key/secret to path with 0600 permissions via official.SaveCredentials.
// The secret is never returned or logged by the caller.
func saveOpenAPICredentials(path string, key, secret string) error {
	return official.SaveCredentials(path, official.Credentials{
		APIKey:    key,
		SecretKey: secret,
		SavedAt:   time.Now().UTC().Format(time.RFC3339),
	})
}

// validateOpenAPICredentials is a reusable seam (consumed by Task 18 wizard).
// It probes the official API using Accounts() and classifies any error into a
// probeResult. opts allows overriding base URL and HTTP client for tests.
func validateOpenAPICredentials(ctx context.Context, creds official.Credentials, tokenFile string, opts ...official.Option) (probeResult, error) {
	client := official.New(creds, tokenFile, opts...)
	_, err := client.Accounts(ctx)
	if err == nil {
		return probeResult{OK: true, Message: "ok"}, nil
	}
	switch {
	case errors.Is(err, official.ErrIPNotAllowed):
		return probeResult{
			OK:        false,
			ErrorKind: "ip_not_allowed",
			Message:   "이 IP에서 API 접근이 허용되지 않습니다. Toss 개발자 포털에서 IP를 허용 목록에 추가해주세요.",
		}, nil
	case errors.Is(err, official.ErrAuth):
		return probeResult{
			OK:        false,
			ErrorKind: "auth",
			Message:   "인증 실패: API 키와 시크릿을 확인해주세요.",
		}, nil
	case errors.Is(err, official.ErrRateLimited):
		return probeResult{
			OK:        false,
			ErrorKind: "rate_limited",
			Message:   "API 호출 한도를 초과했습니다. 잠시 후 다시 시도해주세요.",
		}, nil
	case errors.Is(err, official.ErrServer):
		return probeResult{
			OK:        false,
			ErrorKind: "server_error",
			Message:   "서버 오류가 발생했습니다. 잠시 후 다시 시도해주세요.",
		}, nil
	case errors.Is(err, official.ErrTransport):
		return probeResult{
			OK:        false,
			ErrorKind: "transport_error",
			Message:   "네트워크 연결에 문제가 있습니다. 인터넷 연결을 확인해주세요.",
		}, nil
	default:
		return probeResult{
			OK:        false,
			ErrorKind: "unknown",
			Message:   err.Error(),
		}, nil
	}
}

// resolveOpenAPIPaths returns the credentials file and token file paths,
// honouring --config-dir override. When configDir is set, both files are placed
// inside it so tests can control them via a single temp directory.
func resolveOpenAPIPaths(opts *rootOptions) (credFile, tokenFile string, err error) {
	// Honour the override first so a DefaultPaths() failure never blocks tests
	// (or any caller) that already pin a config directory.
	if opts.configDir != "" {
		return filepath.Join(opts.configDir, "openapi-credentials.json"),
			filepath.Join(opts.configDir, "openapi-token.json"),
			nil
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", "", err
	}
	return paths.CredentialsFile, paths.TokenFile, nil
}

// officialEligibleOpsCount is the number of CLI operations the official Toss
// Open API can serve (10 reads + 3 order writes defined in hybrid/client.go and
// hybrid/broker.go). Hardcoded intentionally; update when new ops are wired.
const officialEligibleOpsCount = 13

// statusInputs holds all the pieces gathered for the status report.
// Every pointer/error field has a graceful-degrade path so the caller
// never needs to abort early.
type statusInputs struct {
	creds       *official.Credentials
	credsSource string // "env" | "file"

	keyInfo    *tossclient.OpenAPIClientInfo // nil when WTS call fails
	keyInfoErr error

	allowedIPs    []string
	allowedIPsErr error

	tokenExpiresAt *time.Time // nil when token file absent/unreadable/expired

	probe probeResult

	prefer   string
	fallback bool
}

// statusReport is the assembled output of buildStatusReport.
// All fields are exported for JSON marshalling.
type statusReport struct {
	CredentialsConfigured bool   `json:"credentials_configured"`
	CredentialsSource     string `json:"credentials_source,omitempty"`
	MaskedKey             string `json:"masked_key,omitempty"`

	KeyActive        bool   `json:"key_active"`
	KeyStatus        string `json:"key_status,omitempty"`
	KeyIssuedAt      string `json:"key_issued_at,omitempty"`
	KeyExpiresAt     string `json:"key_expires_at,omitempty"`
	KeyExpiryWarning string `json:"key_expiry_warning,omitempty"`
	KeyMetaError     string `json:"key_meta_error,omitempty"`

	AllowedIPs    []string `json:"allowed_ips"`
	AllowedIPsErr string   `json:"allowed_ips_error,omitempty"`

	CurrentIPStatus string `json:"current_ip_status"`

	TokenStatus string `json:"token_status"`

	ConnectionOK     bool   `json:"connection_ok"`
	ConnectionStatus string `json:"connection_status"`
	ConnectionDetail string `json:"connection_detail"`

	RoutingPrefer    string `json:"routing_prefer"`
	RoutingFallback  bool   `json:"routing_fallback"`
	EligibleOpsCount int    `json:"eligible_ops_count"`
}

// buildStatusReport is a pure function that assembles the status dashboard from
// pre-gathered pieces. No I/O; safe to call in tests with fake inputs.
func buildStatusReport(in statusInputs) statusReport {
	r := statusReport{
		CredentialsConfigured: in.creds != nil,
		AllowedIPs:            in.allowedIPs,
		RoutingPrefer:         in.prefer,
		RoutingFallback:       in.fallback,
		EligibleOpsCount:      officialEligibleOpsCount,
		ConnectionOK:          in.probe.OK,
	}

	if in.creds != nil {
		r.CredentialsSource = in.credsSource
		r.MaskedKey = in.creds.MaskedKey()
	}

	// Key metadata (from WTS — graceful degrade on error).
	if in.keyInfoErr != nil {
		r.KeyMetaError = "키 메타 조회 실패(웹세션 필요)"
	} else if in.keyInfo != nil {
		r.KeyActive = in.keyInfo.Active
		if in.keyInfo.Active {
			r.KeyStatus = "활성"
		} else {
			r.KeyStatus = "비활성"
		}
		if !in.keyInfo.IssuedAt.IsZero() {
			r.KeyIssuedAt = in.keyInfo.IssuedAt.Format("2006-01-02")
		}
		if !in.keyInfo.ExpiresAt.IsZero() {
			r.KeyExpiresAt = in.keyInfo.ExpiresAt.Format("2006-01-02")
			remaining := time.Until(in.keyInfo.ExpiresAt)
			days := int(remaining.Hours() / 24)
			if days >= 0 && days <= 30 {
				r.KeyExpiryWarning = fmt.Sprintf("⚠ 만료 임박 (D-%d)", days)
			}
		}
	}

	// Allowed IPs error (graceful degrade).
	if in.allowedIPsErr != nil {
		r.AllowedIPsErr = "IP 목록 조회 실패(웹세션 필요)"
	}

	// Access token validity (best-effort).
	if in.tokenExpiresAt == nil {
		r.TokenStatus = "없음/만료"
	} else if time.Now().Before(*in.tokenExpiresAt) {
		r.TokenStatus = fmt.Sprintf("유효 (만료 %s)", in.tokenExpiresAt.Format("15:04"))
	} else {
		r.TokenStatus = "만료됨"
	}

	// Connection + current IP (live probe is ground truth).
	if in.probe.OK {
		r.ConnectionStatus = "✅ 정상"
		r.ConnectionDetail = in.probe.Message
		r.CurrentIPStatus = "현재 IP 허용됨"
	} else {
		r.ConnectionDetail = in.probe.Message
		switch in.probe.ErrorKind {
		case "ip_not_allowed":
			r.ConnectionStatus = "❌ IP 미허용"
			r.CurrentIPStatus = "❌ 현재 공인 IP가 허용목록에 없음 — 토스 설정 > Open API > 허용 IP에 추가"
		case "auth":
			r.ConnectionStatus = "❌ 인증실패"
			r.CurrentIPStatus = "알 수 없음"
		case "rate_limited":
			r.ConnectionStatus = "❌ rate limited"
			r.CurrentIPStatus = "알 수 없음"
		case "server_error":
			r.ConnectionStatus = "❌ 서버오류"
			r.CurrentIPStatus = "알 수 없음"
		case "transport_error":
			r.ConnectionStatus = "❌ 네트워크"
			r.CurrentIPStatus = "알 수 없음"
		default:
			r.ConnectionStatus = "❌ 오류"
			r.CurrentIPStatus = "알 수 없음"
		}
	}

	return r
}

// renderStatusReport writes the dashboard to w in the requested format.
func renderStatusReport(w io.Writer, format output.Format, r statusReport) error {
	if format == output.FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	// Table format — section-by-section.
	writeLine := func(label, value string) {
		fmt.Fprintf(w, "  %-22s %s\n", label, value)
	}

	fmt.Fprintln(w, "[ 자격증명 ]")
	writeLine("설정 여부:", boolLabel(r.CredentialsConfigured, "설정됨", "미설정"))
	if r.CredentialsConfigured {
		writeLine("소스:", r.CredentialsSource)
		writeLine("키:", r.MaskedKey)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "[ 키 상태 (WTS) ]")
	if r.KeyMetaError != "" {
		writeLine("조회 결과:", r.KeyMetaError)
	} else {
		writeLine("상태:", r.KeyStatus)
		if r.KeyIssuedAt != "" {
			writeLine("발급일:", r.KeyIssuedAt)
		}
		if r.KeyExpiresAt != "" {
			writeLine("만료일:", r.KeyExpiresAt)
		}
		if r.KeyExpiryWarning != "" {
			writeLine("경고:", r.KeyExpiryWarning)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "[ 허용 IP ]")
	if r.AllowedIPsErr != "" {
		writeLine("조회 결과:", r.AllowedIPsErr)
	} else if len(r.AllowedIPs) == 0 {
		writeLine("목록:", "(없음)")
	} else {
		writeLine("목록:", strings.Join(r.AllowedIPs, ", "))
	}
	writeLine("현재 IP:", r.CurrentIPStatus)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "[ 액세스 토큰 ]")
	writeLine("상태:", r.TokenStatus)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "[ 연결 (라이브 프로브) ]")
	writeLine("결과:", r.ConnectionStatus)
	if r.ConnectionDetail != "" && r.ConnectionDetail != "ok" {
		writeLine("상세:", r.ConnectionDetail)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "[ 라우팅 ]")
	writeLine("prefer:", r.RoutingPrefer)
	writeLine("fallback:", boolLabel(r.RoutingFallback, "활성화", "비활성화"))
	writeLine("공식 API 지원 ops:", fmt.Sprintf("%d개", r.EligibleOpsCount))

	return nil
}

func boolLabel(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}

// readTokenExpiry reads the expires_at field from the cached token file.
// Returns nil (no error) if the file does not exist or is unreadable.
func readTokenExpiry(tokenFile string) *time.Time {
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil
	}
	var tok struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &tok); err != nil || tok.ExpiresAt.IsZero() {
		return nil
	}
	return &tok.ExpiresAt
}

func newOpenAPICmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "Manage Toss Open API (official) credentials",
	}

	// ── login ──────────────────────────────────────────────────────────────
	var (
		loginKey    string
		loginSecret string
	)
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Save Open API credentials (non-interactive; --key and --secret required)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if loginKey == "" || loginSecret == "" {
				return fmt.Errorf("--key and --secret are both required; or run `tossctl init` wizard")
			}
			credFile, _, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}
			if err := saveOpenAPICredentials(credFile, loginKey, loginSecret); err != nil {
				return err
			}
			masked := official.Credentials{APIKey: loginKey}.MaskedKey()
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "저장됨: %s\n파일: %s\n", masked, credFile)
			return err
		},
	}
	loginCmd.Flags().StringVar(&loginKey, "key", "", "Toss Open API key")
	loginCmd.Flags().StringVar(&loginSecret, "secret", "", "Toss Open API secret (never printed)")

	// ── test ───────────────────────────────────────────────────────────────
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test saved Open API credentials against the live API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseFormat(opts.outputFormat)
			if err != nil {
				return err
			}
			credFile, tokenFile, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}
			creds, err := official.LoadCredentials(os.Getenv, credFile)
			if err != nil {
				return err
			}
			if creds == nil {
				return writeProbeResult(cmd.OutOrStdout(), format, probeResult{
					OK:        false,
					ErrorKind: "no_credentials",
					Message:   "저장된 자격증명이 없습니다. `tossctl openapi login --key K --secret S`를 먼저 실행하세요.",
				})
			}
			result, err := validateOpenAPICredentials(cmd.Context(), *creds, tokenFile)
			if err != nil {
				return err
			}
			return writeProbeResult(cmd.OutOrStdout(), format, result)
		},
	}

	// ── logout ─────────────────────────────────────────────────────────────
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove saved Open API credentials and token cache",
		RunE: func(cmd *cobra.Command, _ []string) error {
			credFile, tokenFile, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}
			if err := official.DeleteCredentials(credFile); err != nil {
				return err
			}
			// best-effort: remove token cache (may not exist)
			_ = os.Remove(tokenFile)
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "자격증명이 삭제되었습니다\n파일: %s\n", credFile)
			return err
		},
	}

	// ── status ─────────────────────────────────────────────────────────────
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show Open API diagnostic dashboard (credentials, key status, IPs, connection)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseFormat(opts.outputFormat)
			if err != nil {
				return err
			}

			credFile, tokenFile, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}

			creds, err := official.LoadCredentials(os.Getenv, credFile)
			if err != nil {
				return err
			}

			// No credentials: print guidance and exit cleanly.
			if creds == nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(),
					"자격증명 미설정 → `tossctl openapi login` 또는 `tossctl init`")
				return nil
			}

			// Determine credentials source.
			credsSource := "file"
			if os.Getenv("TOSSCTL_OPENAPI_KEY") != "" && os.Getenv("TOSSCTL_OPENAPI_SECRET") != "" {
				credsSource = "env"
			}

			// Gather WTS key metadata (graceful degrade on session/network errors).
			var keyInfo *tossclient.OpenAPIClientInfo
			var keyInfoErr error
			var allowedIPs []string
			var allowedIPsErr error

			if app, appErr := newAppContext(opts); appErr == nil {
				info, err := app.client.OpenAPIClientInfo(cmd.Context())
				if err != nil {
					keyInfoErr = err
				} else {
					keyInfo = &info
				}
				ips, err := app.client.OpenAPIAllowedIPs(cmd.Context())
				if err != nil {
					allowedIPsErr = err
				} else {
					allowedIPs = ips
				}
			} else {
				keyInfoErr = appErr
				allowedIPsErr = appErr
			}

			// Read access token expiry (best-effort).
			tokenExpiry := readTokenExpiry(tokenFile)

			// Live probe: ground truth for "is it working".
			probe, _ := validateOpenAPICredentials(cmd.Context(), *creds, tokenFile)

			// Read config for routing info.
			prefer := "auto"
			fallback := true
			if app, appErr := newAppContext(opts); appErr == nil {
				prefer = app.config.OpenAPI.Prefer
				fallback = app.config.OpenAPI.Fallback
			}

			report := buildStatusReport(statusInputs{
				creds:          creds,
				credsSource:    credsSource,
				keyInfo:        keyInfo,
				keyInfoErr:     keyInfoErr,
				allowedIPs:     allowedIPs,
				allowedIPsErr:  allowedIPsErr,
				tokenExpiresAt: tokenExpiry,
				probe:          probe,
				prefer:         prefer,
				fallback:       fallback,
			})

			return renderStatusReport(cmd.OutOrStdout(), format, report)
		},
	}

	cmd.AddCommand(loginCmd, testCmd, logoutCmd, statusCmd)
	return cmd
}

func writeProbeResult(w io.Writer, format output.Format, result probeResult) error {
	switch format {
	case output.FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		if result.OK {
			_, err := fmt.Fprintf(w, "✓ %s\n", result.Message)
			return err
		}
		_, err := fmt.Fprintf(w, "✗ %s\n", result.Message)
		return err
	}
}
