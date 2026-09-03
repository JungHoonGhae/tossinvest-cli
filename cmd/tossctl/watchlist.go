package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
	"github.com/spf13/cobra"
)

// groupItems converts a slice of domain.WatchlistGroup into tui.Item entries
// for interactive folder pickers. It is a pure function with no side effects.
func groupItems(groups []domain.WatchlistGroup) []tui.Item {
	items := make([]tui.Item, len(groups))
	for i, g := range groups {
		items[i] = tui.Item{
			ID:    strconv.FormatInt(g.ID, 10),
			Label: fmt.Sprintf("%s (%d)", g.Name, g.ItemCount),
		}
	}
	return items
}

type folderIntentMode uint8

const (
	folderIntentAll folderIntentMode = iota
	folderIntentSpecific
	folderIntentInteractive
)

type folderIntent struct {
	mode folderIntentMode
	id   int64
}

type folderIntentResolver struct {
	interactive bool
	in          *os.File
	out         *os.File
}

func folderResolverFor(cmd *cobra.Command) folderIntentResolver {
	in, inOK := cmd.InOrStdin().(*os.File)
	out, outOK := cmd.OutOrStdout().(*os.File)
	return folderIntentResolver{
		interactive: inOK && outOK && tui.IsInteractive(in, out),
		in:          in,
		out:         out,
	}
}

func specificFolderIntent(rawID string) (folderIntent, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return folderIntent{}, fmt.Errorf("folder id must be a number: %s", rawID)
	}
	return folderIntent{mode: folderIntentSpecific, id: id}, nil
}

func (r folderIntentResolver) list(rawID string, all bool) (folderIntent, error) {
	if all && rawID != "" {
		return folderIntent{}, fmt.Errorf("cannot specify both a folder id and --all")
	}
	if rawID != "" {
		return specificFolderIntent(rawID)
	}
	if all || !r.interactive {
		return folderIntent{mode: folderIntentAll}, nil
	}
	return folderIntent{mode: folderIntentInteractive}, nil
}

func (r folderIntentResolver) required(rawID string) (folderIntent, error) {
	if rawID != "" {
		return specificFolderIntent(rawID)
	}
	if !r.interactive {
		return folderIntent{}, fmt.Errorf("specify a folder id, or run in an interactive terminal")
	}
	return folderIntent{mode: folderIntentInteractive}, nil
}

// pickFolderID fetches watchlist folders and presents an interactive picker,
// returning the selected folder's int64 ID.
func pickFolderID(ctx context.Context, app *appContext, in, out *os.File) (int64, error) {
	groups, err := app.client.ListWatchlistGroups(ctx)
	if err != nil {
		return 0, userFacingCommandError(err)
	}
	selected, err := tui.PickFromListWith(in, out, "Select a folder", groupItems(groups))
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(selected, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("internal error: failed to parse folder id: %s", selected)
	}
	return id, nil
}

func newWatchlistCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: i18n.T("watchlist.short"),
	}

	cmd.AddCommand(
		newWatchlistListCmd(opts),
		&cobra.Command{
			Use:         "groups",
			Short:       i18n.T("watchlist.groups.short"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				groups, err := app.client.ListWatchlistGroups(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return output.WriteWatchlistGroups(cmd.OutOrStdout(), app.format, groups)
			},
		},
		newWatchlistGroupCmd(opts),
		newWatchlistAddRemoveCmd(opts, "add", i18n.T("watchlist.add.short")),
		newWatchlistAddRemoveCmd(opts, "remove", i18n.T("watchlist.remove.short")),
	)

	return cmd
}

func newWatchlistGroupCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: i18n.T("watchlist.group.short"),
	}

	cmd.AddCommand(
		newWatchlistGroupCreateCmd(opts),
		newWatchlistGroupRenameCmd(opts),
		newWatchlistGroupDeleteCmd(opts),
	)
	return cmd
}

func newWatchlistGroupCreateCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	var confirm string
	cmd := &cobra.Command{
		Use: "create <name>", Short: i18n.T("watchlist.group.create.short"), Args: cobra.MinimumNArgs(1),
		Annotations: watchlistMutationAnnotations(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := watchlistservice.NewService(app.client).ChangeGroup(
				cmd.Context(), watchlistservice.GroupCreate, 0, strings.Join(args, " "),
				watchlistservice.ExecuteOptions{Execute: execute, Confirm: confirm},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderWatchlistPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	bindWatchlistExecutionFlags(cmd, &execute, &confirm, nil)
	return cmd
}

func newWatchlistGroupRenameCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	var confirm string
	cmd := &cobra.Command{
		Use: "rename [<id>] <new-name>", Short: i18n.T("watchlist.group.rename.short"), Args: cobra.RangeArgs(1, 2),
		Annotations: watchlistMutationAnnotations(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			var rawID, name string
			if len(args) == 2 {
				rawID, name = args[0], args[1]
			} else {
				name = args[0]
			}
			resolver := folderResolverFor(cmd)
			intent, err := resolver.required(rawID)
			if err != nil {
				return err
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			id := intent.id
			if intent.mode == folderIntentInteractive {
				id, err = pickFolderID(cmd.Context(), app, resolver.in, resolver.out)
				if err != nil {
					return err
				}
			}
			plan, err := watchlistservice.NewService(app.client).ChangeGroup(
				cmd.Context(), watchlistservice.GroupRename, id, name,
				watchlistservice.ExecuteOptions{Execute: execute, Confirm: confirm},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderWatchlistPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	bindWatchlistExecutionFlags(cmd, &execute, &confirm, nil)
	return cmd
}

func newWatchlistGroupDeleteCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	var confirm string
	var acknowledge bool
	cmd := &cobra.Command{
		Use: "delete [<id>]", Short: i18n.T("watchlist.group.delete.short"), Args: cobra.MaximumNArgs(1),
		Annotations: watchlistMutationAnnotations(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawID := ""
			if len(args) == 1 {
				rawID = args[0]
			}
			resolver := folderResolverFor(cmd)
			intent, err := resolver.required(rawID)
			if err != nil {
				return err
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			id := intent.id
			if intent.mode == folderIntentInteractive {
				id, err = pickFolderID(cmd.Context(), app, resolver.in, resolver.out)
				if err != nil {
					return err
				}
			}
			plan, err := watchlistservice.NewService(app.client).ChangeGroup(
				cmd.Context(), watchlistservice.GroupDelete, id, "",
				watchlistservice.ExecuteOptions{Execute: execute, Confirm: confirm, AcknowledgeIrreversible: acknowledge},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderWatchlistPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	bindWatchlistExecutionFlags(cmd, &execute, &confirm, &acknowledge)
	return cmd
}

func newWatchlistAddRemoveCmd(opts *rootOptions, verb, short string) *cobra.Command {
	var groupID int64
	var execute bool
	var confirm string
	action := watchlistservice.ItemAdd
	if verb == "remove" {
		action = watchlistservice.ItemRemove
	}
	c := &cobra.Command{
		Use:         verb + " <symbol or name>",
		Short:       short,
		Args:        cobra.MinimumNArgs(1),
		Annotations: watchlistMutationAnnotations(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if groupID == 0 {
				return fmt.Errorf("--group <folder-id> is required (see `watchlist groups`)")
			}
			plan, err := watchlistservice.NewService(app.client).ChangeItem(
				cmd.Context(), action, groupID, strings.Join(args, " "),
				watchlistservice.ExecuteOptions{Execute: execute, Confirm: confirm},
			)
			if err != nil {
				return userFacingCommandError(err)
			}
			return renderWatchlistPlan(cmd.OutOrStdout(), app.format, plan)
		},
	}
	c.Flags().Int64Var(&groupID, "group", 0, "target folder id (see `watchlist groups`)")
	bindWatchlistExecutionFlags(c, &execute, &confirm, nil)
	return c
}

func watchlistMutationAnnotations(irreversible bool) map[string]string {
	if irreversible {
		return mutationAnnotations("wts", "securities", "destructive", "irreversible")
	}
	return mutationAnnotations("wts", "securities", "preference", "reversible")
}

func bindWatchlistExecutionFlags(cmd *cobra.Command, execute *bool, confirm *string, acknowledge *bool) {
	cmd.Flags().BoolVar(execute, "execute", false, "apply the previewed watchlist change")
	cmd.Flags().StringVar(confirm, "confirm", "", "confirm token from a fresh preview")
	if acknowledge != nil {
		cmd.Flags().BoolVar(acknowledge, "acknowledge-irreversible", false, "acknowledge that deleting this folder cannot be undone")
	}
}

func renderWatchlistPlan(w io.Writer, format output.Format, plan watchlistservice.Plan) error {
	if format == output.FormatJSON {
		return output.WriteJSON(w, plan)
	}
	if format == output.FormatCSV {
		return writeCommandCSV(w, [][]string{
			{"kind", "action", "group_id", "group_name", "new_name", "product_code", "current_item_count", "irreversible", "requires_irreversible_acknowledgement", "noop", "applied", "reconciled", "confirm_token", "affected_items"},
			{plan.Kind, string(plan.Action), strconv.FormatInt(plan.GroupID, 10), plan.GroupName, plan.NewName, plan.ProductCode, strconv.Itoa(plan.CurrentItemCount), strconv.FormatBool(plan.Irreversible), strconv.FormatBool(plan.RequiresIrreversibleAcknowledgement), strconv.FormatBool(plan.Noop), strconv.FormatBool(plan.Applied), strconv.FormatBool(plan.Reconciled), plan.ConfirmToken, renderWatchlistPreviewItems(plan.AffectedItems)},
		})
	}
	if format != output.FormatTable {
		return fmt.Errorf("unsupported output format: %s", format)
	}
	var rendered strings.Builder
	status := "preview"
	if plan.Applied {
		status = "applied"
	} else if plan.Noop {
		status = "up to date"
	}
	fmt.Fprintf(&rendered, "Status:     %s\nAction:     %s\nFolder:     %s (id=%d)\n", status, plan.Action, plan.GroupName, plan.GroupID)
	if plan.NewName != "" {
		fmt.Fprintf(&rendered, "New name:   %s\n", plan.NewName)
	}
	if plan.ProductCode != "" {
		fmt.Fprintf(&rendered, "Product:    %s\n", plan.ProductCode)
	}
	if plan.Irreversible {
		fmt.Fprintf(&rendered, "Risk:       irreversible deletion (%d current item(s))\n", plan.CurrentItemCount)
		if affected := renderWatchlistPreviewItems(plan.AffectedItems); affected != "" {
			fmt.Fprintf(&rendered, "Affected:   %s\n", affected)
		}
	}
	if plan.Reconciled {
		fmt.Fprintln(&rendered, "Verified:   server applied the request despite a transport error")
	}
	if !plan.Applied && !plan.Noop {
		ack := ""
		if plan.RequiresIrreversibleAcknowledgement {
			ack = " --acknowledge-irreversible"
		}
		fmt.Fprintf(&rendered, "Confirm:    %s\nNext:       repeat this command with --execute --confirm %s%s\n", plan.ConfirmToken, plan.ConfirmToken, ack)
	}
	_, err := io.WriteString(w, rendered.String())
	return err
}

func renderWatchlistPreviewItems(items []watchlistservice.PreviewItem) string {
	rendered := make([]string, 0, len(items))
	for _, item := range items {
		details := make([]string, 0, 2)
		if item.Symbol != "" && item.Symbol != item.ProductCode {
			details = append(details, item.Symbol)
		}
		if item.Name != "" {
			details = append(details, item.Name)
		}
		value := item.ProductCode
		if len(details) > 0 {
			value += " (" + strings.Join(details, ", ") + ")"
		}
		rendered = append(rendered, value)
	}
	return strings.Join(rendered, "; ")
}

func newWatchlistListCmd(opts *rootOptions) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:         "list [<group-id>]",
		Short:       i18n.T("watchlist.list.short"),
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rawID := ""
			if len(args) == 1 {
				rawID = args[0]
			}
			resolver := folderResolverFor(cmd)
			intent, err := resolver.list(rawID, all)
			if err != nil {
				return err
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			if intent.mode == folderIntentAll {
				items, err := app.client.ListAllWatchlistItems(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return output.WriteWatchlist(cmd.OutOrStdout(), app.format, items)
			}

			groupID := intent.id
			if intent.mode == folderIntentInteractive {
				groupID, err = pickFolderID(cmd.Context(), app, resolver.in, resolver.out)
				if err != nil {
					return err
				}
			}

			items, err := app.client.GetWatchlistGroupItems(cmd.Context(), groupID)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteWatchlist(cmd.OutOrStdout(), app.format, items)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "list items from all folders (flat)")
	return cmd
}
