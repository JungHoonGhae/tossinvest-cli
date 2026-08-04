package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

func newSearchCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "search <query>",
		Short:       i18n.T("search.short"),
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			// 인자를 공백으로 이어 붙인다 — 종목명에 공백이 흔해서
			// 따옴표를 요구하면 대부분의 검색이 실패한다.
			r, err := app.client.Search(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WriteSearchResults(cmd.OutOrStdout(), app.format, r)
		},
	}
}
