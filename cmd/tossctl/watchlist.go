package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
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
		&cobra.Command{
			Use:         "create <name>",
			Short:       i18n.T("watchlist.group.create.short"),
			Args:        cobra.MinimumNArgs(1),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, args []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				g, err := app.client.CreateWatchlistGroup(cmd.Context(), strings.Join(args, " "))
				if err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "folder created: %s (id=%d)\n", g.Name, g.ID)
				return nil
			},
		},
		&cobra.Command{
			Use:         "rename [<id>] <new-name>",
			Short:       i18n.T("watchlist.group.rename.short"),
			Args:        cobra.RangeArgs(1, 2),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, args []string) error {
				var name string
				var rawID string
				if len(args) == 2 {
					rawID = args[0]
					name = strings.Join(args[1:], " ")
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
				if err := app.client.RenameWatchlistGroup(cmd.Context(), id, name); err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "folder renamed: id=%d -> %s\n", id, name)
				return nil
			},
		},
		&cobra.Command{
			Use:         "delete [<id>]",
			Short:       i18n.T("watchlist.group.delete.short"),
			Args:        cobra.MaximumNArgs(1),
			Annotations: map[string]string{"source": "wts"},
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
				if err := app.client.DeleteWatchlistGroup(cmd.Context(), id); err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "folder deleted: id=%d\n", id)
				return nil
			},
		},
	)
	return cmd
}

func newWatchlistAddRemoveCmd(opts *rootOptions, verb, short string) *cobra.Command {
	var groupID int64
	c := &cobra.Command{
		Use:         verb + " <symbol or name>",
		Short:       short,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if groupID == 0 {
				return fmt.Errorf("--group <folder-id> is required (see `watchlist groups`)")
			}
			symbol := strings.Join(args, " ")
			if verb == "add" {
				err = app.client.AddWatchlistItem(cmd.Context(), groupID, symbol)
			} else {
				err = app.client.RemoveWatchlistItem(cmd.Context(), groupID, symbol)
			}
			if err != nil {
				return userFacingCommandError(err)
			}
			action := "added"
			if verb == "remove" {
				action = "removed"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watchlist %s: %s (folder id=%d)\n", action, symbol, groupID)
			return nil
		},
	}
	c.Flags().Int64Var(&groupID, "group", 0, "target folder id (see `watchlist groups`)")
	return c
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
