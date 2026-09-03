package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAssetPerformanceReadsVerifiedAllAccountChart(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "" {
			t.Fatalf("all-account request leaked accountKey header %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"hasKrStock":true,"hasKrStockInRange":false,"hasProduct":true,"hasProductInRange":true,"evaluatedAmountDiff":{"krw":1200,"usd":1.25},"maxEvaluated":{"baseDate":"2026-09-03","amount":{"krw":11000,"usd":11}},"minEvaluated":{"baseDate":"2026-09-01","amount":{"krw":9000,"usd":9}},"points":[{"baseDate":"2026-09-03","principalAmount":{"krw":8000,"usd":8},"evaluatedAmount":{"krw":11000,"usd":11},"profitLossRate":{"krw":0.375,"usd":0.375},"realtime":true}]}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAssetPerformance(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "all_accounts" || got.AccountScope != "" || got.Range != "ONE_MONTH" || got.StepUnit != "DAY" {
		t.Fatalf("scope metadata = %#v", got)
	}
	if !got.HasKRStock || got.HasKRStockInRange || !got.HasProduct || !got.HasProductInRange {
		t.Fatalf("availability flags = %#v", got)
	}
	if got.EvaluatedAmountDiff.KRW != 1200 || got.MaxEvaluated.Amount.USD != 11 || got.MinEvaluated.BaseDate != "2026-09-01" {
		t.Fatalf("summary = %#v", got)
	}
	if len(got.Points) != 1 || got.Points[0].BaseDate != "2026-09-03" || got.Points[0].ProfitLossRate.KRW != 0.375 || !got.Points[0].Realtime {
		t.Fatalf("points = %#v", got.Points)
	}
}

func TestGetAssetPerformanceScopesASelectedAccountWithoutExposingItsKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/asset-snapshot/chart/ONE_MONTH/DAY" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "selected-account" {
			t.Fatalf("accountKey = %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"evaluatedAmountDiff":{"krw":0,"usd":0},"maxEvaluated":{"baseDate":"2026-09-03","amount":{"krw":1,"usd":1}},"minEvaluated":{"baseDate":"2026-09-03","amount":{"krw":1,"usd":1}},"points":[]}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAssetPerformance(context.Background(), " selected-account ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "account" || got.AccountScope == "" || got.AccountScope == "selected-account" {
		t.Fatalf("unsafe account scope = %#v", got)
	}
}

func TestListAssetSnapshotsUsesVerifiedCursorContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/asset-snapshot/all-accounts/page" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("pageSize"); got != "2" {
			t.Fatalf("pageSize = %q", got)
		}
		if got := r.URL.Query().Get("cursorKey"); got != "cursor-1" {
			t.Fatalf("cursorKey = %q", got)
		}
		if got := r.Header.Get("accountKey"); got != "" {
			t.Fatalf("all-account request leaked accountKey header %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"body":[{"baseDate":"2026-09-02","principalAmount":{"krw":8000,"usd":8},"evaluatedAmount":{"krw":10000,"usd":10},"profitLossAmount":{"krw":2000,"usd":2},"profitLossRate":{"krw":0.25,"usd":0.25},"realtime":false,"evaluationComplete":true}],"nextCursorKey":"cursor-2"}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).ListAssetSnapshots(context.Background(), "", "cursor-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "all_accounts" || got.PageSize != 2 || got.NextCursor != "cursor-2" || !got.HasNext {
		t.Fatalf("page metadata = %#v", got)
	}
	if len(got.Snapshots) != 1 || got.Snapshots[0].ProfitLossAmount.USD != 2 || !got.Snapshots[0].EvaluationComplete {
		t.Fatalf("snapshots = %#v", got.Snapshots)
	}
}

func TestListAssetSnapshotsDefaultsPageSizeAndScopesSelectedAccount(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/asset-snapshot/page" || r.URL.Query().Get("pageSize") != "20" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "selected-account" {
			t.Fatalf("accountKey = %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"body":[],"nextCursorKey":""}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).ListAssetSnapshots(context.Background(), "selected-account", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "account" || got.AccountScope == "" || got.AccountScope == "selected-account" || got.PageSize != 20 || got.Snapshots == nil || got.HasNext {
		t.Fatalf("page = %#v", got)
	}
}

func TestListAssetSnapshotsRejectsNegativePageSizeBeforeRequest(t *testing.T) {
	t.Parallel()
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested = true
		_, _ = w.Write([]byte(`{"result":{"body":[]}}`))
	}))
	t.Cleanup(server.Close)

	if _, err := testClientFor(server).ListAssetSnapshots(context.Background(), "", "", -1); err == nil {
		t.Fatal("negative page size was accepted")
	}
	if requested {
		t.Fatal("invalid page size reached the API")
	}
}

