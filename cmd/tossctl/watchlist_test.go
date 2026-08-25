package main

import (
	"os"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/spf13/cobra"
)

// TestGroupItems verifies the pure groupItems mapping:
// Item.ID = string representation of group ID, Item.Label contains group Name.
func TestGroupItems(t *testing.T) {
	t.Parallel()

	groups := []domain.WatchlistGroup{
		{ID: 42, Name: "기술주", ItemCount: 5},
		{ID: 100, Name: "배당주", ItemCount: 0},
	}
	items := groupItems(groups)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "42" {
		t.Errorf("items[0].ID: want %q, got %q", "42", items[0].ID)
	}
	if !strings.Contains(items[0].Label, "기술주") {
		t.Errorf("items[0].Label should contain group name 기술주, got %q", items[0].Label)
	}

	if items[1].ID != "100" {
		t.Errorf("items[1].ID: want %q, got %q", "100", items[1].ID)
	}
	if !strings.Contains(items[1].Label, "배당주") {
		t.Errorf("items[1].Label should contain group name 배당주, got %q", items[1].Label)
	}
}

func TestGroupItemsEmpty(t *testing.T) {
	t.Parallel()

	items := groupItems(nil)
	if len(items) != 0 {
		t.Fatalf("expected empty slice, got %+v", items)
	}
}

// TestGroupDeleteNonTTYNoArgsError checks that `watchlist group delete` with
// no arguments and a non-TTY stdin returns a clean error without blocking.
func TestGroupDeleteNonTTYNoArgsError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	groupCmd := newWatchlistGroupCmd(opts)
	deleteCmd := findSubCmd(groupCmd, "delete")
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}

	gotErr := deleteCmd.RunE(deleteCmd, []string{})
	if gotErr == nil {
		t.Fatal("expected error for no args in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "id") && !strings.Contains(gotErr.Error(), "터미널") {
		t.Fatalf("error should mention id or 터미널, got: %v", gotErr)
	}
}

// TestGroupRenameNonTTYOneArgError checks that `watchlist group rename <name>`
// with exactly one argument and a non-TTY stdin returns a clean error.
func TestGroupRenameNonTTYOneArgError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	groupCmd := newWatchlistGroupCmd(opts)
	renameCmd := findSubCmd(groupCmd, "rename")
	if renameCmd == nil {
		t.Fatal("rename subcommand not found")
	}

	gotErr := renameCmd.RunE(renameCmd, []string{"새이름"})
	if gotErr == nil {
		t.Fatal("expected error for 1-arg in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "id") && !strings.Contains(gotErr.Error(), "터미널") {
		t.Fatalf("error should mention id or 터미널, got: %v", gotErr)
	}
}

// TestWatchlistListNonTTYNoArgsError checks that `watchlist list` with no args
// and a non-TTY stdin returns a clean error without blocking.
func TestWatchlistListNonTTYNoArgsError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	listCmd := newWatchlistListCmd(opts)

	gotErr := listCmd.RunE(listCmd, []string{})
	if gotErr == nil {
		t.Fatal("expected error for no args in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "folder id") && !strings.Contains(gotErr.Error(), "--all") {
		t.Fatalf("error should mention 'folder id' or '--all', got: %v", gotErr)
	}
}

// TestWatchlistListArgAndAllExclusiveError checks that passing both a folder ID
// argument and the --all flag returns an error.
func TestWatchlistListArgAndAllExclusiveError(t *testing.T) {
	opts := &rootOptions{}
	listCmd := newWatchlistListCmd(opts)
	_ = listCmd.Flags().Set("all", "true")

	gotErr := listCmd.RunE(listCmd, []string{"123"})
	if gotErr == nil {
		t.Fatal("expected error when both folder id and --all are specified, got nil")
	}
	if !strings.Contains(gotErr.Error(), "both a folder id and --all") {
		t.Fatalf("unexpected error message: %v", gotErr)
	}
}

// TestWatchlistListInvalidIDError checks that passing a non-numeric folder ID
// returns an early error before creating an app context.
func TestWatchlistListInvalidIDError(t *testing.T) {
	opts := &rootOptions{}
	listCmd := newWatchlistListCmd(opts)

	gotErr := listCmd.RunE(listCmd, []string{"not-a-number"})
	if gotErr == nil {
		t.Fatal("expected error when folder id is non-numeric, got nil")
	}
	if !strings.Contains(gotErr.Error(), "folder id must be a number") {
		t.Fatalf("unexpected error message: %v", gotErr)
	}
}

// findSubCmd returns the named subcommand of parent, or nil.
func findSubCmd(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
