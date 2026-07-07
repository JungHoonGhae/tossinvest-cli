package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestAdaptConditionalOrderUnit(t *testing.T) {
	raw := apiConditionalOrder{
		ConditionalOrderID: "co-1", Type: "SINGLE", Status: "WATCHING",
		Symbol: "005930", Market: "KR", Quantity: "10", OrderType: "LIMIT",
		ExpireDate: "2026-12-31", CreatedAt: "2026-07-08T09:00:00+09:00",
		First:  apiConditionalCondition{Type: "STOP", Status: "WATCHING", TriggerPrice: "70000", OrderPrice: "69900"},
		Second: nil,
	}
	got := adaptConditionalOrder(raw)
	if got.ID != "co-1" || got.Type != "SINGLE" || got.Quantity != 10 {
		t.Fatalf("meta: %+v", got)
	}
	if got.First.TriggerPrice != 70000 || got.First.OrderPrice != 69900 {
		t.Fatalf("first: %+v", got.First)
	}
	if got.Second != nil {
		t.Fatalf("second: want nil for SINGLE, got %+v", got.Second)
	}
}

func TestAdaptConditionalOrderOCO(t *testing.T) {
	second := &apiConditionalCondition{Type: "PROFIT_RATE", Status: "WATCHING", TargetProfitRate: "5.5"}
	raw := apiConditionalOrder{
		ConditionalOrderID: "co-2", Type: "OCO", Status: "WATCHING",
		Symbol: "005930", Market: "KR", Quantity: "5", OrderType: "MARKET", ExpireDate: "2026-12-31",
		First:  apiConditionalCondition{Type: "STOP", Status: "WATCHING", TriggerPrice: "68000"},
		Second: second,
	}
	got := adaptConditionalOrder(raw)
	if got.Second == nil {
		t.Fatalf("second: want non-nil for OCO")
	}
	if got.Second.TargetProfitRate != 5.5 {
		t.Fatalf("second rate: %v", got.Second.TargetProfitRate)
	}
}

func TestConditionalOrdersIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/conditional-orders":
			if r.URL.Query().Get("status") != "WATCHING" {
				t.Errorf("status: got %q", r.URL.Query().Get("status"))
			}
			_, _ = w.Write([]byte(`{"result":{"conditionalOrders":[{"conditionalOrderId":"co-1","type":"SINGLE","status":"WATCHING","symbol":"005930","market":"KR","quantity":"10","orderType":"LIMIT","expireDate":"2026-12-31","first":{"type":"STOP","status":"WATCHING","triggerPrice":"70000","targetProfitRate":null,"orderPrice":"69900","triggeredOrderId":null},"second":null,"createdAt":"2026-07-08T09:00:00+09:00"}],"nextCursor":null,"hasNext":false}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(1),
	)
	got, err := c.ConditionalOrders(context.Background(), "WATCHING", "", "", 0)
	if err != nil {
		t.Fatalf("ConditionalOrders: %v", err)
	}
	if len(got.Orders) != 1 || got.Orders[0].ID != "co-1" || got.Orders[0].Second != nil || got.HasNext {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Orders[0].First.TriggerPrice != 70000 {
		t.Fatalf("first trigger: %v", got.Orders[0].First.TriggerPrice)
	}
}
