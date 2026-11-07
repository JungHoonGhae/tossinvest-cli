package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
)

// mcpUpdateNotifier tells an MCP session about a newer release: once in the
// initialize instructions, and once on a later tool result when a release
// lands while the long-running server is up. Each version is announced at
// most once per process.
type mcpUpdateNotifier struct {
	checker *updatecheck.Checker
	current string
	hint    string
	now     func() time.Time

	mu        sync.Mutex
	announced string
	lastLook  time.Time
}

// mcpUpdateLookEvery bounds how often tool calls consult the update cache
// (the cache itself refetches at most every few hours).
const mcpUpdateLookEvery = time.Hour

func newMCPUpdateNotifier(checker *updatecheck.Checker, current, hint string) *mcpUpdateNotifier {
	return &mcpUpdateNotifier{checker: checker, current: current, hint: hint, now: time.Now}
}

func (n *mcpUpdateNotifier) message(latest string) string {
	return fmt.Sprintf("Update available: tossctl v%s (this server runs v%s). Tell the user they can update with %s, then restart this MCP server to pick it up.",
		latest, n.current, n.hint)
}

// startup returns the instructions line, if any.
func (n *mcpUpdateNotifier) startup(ctx context.Context) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lastLook = n.now()
	return n.announce(n.checker.LatestStable(ctx))
}

// perCall returns a notice for the current tool result, or "".
func (n *mcpUpdateNotifier) perCall(ctx context.Context) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.now().Sub(n.lastLook) < mcpUpdateLookEvery {
		return ""
	}
	n.lastLook = n.now()
	lookCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return n.announce(n.checker.LatestStable(lookCtx))
}

func (n *mcpUpdateNotifier) announce(latest string) string {
	if n.current == "dev" || !updatecheck.IsNewer(latest, n.current) || latest == n.announced {
		return ""
	}
	n.announced = latest
	return n.message(latest)
}
