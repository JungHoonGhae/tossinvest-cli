package main

import (
	"encoding/json"

	"github.com/JungHoonGhae/tossinvest-cli/internal/doctor"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *rootOptions) *cobra.Command {
	var reportMode bool

	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       i18n.T("doctor.short"),
		Annotations: map[string]string{"source": "local"},
		Long:        i18n.T("doctor.long"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			configStatus, err := app.configService.Status(cmd.Context())
			if err != nil {
				return err
			}

			svc := doctor.NewService(
				app.paths,
				configStatus,
				app.loginConfig,
				app.authService,
			)

			if reportMode {
				report, err := svc.RunReport(cmd.Context(), app.client)
				if err != nil {
					return err
				}
				// Graceful Open API summary — never fails doctor.
				report.OpenAPISummary = computeDoctorOpenAPISummary(cmd.Context(), opts, app)
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			report, err := svc.Run(cmd.Context())
			if err != nil {
				return err
			}

			// Graceful Open API summary — never fails doctor.
			report.OpenAPISummary = computeDoctorOpenAPISummary(cmd.Context(), opts, app)

			return output.WriteDoctorReport(cmd.OutOrStdout(), app.format, report)
		},
	}

	cmd.Flags().BoolVar(&reportMode, "report", false, "Emit a JSON diagnostic bundle suitable for bug reports (runs endpoint probes, redacts paths)")

	return cmd
}
