package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newPortfolioCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Read portfolio and holdings data",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:         "positions",
			Short:       "List current positions",
			Annotations: map[string]string{"source": "both"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				positions, err := app.client.ListPositions(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WritePositions(cmd.OutOrStdout(), app.format, positions)
			},
		},
		&cobra.Command{
			Use:         "allocation",
			Short:       "Show portfolio allocation",
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				summary, err := app.client.GetAccountSummary(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAllocation(cmd.OutOrStdout(), app.format, summary.Markets)
			},
		},
		newDividendsCmd(opts),
	)

	return cmd
}

func newDividendsCmd(opts *rootOptions) *cobra.Command {
	var (
		year          int
		byPaymentDate bool
	)
	cmd := &cobra.Command{
		Use:   "dividends",
		Short: "Annual dividend report (total, region, monthly)",
		Long: "Annual dividend report (total, region, monthly).\n\n" +
			"Note: uses a WTS internal endpoint; not available via the official Open API and may change without notice.",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			d, err := app.client.GetDividends(cmd.Context(), year, byPaymentDate)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteDividends(cmd.OutOrStdout(), app.format, d)
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "query year (default: this year)")
	cmd.Flags().BoolVar(&byPaymentDate, "by-payment-date", false, "by payment date (includes tax and fees)")
	return cmd
}
