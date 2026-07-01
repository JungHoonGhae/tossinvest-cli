package main

import (
	"fmt"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/spf13/cobra"
)

const officialPreApplyURL = "https://corp.tossinvest.com/ko/open-api"

func newInitCmd(opts *rootOptions) *cobra.Command {
	initChoiceOfficial := i18n.T("init.prompt.choiceOfficial")
	initChoiceWeb := i18n.T("init.prompt.choiceWeb")
	initChoiceBoth := i18n.T("init.prompt.choiceBoth")

	return &cobra.Command{
		Use:         "init",
		Short:       i18n.T("init.short"),
		Annotations: map[string]string{"source": "local"},
		Long:        i18n.T("init.long"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !tui.IsInteractive(os.Stdin, os.Stdout) {
				_, err := fmt.Fprint(cmd.OutOrStdout(), i18n.T("init.msg.nonInteractiveGuidance"))
				return err
			}

			choice, err := tui.Select(i18n.T("init.prompt.selectMethod"), []string{
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
		fmt.Fprintf(cmd.OutOrStdout(), i18n.T("init.msg.applyLink"), officialPreApplyURL)
	}

	key, err := tui.Password(i18n.T("init.prompt.apiKey"))
	if err != nil {
		return err
	}
	secret, err := tui.Password(i18n.T("init.prompt.secretKey"))
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
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("init.msg.officialSaved"))
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
