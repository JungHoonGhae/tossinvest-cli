package ops

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestAssetSnapshotOperationsAreCataloguedWithAllDependencies(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{"portfolio_performance", "portfolio_snapshots", "portfolio_snapshot"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Errorf("operation %q missing", id)
			continue
		}
		if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 1 {
			t.Errorf("operation %q metadata = %#v", id, op)
		}
		if !op.ExtraProbes[0].AccountScoped || !slices.Contains(op.ProbeRefs, "account-list") {
			t.Errorf("operation %q does not monitor its account-scoped variant: %#v", id, op)
		}
	}
}

func TestAssetSnapshotOperationsAreCallableThroughThePublicCatalog(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("accountKey"); got != "" {
			t.Fatalf("all-account request leaked accountKey header %q", got)
		}
		switch r.URL.Path {
		case "/api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY":
			_, _ = w.Write([]byte(`{"result":{"hasKrStock":true,"hasKrStockInRange":true,"hasProduct":true,"hasProductInRange":true,"evaluatedAmountDiff":{"krw":1200,"usd":1},"maxEvaluated":{"baseDate":"2026-09-03","amount":{"krw":11200,"usd":8}},"minEvaluated":{"baseDate":"2026-09-02","amount":{"krw":10000,"usd":7}},"points":[{"baseDate":"2026-09-03","principalAmount":{"krw":10000,"usd":7},"evaluatedAmount":{"krw":11200,"usd":8},"profitLossAmount":{"krw":1200,"usd":1},"profitLossRate":{"krw":12,"usd":14.2},"realtime":true}]}}`))
		case "/api/v1/asset-snapshot/all-accounts/page":
			if r.URL.Query().Get("pageSize") != "2" || r.URL.Query().Get("cursorKey") != "cursor-1" {
				t.Fatalf("snapshot page query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"result":{"body":[{"baseDate":"2026-09-02","evaluatedAmount":{"krw":10000,"usd":7},"evaluationComplete":true}],"nextCursorKey":"cursor-2"}}`))
		case "/api/v1/asset-snapshot/all-accounts/detail-by-date":
			if r.URL.Query().Get("baseDate") != "2026-09-02" {
				t.Fatalf("snapshot detail query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"result":{"baseDate":"2026-09-02","evaluationComplete":true,"principalAmount":{"krw":9000,"usd":6},"evaluatedAmount":{"krw":10000,"usd":7},"profitLossAmount":{"krw":1000,"usd":1},"profitLossRate":{"krw":11.1,"usd":16.6},"kr":{"items":[]},"option":{"items":[]},"us":{"items":[{"productCode":"US-AAPL","symbol":"AAPL","productName":"Apple","quantity":1,"evaluatedAmount":{"krw":10000,"usd":7}}]},"bond":{"items":[]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	catalog := NewCatalog()

	performanceAny, err := catalog.Call(context.Background(), deps, "portfolio_performance", nil)
	if err != nil {
		t.Fatal(err)
	}
	performance := performanceAny.(domain.AssetPerformance)
	if performance.Scope != "all_accounts" || performance.Range != "ONE_MONTH" || len(performance.Points) != 1 || performance.Points[0].EvaluatedAmount.KRW != 11200 {
		t.Fatalf("portfolio_performance result = %#v", performance)
	}

	pageAny, err := catalog.Call(context.Background(), deps, "portfolio_snapshots", map[string]any{"cursor": "cursor-1", "limit": 2})
	if err != nil {
		t.Fatal(err)
	}
	page := pageAny.(domain.AssetSnapshotPage)
	if page.Scope != "all_accounts" || len(page.Snapshots) != 1 || page.NextCursor != "cursor-2" || !page.HasNext {
		t.Fatalf("portfolio_snapshots result = %#v", page)
	}

	detailAny, err := catalog.Call(context.Background(), deps, "portfolio_snapshot", map[string]any{"date": "2026-09-02"})
	if err != nil {
		t.Fatal(err)
	}
	detail := detailAny.(domain.AssetSnapshotDetail)
	if detail.Scope != "all_accounts" || detail.BaseDate != "2026-09-02" || len(detail.Markets) != 4 || len(detail.Markets[2].Holdings) != 1 || detail.Markets[2].Holdings[0].Symbol != "AAPL" {
		t.Fatalf("portfolio_snapshot result = %#v", detail)
	}
}

func TestAssetSnapshotProbesAcceptVerifiedTerminalPageSchemas(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{"portfolio_snapshots"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Fatalf("operation %q missing", id)
		}
		for _, probe := range append([]ProbeSpec{*op.Probe}, op.ExtraProbes...) {
			if err := probe.Check(http.StatusOK, []byte(`{"result":{"body":[],"nextCursorKey":null}}`)); err != nil {
				t.Errorf("%s rejected a terminal cursor page: %v", probe.Name, err)
			}
		}
	}
}
