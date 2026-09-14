package ops

import (
	"context"
	"net/http"
	"testing"
)

func TestCompletedOrdersAllDatesOperation(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/account/list" {
			_, _ = w.Write([]byte(`{"result":{"primaryKey":"test-account","accountList":[]}}`))
			return
		}
		if r.URL.Path != "/api/v3/trading/my-orders/completed" || r.URL.Query().Get("market") != "us" || r.URL.Query().Get("size") != "3" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"result":{"body":[],"lastPage":true,"pagingParam":{"number":1,"size":3,"key":null}}}`))
	}))
	catalog := NewCatalog()
	_, err := catalog.Call(context.Background(), deps, "completed_orders", map[string]any{"all_dates": true, "market": "us", "size": 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"from", "to"} {
		_, err := catalog.Call(context.Background(), deps, "completed_orders", map[string]any{"all_dates": true, date: "2026-09-14"})
		if err == nil {
			t.Fatalf("all_dates accepted %s", date)
		}
	}
	for _, param := range []string{"size", "page"} {
		for _, value := range []int{0, -1, 101} {
			_, err := catalog.Call(context.Background(), deps, "completed_orders", map[string]any{"all_dates": true, param: value})
			if err == nil {
				t.Fatalf("all_dates accepted %s=%d", param, value)
			}
		}
	}
	op, _ := catalog.Get("completed_orders")
	if op.Write || op.Probe == nil || !op.Probe.AccountScoped {
		t.Fatal("expected a read-only operation with an account-scoped probe")
	}
	if err := op.Probe.Check(200, []byte(`{"result":{"body":[],"lastPage":true,"pagingParam":{}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := op.Probe.Check(200, []byte(`{"result":{"body":null}}`)); err == nil {
		t.Fatal("probe accepted an incomplete response")
	}
}
