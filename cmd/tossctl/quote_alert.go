package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	"github.com/spf13/cobra"
)

func newPriceAlertCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alert",
		Short: i18n.T("quote.alert.short"),
	}
	listCmd := &cobra.Command{
		Use:         "list <symbol or name>",
		Short:       i18n.T("quote.alert.list.short"),
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			alerts, err := pricealert.NewService(app.client).List(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WritePriceAlerts(cmd.OutOrStdout(), app.format, alerts)
		},
	}
	cmd.AddCommand(listCmd, newPriceAlertChangeCmd(opts, pricealert.ActionAdd), newPriceAlertChangeCmd(opts, pricealert.ActionRemove))
	return cmd
}

func newPriceAlertChangeCmd(opts *rootOptions, action pricealert.Action) *cobra.Command {
	var price float64
	var currency string
	var execute bool
	var confirm string
	cmd := &cobra.Command{
		Use:         string(action) + " <symbol or name>",
		Short:       i18n.T("quote.alert." + string(action) + ".short"),
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := pricealert.NewService(app.client).Change(
				cmd.Context(), action, strings.Join(args, " "), price, currency,
				pricealert.ExecuteOptions{Execute: execute, Confirm: confirm},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderPriceAlertPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	cmd.Flags().Float64Var(&price, "price", 0, "target price")
	cmd.Flags().StringVar(&currency, "currency", "", "price currency: KRW or USD")
	cmd.Flags().BoolVar(&execute, "execute", false, "apply the previewed price-alert change")
	cmd.Flags().StringVar(&confirm, "confirm", "", "confirm token from a fresh preview")
	_ = cmd.MarkFlagRequired("price")
	_ = cmd.MarkFlagRequired("currency")
	return cmd
}

func renderPriceAlertPlan(w io.Writer, format output.Format, plan pricealert.Plan) error {
	if format == output.FormatJSON {
		return output.WriteJSON(w, plan)
	}
	if format == output.FormatCSV {
		return writeCommandCSV(w, [][]string{
			{"kind", "action", "product_code", "target_price", "currency", "noop", "applied", "reconciled", "canonical", "confirm_token"},
			{plan.Kind, string(plan.Action), plan.ProductCode, strconv.FormatFloat(plan.TargetPrice, 'f', -1, 64), plan.Currency, strconv.FormatBool(plan.Noop), strconv.FormatBool(plan.Applied), strconv.FormatBool(plan.Reconciled), plan.Canonical, plan.ConfirmToken},
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
	fmt.Fprintf(w, "Status:   %s\nAction:   %s\nProduct:  %s\nTarget:   %s %s\n", status, plan.Action, plan.ProductCode, strconv.FormatFloat(plan.TargetPrice, 'f', -1, 64), plan.Currency)
	if plan.Reconciled {
		fmt.Fprintln(w, "Verified: server applied the request despite a transport error")
	}
	if !plan.Applied && !plan.Noop {
		fmt.Fprintf(w, "Confirm:  %s\nNext:     tossctl quote alert %s %s --price %s --currency %s --execute --confirm %s\n", plan.ConfirmToken, plan.Action, plan.ProductCode, strconv.FormatFloat(plan.TargetPrice, 'f', -1, 64), plan.Currency, plan.ConfirmToken)
	}
	return nil
}
