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

// pickFolderID fetches watchlist folders and presents an interactive picker,
// returning the selected folder's int64 ID.
func pickFolderID(ctx context.Context, app *appContext) (int64, error) {
	groups, err := app.client.ListWatchlistGroups(ctx)
	if err != nil {
		return 0, userFacingCommandError(err)
	}
	selected, err := tui.PickFromList("Select a folder", groupItems(groups))
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
				var id int64
				var name string

				if len(args) == 2 {
					// Fully-specified: rename <id> <new-name> — existing path, unchanged.
					var parseErr error
					id, parseErr = strconv.ParseInt(args[0], 10, 64)
					if parseErr != nil {
						return fmt.Errorf("folder id must be a number: %s", args[0])
					}
					name = strings.Join(args[1:], " ")
				} else {
					// 1-arg: treat args[0] as new-name, pick folder interactively.
					name = args[0]
					if !tui.IsInteractive(os.Stdin, os.Stdout) {
						return fmt.Errorf("specify a folder id and new name, or run in an interactive terminal")
					}
					app, err := newAppContext(opts)
					if err != nil {
						return err
					}
					id, err = pickFolderID(cmd.Context(), app)
					if err != nil {
						return err
					}
					if err := app.client.RenameWatchlistGroup(cmd.Context(), id, name); err != nil {
						return userFacingCommandError(err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "folder renamed: id=%d -> %s\n", id, name)
					return nil
				}

				app, err := newAppContext(opts)
				if err != nil {
					return err
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
				var id int64

				if len(args) == 1 {
					// Fully-specified: delete <id> — existing path, unchanged.
					var parseErr error
					id, parseErr = strconv.ParseInt(args[0], 10, 64)
					if parseErr != nil {
						return fmt.Errorf("folder id must be a number: %s", args[0])
					}
				} else {
					// No id: pick folder interactively (TTY only).
					if !tui.IsInteractive(os.Stdin, os.Stdout) {
						return fmt.Errorf("specify a folder id, or run in an interactive terminal")
					}
					app, err := newAppContext(opts)
					if err != nil {
						return err
					}
					id, err = pickFolderID(cmd.Context(), app)
					if err != nil {
						return err
					}
					if err := app.client.DeleteWatchlistGroup(cmd.Context(), id); err != nil {
						return userFacingCommandError(err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "folder deleted: id=%d\n", id)
					return nil
				}

				app, err := newAppContext(opts)
				if err != nil {
					return err
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
			if all && len(args) > 0 {
				return fmt.Errorf("cannot specify both a folder id and --all")
			}
			var groupID int64
			if len(args) == 1 {
				var parseErr error
				groupID, parseErr = strconv.ParseInt(args[0], 10, 64)
				if parseErr != nil {
					return fmt.Errorf("folder id must be a number: %s", args[0])
				}
			} else if !all && !tui.IsInteractive(os.Stdin, os.Stdout) {
				// Early validation: non-TTY without group-id or --all should fail
				// before attempting to create an app context (which requires config).
				return fmt.Errorf("specify a folder id, or pass --all to list all folders (see `watchlist groups`)")
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			if all {
				items, err := app.client.ListAllWatchlistItems(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return output.WriteWatchlist(cmd.OutOrStdout(), app.format, items)
			}

			if len(args) == 0 {
				// TTY mode: interactive picker (non-TTY case already handled above).
				groupID, err = pickFolderID(cmd.Context(), app)
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
