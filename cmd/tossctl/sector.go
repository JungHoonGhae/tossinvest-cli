package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

// newSectorCmd exposes the official sector taxonomy (Open API 1.2.24). It is a
// separate group from `market sectors`/`market sector`, which read the WTS
// TICS tree: the two use different ids and the official one is token-only.
func newSectorCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sector",
		Short:       i18n.T("sector.short"),
		Long:        i18n.T("sector.long"),
		Annotations: map[string]string{"source": "official"},
	}
	src := map[string]string{"source": "official"}
	marketHelp := strings.Join(official.SectorMarkets(), " | ") + " (default KR)"

	listCmd := &cobra.Command{
		Use: "list", Short: i18n.T("sector.list.short"), Args: cobra.NoArgs, Annotations: src,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			c, err := app.client.Sectors(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteSectorCatalog(cmd.OutOrStdout(), app.format, c)
		},
	}

	var rankMarket, rankDuration string
	rankingsCmd := &cobra.Command{
		Use: "rankings <" + strings.Join(official.SectorRankingTypes(), "|") + ">", Short: i18n.T("sector.rankings.short"), Args: cobra.ExactArgs(1), Annotations: src,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			r, err := app.client.SectorRankings(cmd.Context(), args[0], rankMarket, rankDuration)
			if err != nil {
				return err
			}
			return output.WriteSectorRankings(cmd.OutOrStdout(), app.format, r)
		},
	}
	rankingsCmd.Flags().StringVar(&rankMarket, "market", "KR", marketHelp)
	rankingsCmd.Flags().StringVar(&rankDuration, "duration", "1d", strings.Join(official.SectorDurations(), " | "))

	var showMarket string
	showCmd := &cobra.Command{
		Use: "show <sector-id>", Short: i18n.T("sector.show.short"), Args: cobra.ExactArgs(1), Annotations: src,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			p, err := app.client.Sector(cmd.Context(), args[0], showMarket)
			if err != nil {
				return err
			}
			return output.WriteSectorProfile(cmd.OutOrStdout(), app.format, p)
		},
	}
	showCmd.Flags().StringVar(&showMarket, "market", "KR", marketHelp)

	var stocksMarket string
	stocksCmd := &cobra.Command{
		Use: "stocks <sector-id>", Short: i18n.T("sector.stocks.short"), Args: cobra.ExactArgs(1), Annotations: src,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			s, err := app.client.SectorStocks(cmd.Context(), args[0], stocksMarket)
			if err != nil {
				return err
			}
			return output.WriteSectorConstituents(cmd.OutOrStdout(), app.format, s)
		},
	}
	stocksCmd.Flags().StringVar(&stocksMarket, "market", "KR", marketHelp)

	var etfsMarket string
	etfsCmd := &cobra.Command{
		Use: "etfs <sector-id>", Short: i18n.T("sector.etfs.short"), Args: cobra.ExactArgs(1), Annotations: src,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			e, err := app.client.SectorETFs(cmd.Context(), args[0], etfsMarket)
			if err != nil {
				return err
			}
			return output.WriteSectorETFs(cmd.OutOrStdout(), app.format, e)
		},
	}
	etfsCmd.Flags().StringVar(&etfsMarket, "market", "KR", marketHelp)

	cmd.AddCommand(listCmd, rankingsCmd, showCmd, stocksCmd, etfsCmd)
	return cmd
}
