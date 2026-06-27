package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
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
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", "", err
	}
	if opts.configDir != "" {
		return filepath.Join(opts.configDir, "openapi-credentials.json"),
			filepath.Join(opts.configDir, "openapi-token.json"),
			nil
	}
	return paths.CredentialsFile, paths.TokenFile, nil
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
				msg := "저장된 자격증명이 없습니다. `tossctl openapi login --key K --secret S`를 먼저 실행하세요."
				if format == output.FormatJSON {
					return writeProbeResult(cmd.OutOrStdout(), format, probeResult{
						OK:        false,
						ErrorKind: "no_credentials",
						Message:   msg,
					})
				}
				return fmt.Errorf("%s", msg)
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

	cmd.AddCommand(loginCmd, testCmd, logoutCmd)
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
