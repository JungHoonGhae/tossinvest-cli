package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newOpenAPIIPCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "ip",
		Short:       i18n.T("openapi.ip.short"),
		Annotations: map[string]string{"source": "wts", "domain": "system"},
	}

	listCmd := &cobra.Command{
		Use:         "list",
		Short:       i18n.T("openapi.ip.list.short"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "system"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			manager := openapiip.NewService(app.client, openapiip.NewHTTPResolver(nil, ""))
			ips, err := manager.List(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderOpenAPIIPList(cmd.OutOrStdout(), app.format, ips)
		},
	}

	var execute bool
	var confirm string
	replaceCmd := &cobra.Command{
		Use:   "replace-current",
		Short: i18n.T("openapi.ip.replaceCurrent.short"),
		Long: "Resolve this machine's current public IP and replace every other allowed IP. " +
			"The default is a preview. Apply only with --execute and the preview's --confirm token. " +
			"Every mutation is verified; on failure tossctl re-reads server state and restores the previous allowlist.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "system"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return userFacingCommandError(err)
			}
			manager := openapiip.NewService(app.client, openapiip.NewHTTPResolver(nil, ""))
			plan, err := manager.ReplaceCurrent(cmd.Context(), openapiip.ExecuteOptions{
				Execute: execute,
				Confirm: confirm,
			})
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderOpenAPIIPPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	replaceCmd.Flags().BoolVar(&execute, "execute", false, "apply the previewed allowlist replacement")
	replaceCmd.Flags().StringVar(&confirm, "confirm", "", "confirm token from a fresh preview")

	cmd.AddCommand(listCmd, replaceCmd)
	return cmd
}

func renderOpenAPIIPList(w io.Writer, format output.Format, ips []string) error {
	switch format {
	case output.FormatJSON:
		return output.WriteJSON(w, map[string]any{"allowed_ips": ips})
	case output.FormatCSV:
		rows := make([][]string, 0, len(ips)+1)
		rows = append(rows, []string{"allowed_ip"})
		for _, ip := range ips {
			rows = append(rows, []string{ip})
		}
		return writeCommandCSV(w, rows)
	case output.FormatTable:
		if len(ips) == 0 {
			_, err := fmt.Fprintln(w, "Allowed IPs: (none)")
			return err
		}
		_, err := fmt.Fprintf(w, "Allowed IPs: %s\n", strings.Join(ips, ", "))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderOpenAPIIPPlan(w io.Writer, format output.Format, plan openapiip.Plan) error {
	switch format {
	case output.FormatJSON:
		return output.WriteJSON(w, plan)
	case output.FormatCSV:
		return writeCommandCSV(w, [][]string{
			{"kind", "current_ip", "existing_ips", "delete_ips", "add_ip", "noop", "applied", "canonical", "confirm_token"},
			{plan.Kind, plan.CurrentIP, strings.Join(plan.ExistingIPs, "|"), strings.Join(plan.DeleteIPs, "|"), plan.AddIP, strconv.FormatBool(plan.Noop), strconv.FormatBool(plan.Applied), plan.Canonical, plan.ConfirmToken},
		})
	case output.FormatTable:
		status := "preview"
		if plan.Applied {
			status = "applied"
		} else if plan.Noop {
			status = "up to date"
		}
		remove := "(none)"
		if len(plan.DeleteIPs) > 0 {
			remove = strings.Join(plan.DeleteIPs, ", ")
		}
		add := plan.AddIP
		if add == "" {
			add = "(none)"
		}
		var text strings.Builder
		fmt.Fprintf(&text, "Status:     %s\n", status)
		fmt.Fprintf(&text, "Current IP: %s\n", plan.CurrentIP)
		fmt.Fprintf(&text, "Remove:     %s\n", remove)
		fmt.Fprintf(&text, "Add:        %s\n", add)
		if !plan.Applied && !plan.Noop {
			fmt.Fprintf(&text, "Confirm:    %s\n", plan.ConfirmToken)
			fmt.Fprintf(&text, "Next:       tossctl openapi ip replace-current --execute --confirm %s\n", plan.ConfirmToken)
		}
		_, err := io.WriteString(w, text.String())
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeCommandCSV(w io.Writer, rows [][]string) error {
	cw := csv.NewWriter(w)
	cw.WriteAll(rows)
	return cw.Error()
}
