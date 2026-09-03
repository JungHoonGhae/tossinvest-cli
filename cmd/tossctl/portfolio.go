package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newPortfolioCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: i18n.T("portfolio.short"),
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:         "positions",
			Short:       i18n.T("portfolio.positions.short"),
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
			Short:       i18n.T("portfolio.allocation.short"),
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
		newPortfolioPerformanceCmd(opts),
		newPortfolioSnapshotsCmd(opts),
		newPortfolioSnapshotCmd(opts),
		newHiddenHoldingsCmd(opts),
	)

	return cmd
}

func newDividendsCmd(opts *rootOptions) *cobra.Command {
	var (
		year          int
		byPaymentDate bool
	)
	cmd := &cobra.Command{
		Use:         "dividends",
		Short:       i18n.T("portfolio.dividends.short"),
		Long:        i18n.T("portfolio.dividends.long"),
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
