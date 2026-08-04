package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccountCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: i18n.T("account.short"),
	}

	cmd.AddCommand(
		newAccountDetailCmd(opts),
		newAccountInterestCmd(opts),
		&cobra.Command{
			Use:         "list",
			Short:       i18n.T("account.list.short"),
			Annotations: map[string]string{"source": "both"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				accounts, primaryKey, err := app.client.ListAccounts(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccounts(cmd.OutOrStdout(), app.format, accounts, primaryKey)
			},
		},
		&cobra.Command{
			Use:         "summary",
			Short:       i18n.T("account.summary.short"),
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

				return output.WriteAccountSummary(cmd.OutOrStdout(), app.format, summary)
			},
		},
		&cobra.Command{
			Use:         "commission",
			Short:       i18n.T("account.commission.short"),
			Long:        i18n.T("account.commission.long"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				schedule, err := app.client.GetCommissionSchedule(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccountCommission(cmd.OutOrStdout(), app.format, schedule)
			},
		},
		&cobra.Command{
			Use:         "prime",
			Short:       i18n.T("account.prime.short"),
			Long:        i18n.T("account.prime.long"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				status, err := app.client.GetPrimeStatus(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccountPrime(cmd.OutOrStdout(), app.format, status)
			},
		},
		newAccountReceivableCmd(opts),
	)

	return cmd
}

func newAccountReceivableCmd(opts *rootOptions) *cobra.Command {
	var currency string
	cmd := &cobra.Command{
		Use:         "receivable",
		Short:       i18n.T("account.receivable.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			n, err := app.client.GetMarginNotice(cmd.Context(), currency)
			if err != nil {
				return err
			}
			return output.WriteMarginNotice(cmd.OutOrStdout(), app.format, n)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "KRW", "KRW or USD")
	return cmd
}

func newAccountDetailCmd(opts *rootOptions) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:         "detail",
		Short:       i18n.T("account.detail.short"),
		Long:        i18n.T("account.detail.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			d, err := app.client.GetAccountDetail(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccountDetail(cmd.OutOrStdout(), app.format, d, full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "show the account number in full (default masks it)")
	return cmd
}

func newAccountInterestCmd(opts *rootOptions) *cobra.Command {
	var year int

	cmd := &cobra.Command{
		Use:         "interest",
		Short:       i18n.T("account.interest.short"),
		Long:        i18n.T("account.interest.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			interest, err := app.client.GetAccountInterest(cmd.Context(), year)
			if err != nil {
				return userFacingCommandError(err)
			}

			// 빈 해는 흔하다(계좌 개설 전, 이자 미발생). 그때만 연도 목록을
			// 덧붙인다 — 매번 부르면 요청이 두 배가 된다. 목록 조회가 실패해도
			// 본 결과는 그대로 낸다.
			if len(interest.Monthly) == 0 || !hasInterestPayment(interest) {
				if years, yErr := app.client.GetInterestYears(cmd.Context()); yErr == nil {
					interest.AvailableYears = years
				}
			}

			return output.WriteAccountInterest(cmd.OutOrStdout(), app.format, interest)
		},
	}

	cmd.Flags().IntVar(&year, "year", 0, i18n.T("account.interest.yearFlag"))
	return cmd
}

func hasInterestPayment(ai domain.AccountInterest) bool {
	for _, m := range ai.Monthly {
		if len(m.Payments) > 0 {
			return true
		}
	}
	return false
}
