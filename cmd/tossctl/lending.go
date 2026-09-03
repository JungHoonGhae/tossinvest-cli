package main

import (
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLendingCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lending",
		Short: i18n.T("lending.short"),
	}

	expectedCmd := &cobra.Command{
		Use:         "expected",
		Short:       i18n.T("lending.expected.short"),
		Long:        i18n.T("lending.expected.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			data, err := app.client.GetLendingExpected(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteLendingExpected(cmd.OutOrStdout(), app.format, data)
		},
	}

	var topSize int
	topCmd := &cobra.Command{
		Use:         "top",
		Short:       i18n.T("lending.top.short"),
		Long:        i18n.T("lending.top.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if topSize < 0 {
				return fmt.Errorf("--size must be zero or greater")
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			ranking, err := app.client.GetTopLendingRevenue(cmd.Context(), topSize)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteLendingRevenueRanking(cmd.OutOrStdout(), app.format, ranking)
		},
	}
	topCmd.Flags().IntVar(&topSize, "size", 10, "number of ranked accounts (0 = all returned by server)")

	cmd.AddCommand(expectedCmd, topCmd)

	return cmd
}
