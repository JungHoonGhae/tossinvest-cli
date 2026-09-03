package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func paperTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(Config{
		HTTPClient: srv.Client(), CertBaseURL: srv.URL, InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "test"}, Headers: map[string]string{
			"App-Version": "v260903.1200", "Browser-Tab-Id": "paper-test",
		}},
	})
}

func TestPreviewPaperOrderResolvesExchangeAndRedactsOrderKey(t *testing.T) {
	t.Parallel()
	var prepared map[string]any
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/stock-infos/OPT_SPY260904C01000000_20260731":
			_, _ = w.Write([]byte(`{"result":{"symbol":"SPY260904C01000000","market":{"code":"AMX","displayName":"AMEX"}}}`))
		case "/api/v2/paper/trading/order/prepare":
			if err := json.NewDecoder(r.Body).Decode(&prepared); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"result":{"orderKey":"must-not-leak","authRequired":{"required":false},"preparedOrderInfo":{"quantity":1}}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	})

	got, err := c.PreviewPaperOrder(context.Background(), orderintent.OptionPlaceIntent{
		Symbol: "OPT_SPY260904C01000000_20260731", CurrencyMode: "USD",
		Side: "buy", OrderType: "limit", Price: 0.01, Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment != "paper" || got.Intent.Exchange != "AMX" || got.PreparedQuantity != 1 || got.AuthRequired {
		t.Fatalf("preview = %#v", got)
	}
	if prepared["market"] != "AMX" || prepared["withOrderKey"] != true {
		t.Fatalf("prepare body = %#v", prepared)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || strings.Contains(string(encoded), "must-not-leak") || strings.Contains(string(encoded), "order_key") {
		t.Fatalf("preview leaked order key: %s", encoded)
	}
}

func TestGetPaperStatusUsesOnlyPaperEndpoints(t *testing.T) {
	t.Parallel()
	seen := map[string]int{}
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path]++
		switch r.URL.Path {
		case "/api/v1/paper/cash-balance":
			_, _ = w.Write([]byte(`{"result":{"id":7,"deposit":1000.000000,"orderableAmount":850.000000,"withdrawableAmount":800.000000,"marginAmount":10.000000,"unsettledAmount":20.000000,"buyExecutionAmount":100.000000,"sellExecutionAmount":50.000000}}`))
		case "/api/v1/paper/education/summary":
			_, _ = w.Write([]byte(`{"result":{"lecture":{"totalSeconds":120,"requiredSeconds":60,"remainingSeconds":0,"completed":true},"paperTrading":{"totalSeconds":300,"requiredSeconds":180,"remainingSeconds":30,"completed":false},"allCompleted":false,"overseasDerivativeEligible":true}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	})

	got, err := c.GetPaperStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment != "paper" || got.Product != "us-options" || got.Balance.OrderableAmount != 850 {
		t.Fatalf("status = %#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"id":7`) {
		t.Fatalf("status leaked internal paper balance id: %s", encoded)
	}
	if got.Education.AllCompleted || !got.Education.OverseasDerivativeEligible || got.Education.PaperTrading.RemainingSeconds != 30 {
		t.Fatalf("education = %#v", got.Education)
	}
	if seen["/api/v1/paper/cash-balance"] != 1 || seen["/api/v1/paper/education/summary"] != 1 {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestGetPaperStatusRejectsMissingOrIncompleteResults(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		balance   string
		education string
	}{
		{name: "missing balance result", balance: `{}`},
		{name: "null balance result", balance: `{"result":null}`},
		{name: "incomplete balance result", balance: `{"result":{}}`},
		{
			name:      "incomplete education result",
			balance:   `{"result":{"deposit":0,"orderableAmount":0,"withdrawableAmount":0,"marginAmount":0,"unsettledAmount":0,"buyExecutionAmount":0,"sellExecutionAmount":0}}`,
			education: `{"result":{}}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/paper/cash-balance" {
					_, _ = w.Write([]byte(tc.balance))
					return
				}
				_, _ = w.Write([]byte(tc.education))
			})
			if _, err := c.GetPaperStatus(context.Background()); err == nil {
				t.Fatal("expected malformed paper status response to fail closed")
			}
		})
	}
}

func TestInitAndDepositPaperTradingStayOnPaperRoutes(t *testing.T) {
	t.Parallel()
	var bodies = map[string]map[string]any{}
	var initBody []byte
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path == "/api/v1/paper/init" {
			initBody, _ = io.ReadAll(r.Body)
		} else {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			bodies[r.URL.Path] = body
		}
		if r.URL.Path == "/api/v1/paper/deposit" {
			_, _ = w.Write([]byte(`{"result":true}`))
		} else {
			_, _ = w.Write([]byte(`{"result":{"message":"ok"}}`))
		}
	})

	if _, err := c.InitPaperTrading(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DepositPaperCash(context.Background(), 1000); err != nil {
		t.Fatal(err)
	}
	if len(initBody) != 0 {
		t.Fatalf("init body = %q, want empty", initBody)
	}
	if bodies["/api/v1/paper/deposit"]["amount"] != float64(1000) {
		t.Fatalf("deposit body = %#v", bodies["/api/v1/paper/deposit"])
	}
}

func TestPlacePaperOrderUsesDedicatedPrepareAndCreate(t *testing.T) {
	t.Parallel()
	var paths []string
	var bodies []map[string]any
	var createKey string
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		if r.URL.Path == "/api/v2/paper/trading/order/prepare" {
			_, _ = w.Write([]byte(`{"result":{"orderKey":"paper-key","authRequired":{"required":false},"preparedOrderInfo":{"quantity":1}}}`))
			return
		}
		createKey = r.Header.Get("X-Order-Key")
		_, _ = w.Write([]byte(`{"result":{"message":"accepted","orderDate":"2026-09-03","orderNo":12,"orderId":"paper-order"}}`))
	})

	got, err := c.PlacePaperOrder(context.Background(), orderintent.OptionPlaceIntent{
		Symbol: "OPT_SPY260904C00650000_20260724", Exchange: "AMX", CurrencyMode: "USD",
		Side: "buy", OrderType: "limit", Price: 0.05, Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/api/v2/paper/trading/order/prepare" || paths[1] != "/api/v2/paper/trading/order/create" {
		t.Fatalf("paths = %#v", paths)
	}
	if createKey != "paper-key" || got.OrderID != "paper-order" || got.Environment != "paper" {
		t.Fatalf("result = %#v key=%q", got, createKey)
	}
	if bodies[0]["withOrderKey"] != true {
		t.Fatalf("prepare body = %#v", bodies[0])
	}
	if _, ok := bodies[1]["withOrderKey"]; ok {
		t.Fatalf("create leaked withOrderKey: %#v", bodies[1])
	}
	for _, body := range bodies {
		if body["stockCode"] != "OPT_SPY260904C00650000_20260724" || body["market"] != "AMX" || body["orderPriceType"] != "00" || body["marginTrading"] != false {
			t.Fatalf("body = %#v", body)
		}
	}
}

func TestPlacePaperOrderRejectsIncompletePrepareBeforeCreate(t *testing.T) {
	t.Parallel()
	for _, response := range []string{
		`{}`,
		`{"result":null}`,
		`{"result":{}}`,
		`{"result":{"authRequired":{"required":false},"preparedOrderInfo":{"quantity":0}}}`,
	} {
		response := response
		t.Run(response, func(t *testing.T) {
			t.Parallel()
			createCalls := 0
			c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/paper/trading/order/create" {
					createCalls++
				}
				_, _ = w.Write([]byte(response))
			})
			_, err := c.PlacePaperOrder(context.Background(), orderintent.OptionPlaceIntent{
				Symbol: "OPT_SPY260904C01000000_20260731", Exchange: "AMX", CurrencyMode: "USD",
				Side: "buy", OrderType: "limit", Price: 0.01, Quantity: 1,
			})
			if err == nil || createCalls != 0 {
				t.Fatalf("err=%v createCalls=%d", err, createCalls)
			}
		})
	}
}

func TestPlacePaperOrderRejectsCreateWithoutOrderIdentifier(t *testing.T) {
	t.Parallel()
	for _, response := range []string{`{}`, `{"result":null}`, `{"result":{"message":"accepted"}}`} {
		response := response
		t.Run(response, func(t *testing.T) {
			t.Parallel()
			c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/paper/trading/order/prepare" {
					_, _ = w.Write([]byte(`{"result":{"authRequired":{"required":false},"preparedOrderInfo":{"quantity":1}}}`))
					return
				}
				_, _ = w.Write([]byte(response))
			})
			_, err := c.PlacePaperOrder(context.Background(), orderintent.OptionPlaceIntent{
				Symbol: "OPT_SPY260904C01000000_20260731", Exchange: "AMX", CurrencyMode: "USD",
				Side: "buy", OrderType: "limit", Price: 0.01, Quantity: 1,
			})
			if err == nil || !strings.Contains(err.Error(), "inspect pending and completed paper orders") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPlacePaperOrderAllowsPrepareWithoutLiveOrderKey(t *testing.T) {
	t.Parallel()
	var createKey string
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/paper/trading/order/prepare":
			_, _ = w.Write([]byte(`{"result":{"preparedOrderInfo":{"quantity":1},"authRequired":null}}`))
		case "/api/v2/paper/trading/order/create":
			createKey = r.Header.Get("X-Order-Key")
			_, _ = w.Write([]byte(`{"result":{"message":"accepted","orderDate":"2026-09-03","orderNo":13,"orderId":"paper-no-key"}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	})

	got, err := c.PlacePaperOrder(context.Background(), orderintent.OptionPlaceIntent{
		Symbol: "OPT_SPY260904C01000000_20260731", Exchange: "AMX", CurrencyMode: "USD",
		Side: "buy", OrderType: "limit", Price: 0.01, Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != "paper-no-key" || createKey != "" {
		t.Fatalf("result=%#v key=%q", got, createKey)
	}
}

func TestCancelPaperOrderPreparesThenExecutesWithOrderKey(t *testing.T) {
	t.Parallel()
	var paths []string
	var executeKey string
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/paper/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"orderId":"paper-order","orderNo":12,"orderedAt":"2026-09-03T10:00:00","stockCode":"OPT1","tradeType":"buy","pendingQuantity":1,"isAfterMarketOrder":false,"isReservationOrder":false}]}`))
		case "/api/v2/paper/trading/order/cancel/prepare/2026-09-03/12":
			_, _ = w.Write([]byte(`{"result":{"orderKey":"cancel-key"}}`))
		case "/api/v3/paper/trading/order/cancel/2026-09-03/12":
			executeKey = r.Header.Get("X-Order-Key")
			_, _ = w.Write([]byte(`{"result":{"message":"cancelled","orderDate":"2026-09-03","orderNo":12,"orderId":"paper-order"}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	})

	got, err := c.CancelPaperOrder(context.Background(), "paper-order")
	if err != nil {
		t.Fatal(err)
	}
	if executeKey != "cancel-key" || got.OrderID != "paper-order" || len(paths) != 3 {
		t.Fatalf("result=%#v key=%q paths=%#v", got, executeKey, paths)
	}
}

func TestCancelPaperOrderAllowsPrepareWithoutLiveOrderKey(t *testing.T) {
	t.Parallel()
	var executeKey string
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/paper/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"orderId":"paper-order","orderNo":12,"orderedAt":"2026-09-03T10:00:00","stockCode":"OPT1","tradeType":"buy","pendingQuantity":1,"isAfterMarketOrder":true}]}`))
		case "/api/v2/paper/trading/order/cancel/prepare/2026-09-03/12":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"authRequired":null}}`))
		case "/api/v3/paper/trading/order/cancel/2026-09-03/12":
			executeKey = r.Header.Get("X-Order-Key")
			_, _ = w.Write([]byte(`{"result":{"message":"cancelled","orderDate":"2026-09-03","orderNo":12,"orderId":"paper-order"}}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	})

	got, err := c.CancelPaperOrder(context.Background(), "paper-order")
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != "paper-order" || executeKey != "" {
		t.Fatalf("result=%#v key=%q", got, executeKey)
	}
}

func TestBulkCancelPaperOrdersUsesOnlyDedicatedPaperRoutes(t *testing.T) {
	t.Parallel()
	var paths []string
	var executeBody map[string]any
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/api/v1/paper/trading/orders/histories/all/pending" {
			_, _ = w.Write([]byte(`{"result":[{"orderId":"one","orderNo":1,"orderedAt":"2026-09-03T10:00:00","stockCode":"OPT1","tradeType":"buy","pendingQuantity":1,"isAfterMarketOrder":false,"isReservationOrder":true},{"orderId":"two","orderNo":2,"orderedAt":"2026-09-03T10:01:00","stockCode":"OPT2","tradeType":"sell","pendingQuantity":2}]}`))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.Path == "/api/v3/paper/trading/order/bulk-cancel/prepare" {
			_, _ = w.Write([]byte(`{"result":{}}`))
			return
		}
		executeBody = body
		_, _ = w.Write([]byte(`{"result":{"failedCancelCount":0}}`))
	})

	got, err := c.BulkCancelPaperOrders(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestedCount != 2 || got.FailedCount != 0 || len(paths) != 3 {
		t.Fatalf("result=%#v paths=%#v", got, paths)
	}
	orders, ok := executeBody["orderCancels"].([]any)
	if !ok || len(orders) != 2 {
		t.Fatalf("execute body = %#v", executeBody)
	}
	first, ok := orders[0].(map[string]any)
	if !ok || first["isAfterMarketOrder"] != false || first["isReservationOrder"] != true {
		t.Fatalf("bulk flags drifted: %#v", orders[0])
	}
}

func TestListPaperCompletedOrdersDecodesOptionAmounts(t *testing.T) {
	t.Parallel()
	c := paperTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/paper/trading/my-orders/markets/us-opt/by-date/completed" {
			http.Error(w, "unexpected", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"body":[{"orderedAt":"2026-09-03 20:07:31.000","orderNo":1,"orderId":"paper-1","stockCode":"OPT_SPY","stockName":"SPY call","tradeType":"buy","orderQuantity":1.000000,"orderPrice":{"krw":14,"usd":0.010000},"executedQuantity":0,"averageExecutionPrice":{"krw":0,"usd":0},"userOrderDate":"2026-09-03","status":"취소","afterMarketOrder":false}],"lastPage":true,"pagingParam":{"number":2,"size":10,"key":"-1"}}}`))
	})

	got, err := c.ListPaperCompletedOrders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "paper-1" || got[0].OrderPriceUSD != 0.01 || got[0].OrderPriceKRW != 14 || got[0].Status != "취소" {
		t.Fatalf("orders = %#v", got)
	}
}
