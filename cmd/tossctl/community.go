package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCommunityCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "community",
		Short: i18n.T("community.short"),
	}

	var rankType string
	rankingsCmd := &cobra.Command{
		Use:         "rankings",
		Short:       i18n.T("community.rankings.short"),
		Long:        i18n.T("community.rankings.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			r, err := app.client.GetCommunityRankings(cmd.Context(), rankType)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteCommunityRanking(cmd.OutOrStdout(), app.format, r)
		},
	}
	rankingsCmd.Flags().StringVar(&rankType, "type", "influencer", "ranking type: influencer | profit | followers")

	boardsCmd := &cobra.Command{
		Use:         "boards",
		Short:       i18n.T("community.boards.short"),
		Long:        i18n.T("community.boards.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			boards, err := app.client.GetPopularBoards(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteCommunityBoards(cmd.OutOrStdout(), app.format, boards)
		},
	}

	cmd.AddCommand(rankingsCmd, boardsCmd)
	return cmd
}
