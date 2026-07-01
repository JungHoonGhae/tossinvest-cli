package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCommunityCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "community",
		Short: "Toss community leaderboards and social data",
	}

	var rankType string
	rankingsCmd := &cobra.Command{
		Use:   "rankings",
		Short: "Community leaderboards (influencers, returns, follower surges)",
		Long: "Community leaderboards (influencers, returns, follower surges).\n\n" +
			"Note: uses a WTS internal endpoint; not available via the official Open API and may change without notice.",
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

	cmd.AddCommand(rankingsCmd)
	return cmd
}
