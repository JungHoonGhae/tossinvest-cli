package main

import (
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newOrdersCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: i18n.T("orders.short"),
	}

	var completedMarket string
	var completedAllDates bool
	var completedSize, completedPage int
	cmd.AddCommand(
		&cobra.Command{
			Use:         "list",
			Short:       i18n.T("orders.list.short"),
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
		Short:       i18n.T("orders.completed.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			now := time.Now()
			from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			var orders []domain.Order
			if completedAllDates {
				orders, err = app.client.ListCompletedOrdersAllDates(cmd.Context(), completedMarket, completedSize, completedPage)
			} else {
				if completedSize < 1 || completedPage < 1 {
					return fmt.Errorf("size and page must be positive")
				}
				orders, err = app.client.ListCompletedOrdersRange(cmd.Context(), completedMarket, from, now, completedSize, completedPage)
			}
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteCompletedOrders(cmd.OutOrStdout(), app.format, orders)
		},
	}
	completedCmd.Flags().StringVar(&completedMarket, "market", "all", "Completed-history market filter: all, us, kr")
	completedCmd.Flags().BoolVar(&completedAllDates, "all-dates", false, "Read history across all dates, including canceled orders (default: current month)")
	completedCmd.Flags().IntVar(&completedSize, "size", 50, "Orders per page (all-dates: 1–100)")
	completedCmd.Flags().IntVar(&completedPage, "page", 1, "Page number, 1-based (all-dates: 1–100; reads preceding pages for cursors)")
	cmd.AddCommand(completedCmd)

	return cmd
}
