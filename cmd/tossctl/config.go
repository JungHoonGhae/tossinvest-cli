package main

import (
	"encoding/json"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: i18n.T("config.short"),
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:         "show",
			Short:       i18n.T("config.show.short"),
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				status, err := app.configService.Status(cmd.Context())
				if err != nil {
					return err
				}

				return output.WriteConfigStatus(cmd.OutOrStdout(), app.format, status)
			},
		},
		&cobra.Command{
			Use:         "init",
			Short:       i18n.T("config.init.short"),
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				result, err := app.configService.Init(cmd.Context())
				if err != nil {
					return err
				}

				if app.format == output.FormatJSON {
					encoder := json.NewEncoder(cmd.OutOrStdout())
					encoder.SetIndent("", "  ")
					return encoder.Encode(result)
				}
				if app.format == output.FormatCSV {
					return fmt.Errorf("csv output is not supported for config init")
				}
				if result.Created {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created config file: %s\n", result.Status.ConfigFile)
				} else {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Config file already exists: %s\n", result.Status.ConfigFile)
				}
				if err != nil {
					return err
				}
				return output.WriteConfigStatus(cmd.OutOrStdout(), output.FormatTable, result.Status)
			},
		},
	)

	return cmd
}
