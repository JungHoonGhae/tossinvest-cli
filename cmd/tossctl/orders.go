package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newOrdersCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Read order history data",
	}

	var completedMarket string
	cmd.AddCommand(
		&cobra.Command{
			Use:         "list",
			Short:       "List read-only order history",
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				orders, err := app.client.ListPendingOrders(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteOrders(cmd.OutOrStdout(), app.format, orders)
			},
		},
	)

	completedCmd := &cobra.Command{
		Use:         "completed",
		Short:       "List completed-history orders for the current month",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			orders, err := app.client.ListCompletedOrders(cmd.Context(), completedMarket)
			if err != nil {
				return userFacingCommandError(err)
			}

			return output.WriteCompletedOrders(cmd.OutOrStdout(), app.format, orders)
		},
	}
	completedCmd.Flags().StringVar(&completedMarket, "market", "all", "Completed-history market filter: all, us, kr")
	cmd.AddCommand(completedCmd)

	return cmd
}
