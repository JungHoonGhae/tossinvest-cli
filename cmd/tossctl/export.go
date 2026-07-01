package main

import (
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newExportCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: i18n.T("export.short"),
	}

	var positionsMarket string
	positionsCmd := &cobra.Command{
		Use:         "positions",
		Short:       i18n.T("export.positions.short"),
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
			positions = filterPositionsByMarket(positions, positionsMarket)
			return output.WritePositions(cmd.OutOrStdout(), output.FormatCSV, positions)
		},
	}
	positionsCmd.Flags().StringVar(&positionsMarket, "market", "all", "Filter by market: all, us, kr")

	var ordersMarket string
	ordersCmd := &cobra.Command{
		Use:         "orders",
		Short:       i18n.T("export.orders.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			orders, err := app.client.ListCompletedOrders(cmd.Context(), ordersMarket)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteCompletedOrders(cmd.OutOrStdout(), output.FormatCSV, orders)
		},
	}
	ordersCmd.Flags().StringVar(&ordersMarket, "market", "all", "Filter by market: all, us, kr")

	cmd.AddCommand(positionsCmd, ordersCmd)

	return cmd
}

func filterPositionsByMarket(positions []domain.Position, market string) []domain.Position {
	market = strings.ToLower(strings.TrimSpace(market))
	if market == "" || market == "all" {
		return positions
	}
	var filtered []domain.Position
	for _, p := range positions {
		marketType := strings.ToLower(p.MarketType)
		if (market == "kr" && strings.Contains(marketType, "kr")) ||
			(market == "us" && strings.Contains(marketType, "us")) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
