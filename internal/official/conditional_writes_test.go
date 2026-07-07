package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCancelConditionalOrderIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/conditional-orders/co-1":
			if r.Header.Get("X-Tossinvest-Account") != "1" {
				t.Errorf("account header: got %q", r.Header.Get("X-Tossinvest-Account"))
			}
			_, _ = w.Write([]byte(`{"result":null}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
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
	if err := c.CancelConditionalOrder(context.Background(), "co-1"); err != nil {
		t.Fatalf("CancelConditionalOrder: %v", err)
	}
}

func TestCreateConditionalOrderIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/conditional-orders":
			if r.Header.Get("X-Tossinvest-Account") != "1" {
				t.Errorf("account header: got %q", r.Header.Get("X-Tossinvest-Account"))
			}
			var got map[string]any
			_ = json.NewDecoder(r.Body).Decode(&got)
			if got["symbol"] != "005930" || got["quantity"] != "10" || got["type"] != "SINGLE" {
				t.Errorf("body: %+v", got)
			}
			first, _ := got["first"].(map[string]any)
			if first["orderSide"] != "SELL" || first["triggerPrice"] != "70000" {
				t.Errorf("first leg: %+v", first)
			}
			_, _ = w.Write([]byte(`{"result":{"conditionalOrderId":"co-99","clientOrderId":null}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
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
	ref, err := c.CreateConditionalOrder(context.Background(), ConditionalCreateBody{
		Symbol: "005930", Type: "SINGLE", Quantity: "10", OrderType: "LIMIT", ExpireDate: "2026-12-31",
		First: ConditionLegBody{OrderSide: "SELL", TriggerPrice: "70000", OrderPrice: "69900"},
	})
	if err != nil {
		t.Fatalf("CreateConditionalOrder: %v", err)
	}
	if ref.ID != "co-99" {
		t.Fatalf("ref: %+v", ref)
	}
}
