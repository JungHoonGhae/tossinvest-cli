package main

import (
	"fmt"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/spf13/cobra"
)

const officialPreApplyURL = "https://corp.tossinvest.com/ko/open-api"

const (
	initChoiceOfficial = "공식 Open API 키만 — 핵심·공식 지원 기능만. 가장 안정적(토큰 자동 갱신, 브라우저 불필요)"
	initChoiceWeb      = "웹 세션 로그인만 — 고유·비공식 기능까지. 단 더 빨리 끊기고 갱신 주기 짧음"
	initChoiceBoth     = "둘 다 (권장) — 안정성 + 최대 범위, 공식 장애 시 폴백"
)

func newInitCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "온보딩 위저드 — 인증 방식을 대화형으로 설정",
		Long: "인터랙티브 위저드로 공식 Open API 키 또는 웹 세션 인증을 설정합니다.\n" +
			"비대화형(CI/AI) 환경에서는 플래그 기반 명령어 안내를 출력하고 정상 종료합니다.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !tui.IsInteractive(os.Stdin, os.Stdout) {
				_, err := fmt.Fprint(cmd.OutOrStdout(),
					"비대화형 환경에서는 아래 명령으로 인증을 설정하세요:\n"+
						"  웹 세션:        tossctl auth login\n"+
						"  공식 Open API:  tossctl openapi login --key <KEY> --secret <SECRET>\n",
				)
				return err
			}

			choice, err := tui.Select("인증 방식을 선택하세요", []string{
				initChoiceOfficial,
				initChoiceWeb,
				initChoiceBoth,
			})
			if err != nil {
				return err
			}

			switch choice {
			case initChoiceOfficial:
				return runInitOfficialFlow(cmd, opts)
			case initChoiceWeb:
				return runInitWebFlow(cmd, opts)
			case initChoiceBoth:
				if err := runInitWebFlow(cmd, opts); err != nil {
					return err
				}
				return runInitOfficialFlow(cmd, opts)
			default:
				return fmt.Errorf("알 수 없는 선택: %q", choice)
			}
		},
	}
}

// runInitOfficialFlow handles the official Open API key setup sub-flow.
// If no credentials exist yet, it prints the pre-application link before prompting.
// Secrets are never printed or logged.
func runInitOfficialFlow(cmd *cobra.Command, opts *rootOptions) error {
	credFile, tokenFile, err := resolveOpenAPIPaths(opts)
	if err != nil {
		return err
	}

	// Show pre-application link when the user has no key yet.
	existing, _ := official.LoadCredentials(os.Getenv, credFile)
	if existing == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "공식 Open API 키가 없으시면 먼저 신청하세요: %s\n\n", officialPreApplyURL)
	}

	key, err := tui.Password("API Key")
	if err != nil {
		return err
	}
	secret, err := tui.Password("Secret Key")
	if err != nil {
		return err
	}

	creds := official.Credentials{APIKey: key, SecretKey: secret}
	result, err := validateOpenAPICredentials(cmd.Context(), creds, tokenFile)
	if err != nil {
		return err
	}

	if result.OK {
		if err := saveOpenAPICredentials(credFile, key, secret); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ 공식 Open API 키 저장 완료")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✗ %s\n", result.Message)
	}
	return nil
}

// runInitWebFlow triggers the existing browser-assisted login flow.
func runInitWebFlow(cmd *cobra.Command, opts *rootOptions) error {
	app, err := newAppContext(opts)
	if err != nil {
		return err
	}
	_, err = app.authService.LoginWith(cmd.Context(), app.loginConfig)
	return err
}
