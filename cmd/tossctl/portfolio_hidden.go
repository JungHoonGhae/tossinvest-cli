package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newHiddenHoldingsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "hidden", Short: i18n.T("portfolio.hidden.short")}
	var listAccount string
	listCmd := &cobra.Command{
		Use:         "list",
		Short:       i18n.T("portfolio.hidden.list.short"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			holdings, err := hiddenholding.NewService(app.client).List(cmd.Context(), listAccount)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteHiddenHoldings(cmd.OutOrStdout(), app.format, holdings)
		},
	}
	listCmd.Flags().StringVar(&listAccount, "account", "", "Securities account key (default: primary account)")
	cmd.AddCommand(listCmd, newHiddenHoldingChangeCmd(opts, hiddenholding.ActionHide), newHiddenHoldingChangeCmd(opts, hiddenholding.ActionShow))
	return cmd
}

func newHiddenHoldingChangeCmd(opts *rootOptions, action hiddenholding.Action) *cobra.Command {
	var account string
	var execute bool
	var confirm string
	cmd := &cobra.Command{
		Use:         string(action) + " <symbol or name>",
		Short:       i18n.T("portfolio.hidden." + string(action) + ".short"),
		Args:        cobra.MinimumNArgs(1),
		Annotations: mutationAnnotations("wts", "securities", "preference", "reversible"),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := hiddenholding.NewService(app.client).Change(
				cmd.Context(), action, strings.Join(args, " "), account,
				hiddenholding.ExecuteOptions{Execute: execute, Confirm: confirm},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderHiddenHoldingPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Securities account key (default: primary account)")
	cmd.Flags().BoolVar(&execute, "execute", false, "apply the previewed hidden-holding change")
	cmd.Flags().StringVar(&confirm, "confirm", "", "confirm token from a fresh preview")
	return cmd
}

func renderHiddenHoldingPlan(w io.Writer, format output.Format, plan hiddenholding.Plan) error {
	if format == output.FormatJSON {
		return output.WriteJSON(w, plan)
	}
	if format == output.FormatCSV {
		return writeCommandCSV(w, [][]string{
			{"kind", "action", "product_code", "account_scope", "noop", "applied", "reconciled", "canonical", "confirm_token"},
			{plan.Kind, string(plan.Action), plan.ProductCode, plan.AccountScope, strconv.FormatBool(plan.Noop), strconv.FormatBool(plan.Applied), strconv.FormatBool(plan.Reconciled), plan.Canonical, plan.ConfirmToken},
		})
	}
	if format != output.FormatTable {
		return fmt.Errorf("unsupported output format: %s", format)
	}
	status := "preview"
	if plan.Applied {
		status = "applied"
	} else if plan.Noop {
		status = "up to date"
	}
	fmt.Fprintf(w, "Status:        %s\nAction:        %s\nProduct:       %s\nAccount scope: %s\n", status, plan.Action, plan.ProductCode, plan.AccountScope)
	if plan.Reconciled {
		fmt.Fprintln(w, "Verified:      server applied the request despite a transport error")
	}
	if !plan.Applied && !plan.Noop {
		fmt.Fprintf(w, "Confirm:       %s\nNext:          repeat this command with --execute --confirm %s (and the same --account, if used)\n", plan.ConfirmToken, plan.ConfirmToken)
	}
	return nil
}
