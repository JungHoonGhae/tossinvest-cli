package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
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
	selected, err := tui.PickFromList("폴더 선택", groupItems(groups))
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(selected, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("내부 오류: 폴더 id 파싱 실패: %s", selected)
	}
	return id, nil
}

func newWatchlistCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Read and manage watchlist (관심종목 조회·관리)",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List watchlist entries",
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				items, err := app.client.ListWatchlist(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return output.WriteWatchlist(cmd.OutOrStdout(), app.format, items)
			},
		},
		&cobra.Command{
			Use:   "groups",
			Short: "List watchlist folders (관심종목 폴더)",
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
		newWatchlistAddRemoveCmd(opts, "add", "관심종목에 종목 추가"),
		newWatchlistAddRemoveCmd(opts, "remove", "관심종목에서 종목 제거"),
	)

	return cmd
}

func newWatchlistGroupCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage watchlist folders (폴더 생성·이름변경·삭제)",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "create <name>",
			Short: "Create a watchlist folder",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				g, err := app.client.CreateWatchlistGroup(cmd.Context(), strings.Join(args, " "))
				if err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "폴더 생성: %s (id=%d)\n", g.Name, g.ID)
				return nil
			},
		},
		&cobra.Command{
			Use:   "rename [<id>] <new-name>",
			Short: "Rename a watchlist folder",
			Args:  cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				var id int64
				var name string

				if len(args) == 2 {
					// Fully-specified: rename <id> <new-name> — existing path, unchanged.
					var parseErr error
					id, parseErr = strconv.ParseInt(args[0], 10, 64)
					if parseErr != nil {
						return fmt.Errorf("폴더 id 는 숫자여야 합니다: %s", args[0])
					}
					name = strings.Join(args[1:], " ")
				} else {
					// 1-arg: treat args[0] as new-name, pick folder interactively.
					name = args[0]
					if !tui.IsInteractive(os.Stdin, os.Stdout) {
						return fmt.Errorf("폴더 id 와 새 이름을 지정하거나 터미널에서 실행하세요")
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
					fmt.Fprintf(cmd.OutOrStdout(), "폴더 이름 변경: id=%d → %s\n", id, name)
					return nil
				}

				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				if err := app.client.RenameWatchlistGroup(cmd.Context(), id, name); err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "폴더 이름 변경: id=%d → %s\n", id, name)
				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [<id>]",
			Short: "Delete a watchlist folder",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var id int64

				if len(args) == 1 {
					// Fully-specified: delete <id> — existing path, unchanged.
					var parseErr error
					id, parseErr = strconv.ParseInt(args[0], 10, 64)
					if parseErr != nil {
						return fmt.Errorf("폴더 id 는 숫자여야 합니다: %s", args[0])
					}
				} else {
					// No id: pick folder interactively (TTY only).
					if !tui.IsInteractive(os.Stdin, os.Stdout) {
						return fmt.Errorf("폴더 id 를 지정하거나 터미널에서 실행하세요")
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
					fmt.Fprintf(cmd.OutOrStdout(), "폴더 삭제: id=%d\n", id)
					return nil
				}

				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				if err := app.client.DeleteWatchlistGroup(cmd.Context(), id); err != nil {
					return userFacingCommandError(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "폴더 삭제: id=%d\n", id)
				return nil
			},
		},
	)
	return cmd
}

func newWatchlistAddRemoveCmd(opts *rootOptions, verb, short string) *cobra.Command {
	var groupID int64
	c := &cobra.Command{
		Use:   verb + " <symbol or name>",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if groupID == 0 {
				return fmt.Errorf("--group <폴더id> 필요 (`watchlist groups` 로 확인)")
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
			action := "추가"
			if verb == "remove" {
				action = "제거"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "관심종목 %s: %s (폴더 id=%d)\n", action, symbol, groupID)
			return nil
		},
	}
	c.Flags().Int64Var(&groupID, "group", 0, "대상 폴더 id (watchlist groups 로 확인)")
	return c
}
