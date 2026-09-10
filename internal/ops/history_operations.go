package ops

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/briefing"
	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
)

func historyOperations() []Operation {
	// Primitive values have already been validated by Catalog.Call.
	str := func(a map[string]any, k string) string { v, _ := argString(a, k); return v }
	num := func(a map[string]any, k string, def int) int {
		if _, ok := a[k]; !ok {
			return def
		}
		v, _ := argInt(a, k)
		return v
	}
	wrap := func(id, summary string, params []Param, fn func(context.Context, *history.Service, map[string]any) (any, error)) Operation {
		return Operation{ID: id, Method: "GET", Path: "local:" + id, Backend: "none", Category: "history", Summary: summary, Params: params, handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
			if d.History == nil {
				return nil, fmt.Errorf("local history service is not configured")
			}
			return fn(ctx, d.History, args)
		}}
	}
	idParam := Param{Name: "snapshot", Type: "integer", Desc: "local snapshot ID (default: latest); use history_list to discover IDs"}
	list := wrap("history_list", "List local WTS collections, including freshness, date coverage, and completeness. Offline; no credentials required.", []Param{{Name: "limit", Type: "integer", Desc: "maximum snapshots, newest first (1..1000; default 20)"}}, func(ctx context.Context, s *history.Service, a map[string]any) (any, error) {
		return s.Store.List(ctx, num(a, "limit", 20))
	})
	positions := wrap("history_positions", "Holdings from one local WTS collection. Historical observations, never live balances. Offline.", []Param{idParam}, func(ctx context.Context, s *history.Service, a map[string]any) (any, error) {
		return s.Store.Positions(ctx, int64(num(a, "snapshot", 0)))
	})
	transactions := wrap("history_transactions", "Search a local transaction collection by literal substring in symbol, stock name, category, or summary. Preserves currencies and coverage metadata. Collections are not merged; default latest. Offline.", []Param{idParam, {Name: "query", Type: "string", Desc: "literal search substring"}, {Name: "from", Type: "string", Desc: "start date YYYY-MM-DD within this collection"}, {Name: "to", Type: "string", Desc: "end date YYYY-MM-DD within this collection"}, {Name: "market", Type: "string", Desc: "kr|us; omit for both"}, {Name: "limit", Type: "integer", Desc: "1..1000; default 50"}, {Name: "offset", Type: "integer", Desc: "row offset; follow next_offset while has_more is true"}}, func(ctx context.Context, s *history.Service, a map[string]any) (any, error) {
		return s.Store.Transactions(ctx, history.Query{SnapshotID: int64(num(a, "snapshot", 0)), Search: str(a, "query"), From: str(a, "from"), To: str(a, "to"), Market: str(a, "market"), Limit: num(a, "limit", 50), Offset: num(a, "offset", 0)})
	})
	compare := wrap("history_compare", "Compare holdings in two local collections: added/removed positions, quantity and valuation deltas. Value changes include trades and FX and are not investment returns. Offline.", []Param{{Name: "before", Type: "integer", Required: true, Desc: "earlier snapshot ID"}, {Name: "after", Type: "integer", Required: true, Desc: "later snapshot ID (must exceed before)"}}, func(ctx context.Context, s *history.Service, a map[string]any) (any, error) {
		return s.Store.Compare(ctx, int64(num(a, "before", 0)), int64(num(a, "after", 0)))
	})
	sync := wrap("history_sync", "Preview a local SQLite append of current WTS holdings and dated transactions. Execute only with the fresh preview confirm token. Writes local history only; does not trade or change the account. Incomplete transaction paging is explicitly labeled.", []Param{{Name: "from", Type: "string", Desc: "YYYY-MM-DD; default to minus 30 days; at most 200 days inclusive"}, {Name: "to", Type: "string", Desc: "YYYY-MM-DD; default today in Korea"}, {Name: "market", Type: "string", Desc: "transaction markets all|kr|us (default all); holdings always include all session positions"}, {Name: "page_limit", Type: "integer", Desc: "maximum pages per market, 1..200 (default 20)"}, {Name: "execute", Type: "boolean", Desc: "false/omitted: preview; true: write local collection"}, {Name: "confirm", Type: "string", Desc: "confirm_token from the same preview (expires in 5 minutes)"}}, func(ctx context.Context, s *history.Service, a map[string]any) (any, error) {
		execute, _ := argBool(a, "execute")
		return s.Sync(ctx, history.SyncOptions{From: str(a, "from"), To: str(a, "to"), Market: str(a, "market"), PageLimit: num(a, "page_limit", 20)}, execute, str(a, "confirm"))
	})
	sync.Backend = "wts"
	sync.Path = "wts:history/sync"
	sync.Write = true
	sync.Method = "POST"
	sync.Mutation = reversiblePreferenceMutation("Atomic local transaction; reject changed account scope or snapshot revision; receipt reports stored snapshot ID and counts. No remote writes.")
	sync.ProbeRefs = []string{"account-list", "portfolio-positions", "transactions-kr", "transactions-us"}
	brief := Operation{ID: "portfolio_briefing", Method: "GET", Path: "wts:portfolio/briefing", Backend: "wts", Category: "portfolio", Summary: "One read combines current holdings with their upcoming earnings calls, newest holdings news, and matching pending orders. Partial failures are labeled per section; this is not an atomic account snapshot.", Params: []Param{{Name: "news_limit", Type: "integer", Desc: "newest holdings news articles, 1..50 (default 10)"}}, ProbeRefs: []string{"portfolio-positions", "pending-orders", "earning-call", "holdings-news"}, handler: func(ctx context.Context, d *Deps, a map[string]any) (any, error) {
		return briefing.Collect(ctx, d.WTS.Client, num(a, "news_limit", 10))
	}}
	return []Operation{list, positions, transactions, compare, sync, brief}
}
