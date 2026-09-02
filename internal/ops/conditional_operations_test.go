package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

func TestConditionalOrdersOperationCallsOfficialClient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/conditional-orders":
			if got := r.URL.Query().Get("status"); got != "OPEN" {
				t.Errorf("status = %q, want OPEN", got)
			}
			_, _ = w.Write([]byte(`{"result":{"conditionalOrders":[{"conditionalOrderId":"co-1","type":"SINGLE","status":"WATCHING","symbol":"005930","market":"KR","quantity":"1","orderType":"LIMIT","expireDate":"2026-12-31","first":{"type":"STOP","status":"WATCHING","triggerPrice":"70000","orderPrice":"69900"},"second":null,"createdAt":"2026-09-02T09:00:00+09:00"}],"hasNext":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := official.New(official.Credentials{APIKey: "key", SecretKey: "secret"}, filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL), official.WithHTTPClient(srv.Client()), official.WithAccountSeq(1))
	deps := &Deps{Client: client, Auth: AuthStatus{Official: BackendStatus{Connected: true}}}
	result, err := NewCatalog().Call(context.Background(), deps, "conditional_orders", map[string]any{"status": "OPEN"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	orders, ok := result.(domain.ConditionalOrderList)
	if !ok || len(orders.Orders) != 1 || orders.Orders[0].ID != "co-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestConditionalOrderOperationCallsOfficialClient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/conditional-orders/co-1":
			_, _ = w.Write([]byte(`{"result":{"conditionalOrderId":"co-1","type":"SINGLE","status":"WATCHING","symbol":"005930","market":"KR","quantity":"1","orderType":"LIMIT","expireDate":"2026-12-31","first":{"type":"STOP","status":"WATCHING","triggerPrice":"70000","orderPrice":"69900"},"second":null,"createdAt":"2026-09-02T09:00:00+09:00"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := official.New(official.Credentials{APIKey: "key", SecretKey: "secret"}, filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL), official.WithHTTPClient(srv.Client()), official.WithAccountSeq(1))
	deps := &Deps{Client: client, Auth: AuthStatus{Official: BackendStatus{Connected: true}}}
	result, err := NewCatalog().Call(context.Background(), deps, "conditional_order", map[string]any{"conditional_order_id": "co-1"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	order, ok := result.(domain.ConditionalOrder)
	if !ok || order.ID != "co-1" || order.First.TriggerPrice != 70000 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
