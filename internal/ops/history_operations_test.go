package ops

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
)

func TestHistoryOperationsAndBriefingDependencies(t *testing.T) {
	c := NewCatalog()
	for id, want := range map[string][]string{"history_sync": {"account-list", "portfolio-positions", "transactions-kr", "transactions-us"}, "portfolio_briefing": {"portfolio-positions", "pending-orders", "earning-call", "holdings-news"}} {
		op, ok := c.Get(id)
		if !ok || op.Backend != "wts" {
			t.Fatalf("missing WTS operation %s", id)
		}
		for _, name := range want {
			if !slices.Contains(op.ProbeRefs, name) {
				t.Errorf("%s misses dependency %s", id, name)
			}
		}
	}
	deps := &Deps{History: history.New(filepath.Join(t.TempDir(), "history.sqlite"), nil)}
	result, err := c.Call(context.Background(), deps, "history_list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.([]history.Snapshot)) != 0 {
		t.Fatal("new history should be empty")
	}
	for id, args := range map[string]map[string]any{"history_list": {"limit": -1}, "history_transactions": {"offset": -1}, "history_compare": {"before": 2, "after": 1}} {
		if _, err := c.Call(context.Background(), deps, id, args); err == nil {
			t.Fatalf("invalid arguments accepted: %s", id)
		}
	}
	if _, err := c.Call(context.Background(), deps, "history_sync", map[string]any{"execute": true}); err == nil {
		t.Fatal("history sync must require WTS auth")
	}
}
