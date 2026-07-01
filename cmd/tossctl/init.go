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
	initChoiceOfficial = "Official Open API key only — core supported features only. Most stable (auto token refresh, no browser needed)"
	initChoiceWeb      = "Web session login only — unlocks unofficial features too, but disconnects sooner and needs more frequent renewal"
	initChoiceBoth     = "Both (recommended) — stability + maximum coverage, with fallback if the official API is down"
)

func newInitCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "init",
		Short:       "Interactive onboarding wizard for authentication setup",
		Annotations: map[string]string{"source": "local"},
		Long: "Interactively set up authentication via an official Open API key or a web session.\n" +
			"In non-interactive (CI/AI) environments, prints flag-based command guidance and exits cleanly.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !tui.IsInteractive(os.Stdin, os.Stdout) {
				_, err := fmt.Fprint(cmd.OutOrStdout(),
					"In a non-interactive environment, set up auth with one of these commands:\n"+
						"  Web session:    tossctl auth login\n"+
						"  Official Open API:  tossctl openapi login --key <KEY> --secret <SECRET>\n",
				)
				return err
			}

			choice, err := tui.Select("Select an authentication method", []string{
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
				return fmt.Errorf("unknown choice: %q", choice)
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
		fmt.Fprintf(cmd.OutOrStdout(), "If you don't have an official Open API key yet, apply first: %s\n\n", officialPreApplyURL)
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
		fmt.Fprintln(cmd.OutOrStdout(), "✓ official Open API key saved")
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