func TestGetAssetSnapshotMapsVerifiedAllAccountDetail(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/asset-snapshot/all-accounts/detail-by-date" || r.URL.Query().Get("baseDate") != "2026-09-03" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "" {
			t.Fatalf("all-account request leaked accountKey header %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"baseDate":"2026-09-03","evaluationComplete":true,"principalAmount":{"krw":8000,"usd":8},"evaluatedAmount":{"krw":11000,"usd":11},"profitLossAmount":{"krw":3000,"usd":3},"profitLossRate":{"krw":0.375,"usd":0.375},"kr":{"principalAmount":{"krw":0,"usd":0},"evaluatedAmount":{"krw":0,"usd":0},"profitLossAmount":{"krw":0,"usd":0},"profitLossRate":{"krw":0,"usd":0},"items":[]},"option":{"principalAmount":{"krw":0,"usd":0},"evaluatedAmount":{"krw":0,"usd":0},"profitLossAmount":{"krw":0,"usd":0},"profitLossRate":{"krw":0,"usd":0},"items":[]},"us":{"principalAmount":{"krw":8000,"usd":8},"evaluatedAmount":{"krw":11000,"usd":11},"profitLossAmount":{"krw":3000,"usd":3},"profitLossRate":{"krw":0.375,"usd":0.375},"items":[{"productCode":"US.AAPL","isin":"US0378331005","symbol":"AAPL","productName":"Apple","quantity":2,"purchasePrice":{"krw":4000,"usd":4},"purchaseAmount":{"krw":8000,"usd":8},"evaluatedAmount":{"krw":11000,"usd":11},"profitLossAmount":{"krw":3000,"usd":3},"profitLossRate":{"krw":0.375,"usd":0.375},"marketDivision":"NASDAQ","logoImageUrl":"https://example.test/apple.png","type":"STOCK"}]},"bond":{"principalAmount":{"krw":0,"usd":0},"evaluatedAmount":{"krw":0,"usd":0},"profitLossAmount":{"krw":0,"usd":0},"profitLossRate":{"krw":0,"usd":0},"items":[]}}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAssetSnapshot(context.Background(), "", "2026-09-03")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "all_accounts" || got.BaseDate != "2026-09-03" || !got.EvaluationComplete || got.ProfitLossAmount.KRW != 3000 {
		t.Fatalf("detail = %#v", got)
	}
	if len(got.Markets) != 4 || got.Markets[0].Market != "kr" || got.Markets[1].Market != "option" || got.Markets[2].Market != "us" || got.Markets[3].Market != "bond" {
		t.Fatalf("market order = %#v", got.Markets)
	}
	if len(got.Markets[2].Holdings) != 1 || got.Markets[2].Holdings[0].Symbol != "AAPL" || got.Markets[2].Holdings[0].ISIN != "US0378331005" || got.Markets[2].Holdings[0].PurchaseAmount.USD != 8 {
		t.Fatalf("US holdings = %#v", got.Markets[2].Holdings)
	}
}

func TestGetAssetSnapshotScopesASelectedAccount(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/asset-snapshot/detail-by-date" || r.URL.Query().Get("baseDate") != "2026-09-03" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "selected-account" {
			t.Fatalf("accountKey = %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{"baseDate":"2026-09-03","kr":{"items":[]},"option":{"items":[]},"us":{"items":[]},"bond":{"items":[]}}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAssetSnapshot(context.Background(), "selected-account", "2026-09-03")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "account" || got.AccountScope == "" || got.AccountScope == "selected-account" {
		t.Fatalf("unsafe account scope = %#v", got)
	}
}

func TestGetAssetSnapshotRejectsInvalidDateBeforeRequest(t *testing.T) {
	t.Parallel()
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested = true
	}))
	t.Cleanup(server.Close)
	client := testClientFor(server)

	for _, date := range []string{"", "20260903", "2026-02-30"} {
		if _, err := client.GetAssetSnapshot(context.Background(), "", date); err == nil {
			t.Errorf("invalid date %q was accepted", date)
		}
	}
	if requested {
		t.Fatal("invalid date reached the API")
	}
}
