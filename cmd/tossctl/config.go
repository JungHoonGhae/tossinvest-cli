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
		newConfigExperimentalCmd(opts),
	)

	return cmd
}

func newConfigExperimentalCmd(opts *rootOptions) *cobra.Command {
	group := &cobra.Command{Use: "experimental", Short: "Manage opt-in rolling features"}
	var enable, disable bool
	paper := &cobra.Command{
		Use: "paper-trading", Short: "Enable or disable the rolling paper-trading surface",
		Annotations: mutationAnnotations("local", "system", "preference", "reversible"),
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if enable == disable {
				return fmt.Errorf("set exactly one of --enable or --disable")
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if err := app.configService.SetExperimentalPaperTrading(cmd.Context(), enable); err != nil {
				return err
			}
			status, err := app.configService.Status(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteConfigStatus(cmd.OutOrStdout(), app.format, status)
		},
	}
	paper.Flags().BoolVar(&enable, "enable", false, "Opt in to paper-trading commands and operations")
	paper.Flags().BoolVar(&disable, "disable", false, "Hide and block paper-trading commands and operations")
	group.AddCommand(paper)
	return group
}
