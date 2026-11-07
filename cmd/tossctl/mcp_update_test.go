package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
)

func seedLatest(t *testing.T, path, latest string) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"last_checked_at": time.Now(), "latest_version": latest})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A long-running MCP server must announce a release published while it is
// up — once — and never repeat the version it already announced.
func TestMCPUpdateNotifierAnnouncesEachReleaseOnce(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "update-check.json")
	seedLatest(t, cache, "0.56.0")
	clock := time.Now()
	n := newMCPUpdateNotifier(updatecheck.New(cache), "0.56.0", "`brew upgrade tossctl`")
	n.now = func() time.Time { return clock }
	ctx := context.Background()

	if got := n.startup(ctx); got != "" {
		t.Fatalf("no newer release at startup, got %q", got)
	}
	seedLatest(t, cache, "0.57.0") // released while the server runs
	if got := n.perCall(ctx); got != "" {
		t.Fatalf("within the look interval the cache is not consulted, got %q", got)
	}
	clock = clock.Add(mcpUpdateLookEvery + time.Minute)
	got := n.perCall(ctx)
	if !strings.Contains(got, "v0.57.0") || !strings.Contains(got, "v0.56.0") || !strings.Contains(got, "brew upgrade tossctl") {
		t.Fatalf("want a notice naming both versions and the upgrade command, got %q", got)
	}
	clock = clock.Add(mcpUpdateLookEvery + time.Minute)
	if again := n.perCall(ctx); again != "" {
		t.Fatalf("the same release must not be announced twice, got %q", again)
	}
}

func TestMCPUpdateNotifierStartupAnnouncesOnlyOnce(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "update-check.json")
	seedLatest(t, cache, "0.57.0")
	clock := time.Now()
	n := newMCPUpdateNotifier(updatecheck.New(cache), "0.56.0", "`tossctl update`")
	n.now = func() time.Time { return clock }
	if got := n.startup(context.Background()); !strings.Contains(got, "v0.57.0") {
		t.Fatalf("startup must announce a newer release, got %q", got)
	}
	clock = clock.Add(2 * mcpUpdateLookEvery)
	if got := n.perCall(context.Background()); got != "" {
		t.Fatalf("a release already in the instructions must not repeat on tool results, got %q", got)
	}
}
