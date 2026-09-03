package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
	"github.com/spf13/cobra"
)

type failingWatchlistWriter struct{}

func (failingWatchlistWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

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
	opts := &rootOptions{}
	groupCmd := newWatchlistGroupCmd(opts)
	deleteCmd := findSubCmd(groupCmd, "delete")
	if deleteCmd == nil {
		t.Fatal("delete subcommand not found")
	}
	deleteCmd.SetIn(strings.NewReader(""))

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
	opts := &rootOptions{}
	groupCmd := newWatchlistGroupCmd(opts)
	renameCmd := findSubCmd(groupCmd, "rename")
	if renameCmd == nil {
		t.Fatal("rename subcommand not found")
	}
	renameCmd.SetIn(strings.NewReader(""))

	gotErr := renameCmd.RunE(renameCmd, []string{"새이름"})
	if gotErr == nil {
		t.Fatal("expected error for 1-arg in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "id") && !strings.Contains(gotErr.Error(), "터미널") {
		t.Fatalf("error should mention id or 터미널, got: %v", gotErr)
	}
}

// TestWatchlistListNonTTYNoArgsListsAll checks that `watchlist list` with no
// args and a non-TTY stdin falls back to the flat all-folders list instead of
// erroring or blocking on the interactive picker.
//
// 이 커맨드의 원래 동작이고 스크립트·MCP 가 그대로 쓴다. 폴더를 요구하면 피커가
// 행(hang)되는 것은 막지만 자동화가 깨진다.
func TestWatchlistListNonTTYNoArgsListsAll(t *testing.T) {
	opts := &rootOptions{}
	listCmd := newWatchlistListCmd(opts)
	listCmd.SetIn(strings.NewReader(""))

	// 세션이 없으므로 config/네트워크 단계에서 실패한다. 확인하려는 것은 "폴더를
	// 고르라는 요구로 거절당하지 않는다" 는 것뿐이다.
	gotErr := listCmd.RunE(listCmd, []string{})
	if gotErr != nil {
		for _, refusal := range []string{"folder id", "--all", "interactive terminal"} {
			if strings.Contains(gotErr.Error(), refusal) {
				t.Fatalf("non-TTY with no args must fall back to all folders, got refusal: %v", gotErr)
			}
		}
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

func TestFolderIntentResolverAcceptsExplicitID(t *testing.T) {
	r := folderIntentResolver{}
	for _, resolve := range []struct {
		name string
		call func() (folderIntent, error)
	}{
		{name: "list", call: func() (folderIntent, error) { return r.list("42", false) }},
		{name: "required", call: func() (folderIntent, error) { return r.required("42") }},
	} {
		t.Run(resolve.name, func(t *testing.T) {
			got, err := resolve.call()
			if err != nil {
				t.Fatal(err)
			}
			if got.mode != folderIntentSpecific || got.id != 42 {
				t.Fatalf("intent = %+v, want specific folder 42", got)
			}
		})
	}
}

func TestFolderIntentResolverKeepsInteractiveIntent(t *testing.T) {
	r := folderIntentResolver{interactive: true}
	list, err := r.list("", false)
	if err != nil || list.mode != folderIntentInteractive {
		t.Fatalf("interactive list intent = %+v, %v", list, err)
	}
	required, err := r.required("")
	if err != nil || required.mode != folderIntentInteractive {
		t.Fatalf("interactive required intent = %+v, %v", required, err)
	}
}

func TestWatchlistMutationsDeclareGuardrails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path         []string
		irreversible bool
	}{
		{path: []string{"watchlist", "group", "create"}},
		{path: []string{"watchlist", "group", "rename"}},
		{path: []string{"watchlist", "group", "delete"}, irreversible: true},
		{path: []string{"watchlist", "add"}},
		{path: []string{"watchlist", "remove"}},
	}
	for _, tc := range tests {
		cmd, _, err := newRootCmd().Find(tc.path)
		if err != nil || cmd.Name() != tc.path[len(tc.path)-1] {
			t.Fatalf("%v: command=%v err=%v", tc.path, cmd, err)
		}
		if cmd.Annotations["writes_state"] != "true" || cmd.Annotations["mutating"] != "" {
			t.Errorf("%v: annotations=%#v", tc.path, cmd.Annotations)
		}
		if cmd.Flags().Lookup("execute") == nil || cmd.Flags().Lookup("confirm") == nil {
			t.Errorf("%v: missing execute/confirm guard", tc.path)
		}
		if tc.irreversible {
			if cmd.Annotations["mutation_risk"] != "destructive" || cmd.Annotations["reversibility"] != "irreversible" || cmd.Flags().Lookup("acknowledge-irreversible") == nil {
				t.Errorf("%v: incomplete irreversible guard: annotations=%#v", tc.path, cmd.Annotations)
			}
		} else if cmd.Annotations["reversibility"] != "reversible" {
			t.Errorf("%v: reversible mutation not declared: %#v", tc.path, cmd.Annotations)
		}
	}
}

func TestRenderWatchlistPlanFormatsAndSafetyStates(t *testing.T) {
	t.Parallel()
	preview := watchlistservice.Plan{
		Kind: "securities_watchlist_change", Action: watchlistservice.GroupDelete,
		GroupID: 7, GroupName: "Long term", CurrentItemCount: 3,
		Irreversible: true, RequiresIrreversibleAcknowledgement: true,
		AffectedItems: []watchlistservice.PreviewItem{{ProductCode: "US.AAPL", Symbol: "AAPL", Name: "Apple"}},
		ConfirmToken:  "confirm-123",
	}
	for _, tc := range []struct {
		name   string
		format output.Format
		plan   watchlistservice.Plan
		wants  []string
		absent []string
	}{
		{name: "json", format: output.FormatJSON, plan: preview, wants: []string{`"action": "group_delete"`, `"confirm_token": "confirm-123"`, `"irreversible": true`, `"product_code": "US.AAPL"`}},
		{name: "csv", format: output.FormatCSV, plan: preview, wants: []string{"kind,action,group_id", "securities_watchlist_change,group_delete,7", "confirm-123", "US.AAPL (AAPL, Apple)"}},
		{name: "preview table", format: output.FormatTable, plan: preview, wants: []string{"Status:     preview", "irreversible deletion (3 current item(s))", "Affected:   US.AAPL (AAPL, Apple)", "--acknowledge-irreversible", "confirm-123"}},
		{name: "noop table", format: output.FormatTable, plan: watchlistservice.Plan{Action: watchlistservice.GroupRename, GroupID: 7, GroupName: "Long term", Noop: true}, wants: []string{"Status:     up to date"}, absent: []string{"Confirm:"}},
		{name: "reconciled table", format: output.FormatTable, plan: watchlistservice.Plan{Action: watchlistservice.ItemAdd, GroupID: 7, GroupName: "Long term", ProductCode: "US.AAPL", Applied: true, Reconciled: true}, wants: []string{"Status:     applied", "Product:    US.AAPL", "Verified:"}, absent: []string{"Confirm:"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := renderWatchlistPlan(&out, tc.format, tc.plan); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.wants {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %q", want, out.String())
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(out.String(), absent) {
					t.Errorf("unexpected %q in %q", absent, out.String())
				}
			}
		})
	}
}

func TestRenderWatchlistPlanRejectsUnsupportedFormatAndPropagatesWrites(t *testing.T) {
	t.Parallel()
	plan := watchlistservice.Plan{Action: watchlistservice.GroupCreate, ConfirmToken: "token"}
	if err := renderWatchlistPlan(&bytes.Buffer{}, output.Format("yaml"), plan); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported format error = %v", err)
	}
	for _, format := range []output.Format{output.FormatTable, output.FormatJSON, output.FormatCSV} {
		if err := renderWatchlistPlan(failingWatchlistWriter{}, format, plan); err == nil {
			t.Errorf("%s did not propagate writer failure", format)
		}
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
