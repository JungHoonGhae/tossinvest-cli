package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	tradingflow "github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

func TestCancelPendingOrder(t *testing.T) {
	t.Parallel()

	var paths []string
	var bodies []string
	var browserTabIDs []string
	var accountHeaders []string
	var appVersions []string
	var orderKeys []string
	pendingCalls := 0
	today := time.Now().Format("2006-01-02")
	now := time.Now().Format("2006-01-02 15:04:05.000")
	preparePath := "/api/v2/wts/trading/order/cancel/prepare/" + today + "/14"
	cancelPath := "/api/v3/wts/trading/order/cancel/" + today + "/14"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/trading/orders/histories/all/pending" {
			pendingCalls++
			if pendingCalls == 1 {
				_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"` + today + `","orderNo":14,"tradeType":"buy","orderPrice":700,"orderUsdPrice":0.4753,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
			return
		}
		paths = append(paths, r.URL.Path)
		browserTabIDs = append(browserTabIDs, r.Header.Get("Browser-Tab-Id"))
		accountHeaders = append(accountHeaders, r.Header.Get("X-Tossinvest-Account"))
		appVersions = append(appVersions, r.Header.Get("App-Version"))
		orderKeys = append(orderKeys, r.Header.Get("X-Order-Key"))
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case preparePath:
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::cancel","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case cancelPath:
			_, _ = w.Write([]byte(`{"result":{"message":"취소 되었어요.","orderDate":"` + today + `","orderNo":14,"orderId":"test-order-id"}}`))
		case "/api/v2/trading/my-orders/markets/us/by-date/completed":
			_, _ = io.WriteString(w, `{"result":{"body":[{"orderedAt":"`+now+`","lastExecutedAt":"`+now+`","orderNo":15,"orderId":"completed-order-id","stockCode":"US20220809012","stockName":"TSLL","symbol":"TSLL","tradeType":"buy","status":"취소","orderQuantity":1,"executedQuantity":0,"userOrderDate":"`+today+`","orderPrice":{"krw":700},"averageExecutionPrice":{"krw":0}}]}}`)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.1636"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizeCancel("14", "TSLL")
	if err != nil {
		t.Fatalf("NormalizeCancel returned error: %v", err)
	}

	result, err := client.CancelPendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("CancelPendingOrder returned error: %v", err)
	}
	if result.Status != "canceled" {
		t.Fatalf("expected canceled result, got %q", result.Status)
	}
	if result.CurrentOrderID != today+"/15" {
		t.Fatalf("expected current order id %s/15, got %q", today, result.CurrentOrderID)
	}

	if len(paths) != 3 {
		t.Fatalf("expected 3 non-pending requests, got %d", len(paths))
	}
	if paths[0] != preparePath {
		t.Fatalf("unexpected prepare path: %s", paths[0])
	}
	if paths[1] != cancelPath {
		t.Fatalf("unexpected cancel path: %s", paths[1])
	}
	if browserTabIDs[0] != "browser-tab-test123" || browserTabIDs[1] != "browser-tab-test123" {
		t.Fatalf("unexpected browser-tab-id headers: %#v", browserTabIDs)
	}
	if accountHeaders[0] != "1" || accountHeaders[1] != "1" {
		t.Fatalf("unexpected account headers: %#v", accountHeaders)
	}
	if appVersions[0] != "v260311.1636" || appVersions[1] != "v260311.1636" {
		t.Fatalf("unexpected app-version headers: %#v", appVersions)
	}
	if orderKeys[0] != "" {
		t.Fatalf("prepare request should not include x-order-key: %#v", orderKeys)
	}
	if orderKeys[1] != "trade::session::test::cancel" {
		t.Fatalf("unexpected final x-order-key header: %#v", orderKeys)
	}

	var gotPrepare map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &gotPrepare); err != nil {
		t.Fatalf("prepare body was not valid json: %v", err)
	}
	if gotPrepare["stockCode"] != "US20220809012" || gotPrepare["tradeType"] != "buy" || gotPrepare["withOrderKey"] != true {
		t.Fatalf("unexpected prepare body: %#v", gotPrepare)
	}

	var gotCancel map[string]any
	if err := json.Unmarshal([]byte(bodies[1]), &gotCancel); err != nil {
		t.Fatalf("cancel body was not valid json: %v", err)
	}
	if _, ok := gotCancel["withOrderKey"]; ok {
		t.Fatalf("cancel body should not include withOrderKey: %#v", gotCancel)
	}
	if gotCancel["stockCode"] != "US20220809012" || gotCancel["tradeType"] != "buy" {
		t.Fatalf("unexpected cancel body: %#v", gotCancel)
	}
}

func TestGetOrderAvailableActionsUsesResolvedPendingOrderID(t *testing.T) {
	t.Parallel()

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/trading/orders/histories/all/pending" {
			_, _ = w.Write([]byte(`{"result":[{"orderId":"broker/order+raw==","stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":14,"tradeType":"buy","orderPrice":700,"orderUsdPrice":0.4753,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
			return
		}

		requestedPath = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"enabled":true}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.1636"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	if _, err := client.GetOrderAvailableActions(context.Background(), "2026-03-11/14"); err != nil {
		t.Fatalf("GetOrderAvailableActions returned error: %v", err)
	}

	want := "/api/v3/trading/order/broker%2Forder+raw==/available-actions?fractional=false&isReservationOrder=false&orderPriceType=00&stockCode=US20220809012&tradeType=buy"
	if requestedPath != want {
		t.Fatalf("unexpected available-actions path:\nwant: %s\ngot:  %s", want, requestedPath)
	}
}

func TestGetOrderAvailableActionsTreats400AsSoftFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/trading/orders/histories/all/pending" {
			_, _ = w.Write([]byte(`{"result":[{"orderId":"broker/order+raw==","stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":14,"tradeType":"buy","orderPrice":700,"orderUsdPrice":0.4753,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"unsupported"}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.1636"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	result, err := client.GetOrderAvailableActions(context.Background(), "2026-03-11/14")
	if err != nil {
		t.Fatalf("GetOrderAvailableActions returned error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result on soft failure, got %#v", result)
	}
}

func TestCancelPendingOrderReturnsCompletedHistoryRollover(t *testing.T) {
	t.Parallel()

	today := time.Now().Format("2006-01-02")
	now := time.Now().Format("2006-01-02 15:04:05.000")
	preparePath := "/api/v2/wts/trading/order/cancel/prepare/" + today + "/14"
	cancelPath := "/api/v3/wts/trading/order/cancel/" + today + "/14"
	pendingCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/trading/orders/histories/all/pending":
			pendingCalls++
			if pendingCalls == 1 {
				_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"` + today + `","orderNo":14,"tradeType":"buy","orderPrice":700,"orderUsdPrice":0.4753,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
		case preparePath:
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::cancel","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case cancelPath:
			_, _ = w.Write([]byte(`{"result":{"message":"취소 되었어요.","orderDate":"` + today + `","orderNo":14,"orderId":"test-order-id"}}`))
		case "/api/v2/trading/my-orders/markets/us/by-date/completed":
			_, _ = io.WriteString(w, `{"result":{"body":[{"orderedAt":"`+now+`","lastExecutedAt":"`+now+`","orderNo":15,"orderId":"completed-cancel-order-id","stockCode":"US20220809012","stockName":"TSLL","symbol":"TSLL","tradeType":"buy","status":"취소","orderQuantity":1,"executedQuantity":0,"userOrderDate":"`+today+`","orderPrice":{"krw":700},"averageExecutionPrice":{"krw":0}}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.1636"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizeCancel(today+"/14", "TSLL")
	if err != nil {
		t.Fatalf("NormalizeCancel returned error: %v", err)
	}

	result, err := client.CancelPendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("CancelPendingOrder returned error: %v", err)
	}
	if result.Status != "canceled" {
		t.Fatalf("expected canceled result, got %q", result.Status)
	}
	if result.OriginalOrderID != today+"/14" {
		t.Fatalf("expected original order id %s/14, got %q", today, result.OriginalOrderID)
	}
	if result.CurrentOrderID != today+"/15" {
		t.Fatalf("expected current order id %s/15, got %q", today, result.CurrentOrderID)
	}
	if result.OrderID != today+"/15" {
		t.Fatalf("expected order id %s/15, got %q", today, result.OrderID)
	}
}

func TestBuildAmendBodyMatchesCapturedShape(t *testing.T) {
	order := pendingOrderDetails{
		OrderNo:            "13",
		OrderedDate:        "2026-03-11",
		StockCode:          "US20220809012",
		TradeType:          "buy",
		OrderPrice:         600,
		OrderUSDPrice:      0.4074,
		Quantity:           1,
		PendingQuantity:    1,
		OrderPriceTypeCode: "00",
	}
	price := 700.0

	body, expectedPriceKRW, expectedQty, err := buildAmendBody(order, "NSQ", 1472.8, nil, &price, true)
	if err != nil {
		t.Fatalf("buildAmendBody returned error: %v", err)
	}
	if expectedPriceKRW != 700 {
		t.Fatalf("expected KRW price 700, got %v", expectedPriceKRW)
	}
	if expectedQty != 1 {
		t.Fatalf("expected qty 1, got %v", expectedQty)
	}

	expected := `{"agreedOver100Million":false,"currencyMode":"KRW","isReservationOrder":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"00","price":0.48,"quantity":1,"stockCode":"US20220809012","tradeType":"buy","withOrderKey":true}`
	if string(body) != expected {
		t.Fatalf("unexpected amend prepare body:\nwant: %s\ngot:  %s", expected, string(body))
	}

	body, _, _, err = buildAmendBody(order, "NSQ", 1472.8, nil, &price, false)
	if err != nil {
		t.Fatalf("buildAmendBody returned error: %v", err)
	}
	expected = `{"agreedOver100Million":false,"currencyMode":"KRW","isReservationOrder":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"00","price":0.48,"quantity":1,"stockCode":"US20220809012","tradeType":"buy"}`
	if string(body) != expected {
		t.Fatalf("unexpected amend body:\nwant: %s\ngot:  %s", expected, string(body))
	}
}

func TestCancelPendingOrderReturnsInteractiveAuthRequired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/trading/orders/histories/all/pending" {
			_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":14,"tradeType":"buy","orderPrice":700,"orderUsdPrice":0.4753,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/v2/wts/trading/order/cancel/prepare/2026-03-11/14":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::cancel","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v3/wts/trading/order/cancel/2026-03-11/14":
			_, _ = w.Write([]byte(`{"result":{"uuid":"challenge","modulus":"abc","exponent":"10001","keyboard":"<svg/>"}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.1636"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizeCancel("14", "TSLL")
	if err != nil {
		t.Fatalf("NormalizeCancel returned error: %v", err)
	}

	_, err = client.CancelPendingOrder(context.Background(), intent)
	if err == nil {
		t.Fatal("expected auth challenge error")
	}
	if err != tradingflow.ErrInteractiveAuthRequired {
		t.Fatalf("expected ErrInteractiveAuthRequired, got %v", err)
	}
}

func TestBuildPlaceBodyMatchesCapturedShape(t *testing.T) {
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	meta := stockPriceMetadata{
		Close:        14.4,
		CloseKRW:     21208,
		ExchangeRate: 1472.8,
	}

	body, err := buildPlaceBody("US20220809012", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}
	expected := `{"agreedOver100Million":false,"allowAutoExchange":true,"currencyMode":"KRW","isReservationOrder":false,"marginTrading":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"00","price":0.34,"quantity":1,"stockCode":"US20220809012","tradeType":"buy","withOrderKey":true}`
	if string(body) != expected {
		t.Fatalf("unexpected place prepare body:\nwant: %s\ngot:  %s", expected, string(body))
	}

	body, err = buildPlaceBody("US20220809012", "NSQ", intent, meta, false)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}
	expected = `{"agreedOver100Million":false,"allowAutoExchange":true,"currencyMode":"KRW","extra":{"close":14.4,"closeKrw":21208,"exchangeRate":1472.8,"orderMethod":"종목상세__주문하기"},"isReservationOrder":false,"marginTrading":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"00","price":0.34,"quantity":1,"stockCode":"US20220809012","tradeType":"buy"}`
	if string(body) != expected {
		t.Fatalf("unexpected place create body:\nwant: %s\ngot:  %s", expected, string(body))
	}
}

func TestBuildPlaceBodyFractionalSellSendsDecimalQuantity(t *testing.T) {
	// Official Open API (≥1.1.5): US MARKET SELL accepts a decimal `quantity`.
	// tossctl sends it through the `quantity` field with orderAmount=0 and the
	// fractional/market flags, unlike fractional BUY which is amount-based.
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:     "TSLL",
		Market:     "us",
		Side:       "sell",
		Quantity:   0.5,
		Fractional: true,
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	meta := stockPriceMetadata{Close: 14.4, CloseKRW: 21208, ExchangeRate: 1472.8}

	body, err := buildPlaceBody("US20220809012", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}
	expected := `{"agreedOver100Million":false,"allowAutoExchange":true,"currencyMode":"KRW","isFractionalOrder":true,"isReservationOrder":false,"marginTrading":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"01","price":0,"quantity":0.5,"stockCode":"US20220809012","tradeType":"sell","withOrderKey":true}`
	if string(body) != expected {
		t.Fatalf("unexpected fractional-sell body:\nwant: %s\ngot:  %s", expected, string(body))
	}
}

func TestBuildPlaceBodySellTradeType(t *testing.T) {
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "sell",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	meta := stockPriceMetadata{
		Close:        14.4,
		CloseKRW:     21208,
		ExchangeRate: 1472.8,
	}

	body, err := buildPlaceBody("US20220809012", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}
	expected := `{"agreedOver100Million":false,"allowAutoExchange":true,"currencyMode":"KRW","isReservationOrder":false,"marginTrading":false,"market":"NSQ","max":false,"openPriceSinglePriceYn":false,"orderAmount":0,"orderPriceType":"00","price":0.34,"quantity":1,"stockCode":"US20220809012","tradeType":"sell","withOrderKey":true}`
	if string(body) != expected {
		t.Fatalf("unexpected sell prepare body:\nwant: %s\ngot:  %s", expected, string(body))
	}
}

func TestBuildPlaceBodyUSDLimitPriceAsIs(t *testing.T) {
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "MRVL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        158.01,
		CurrencyMode: "USD",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	// ExchangeRate deliberately non-1 to prove the USD path ignores it.
	meta := stockPriceMetadata{Close: 158.01, CloseKRW: 233000, ExchangeRate: 1477.6}

	body, err := buildPlaceBody("US20000627001", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	// USD input: price must be sent as-is, not divided by ExchangeRate.
	if payload["price"] != 158.01 {
		t.Fatalf("expected USD price 158.01 as-is, got %v", payload["price"])
	}
	// Wire payload must keep currencyMode=KRW (Toss spec) even when CLI input was USD.
	if payload["currencyMode"] != "KRW" {
		t.Fatalf("expected wire currencyMode KRW, got %v", payload["currencyMode"])
	}
	if payload["orderPriceType"] != "00" {
		t.Fatalf("expected limit orderPriceType 00, got %v", payload["orderPriceType"])
	}
	if payload["allowAutoExchange"] != true {
		t.Fatal("expected allowAutoExchange=true for US orders")
	}
}

func TestBuildPlaceBodyKRRawKRWPrice(t *testing.T) {
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "290080",
		Market:       "kr",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        8000,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	meta := stockPriceMetadata{
		Close:        8355,
		CloseKRW:     8355,
		ExchangeRate: 1,
	}

	body, err := buildPlaceBody("A290080", "KSP", intent, meta, true)
	if err != nil {
		t.Fatalf("buildPlaceBody returned error: %v", err)
	}

	// KR: raw KRW price, no allowAutoExchange
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if payload["price"] != float64(8000) {
		t.Fatalf("expected raw KRW price 8000, got %v", payload["price"])
	}
	if _, hasAutoExchange := payload["allowAutoExchange"]; hasAutoExchange {
		t.Fatal("expected no allowAutoExchange field for KR orders")
	}
	if payload["market"] != "KSP" {
		t.Fatalf("expected market KSP, got %v", payload["market"])
	}
	if payload["tradeType"] != "buy" {
		t.Fatalf("expected tradeType buy, got %v", payload["tradeType"])
	}
}

func TestBuildPlaceBodyFractionalMarketOrder(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol: "TSLL", Market: "us", Side: "buy", OrderType: "market",
		Amount: 18000, CurrencyMode: "KRW", Fractional: true,
	}
	meta := stockPriceMetadata{Close: 12.12, CloseKRW: 18000, ExchangeRate: 1500}
	body, err := buildPlaceBody("US20220809012", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["price"] != float64(0) {
		t.Fatalf("expected price 0, got %v", payload["price"])
	}
	if payload["quantity"] != float64(0) {
		t.Fatalf("expected quantity 0, got %v", payload["quantity"])
	}
	if payload["orderAmount"] != float64(18000) {
		t.Fatalf("expected orderAmount 18000, got %v", payload["orderAmount"])
	}
	if payload["orderPriceType"] != "01" {
		t.Fatalf("expected orderPriceType 01, got %v", payload["orderPriceType"])
	}
	if payload["isFractionalOrder"] != true {
		t.Fatal("expected isFractionalOrder true")
	}
}

// Issue #28: fractional + USD currency-mode previously sent the raw USD value
// in orderAmount with currencyMode="KRW", which the server interpreted as
// a tiny KRW amount and rejected ("금액주문은 $1 또는 1,000원 이상").
// USD inputs must be converted to KRW on the wire so the server accepts the
// floor amount.
func TestBuildPlaceBodyFractionalUSDConvertsToKRW(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol: "TSLL", Market: "us", Side: "buy", OrderType: "market",
		Amount: 100, CurrencyMode: "USD", Fractional: true,
	}
	meta := stockPriceMetadata{Close: 12.12, CloseKRW: 17854, ExchangeRate: 1477.6}
	body, err := buildPlaceBody("US20220809012", "NSQ", intent, meta, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// $100 * 1477.6 = 147760 KRW.
	if payload["orderAmount"] != float64(147760) {
		t.Fatalf("expected orderAmount 147760 (KRW from USD*rate), got %v", payload["orderAmount"])
	}
	if payload["currencyMode"] != "KRW" {
		t.Fatalf("expected wire currencyMode KRW, got %v", payload["currencyMode"])
	}
	if payload["isFractionalOrder"] != true {
		t.Fatal("expected isFractionalOrder true")
	}
	if payload["orderPriceType"] != "01" {
		t.Fatalf("expected market order type 01, got %v", payload["orderPriceType"])
	}
}

func TestPlacePendingOrderSendsXOrderKeyOnCreate(t *testing.T) {
	t.Parallel()

	var paths []string
	var orderKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			paths = append(paths, r.URL.Path)
			orderKeys = append(orderKeys, r.Header.Get("X-Order-Key"))
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::place","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v2/wts/trading/order/create":
			paths = append(paths, r.URL.Path)
			orderKeys = append(orderKeys, r.Header.Get("X-Order-Key"))
			_, _ = w.Write([]byte(`{"result":{"message":"주문 접수 되었어요."}}`))
		case "/api/v1/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":16,"tradeType":"buy","orderPrice":500,"orderUsdPrice":0.3395,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	result, err := client.PlacePendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("PlacePendingOrder returned error: %v", err)
	}
	if result.Status != "accepted_pending" {
		t.Fatalf("expected accepted_pending result, got %q", result.Status)
	}
	if result.OrderID != "2026-03-11/16" {
		t.Fatalf("expected reconciled order id 2026-03-11/16, got %q", result.OrderID)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 mutation requests, got %d", len(paths))
	}
	if orderKeys[0] != "" {
		t.Fatalf("prepare request should not include x-order-key: %#v", orderKeys)
	}
	if orderKeys[1] != "trade::session::test::place" {
		t.Fatalf("unexpected create x-order-key header: %#v", orderKeys)
	}
}

func TestPlacePendingOrderReturnsFilledCompletedFromHistory(t *testing.T) {
	t.Parallel()

	today := time.Now().Format("2006-01-02")
	now := time.Now().Format("2006-01-02 15:04:05.000")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::place","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v2/wts/trading/order/create":
			_, _ = w.Write([]byte(`{"result":{"message":"주문 접수 되었어요."}}`))
		case "/api/v1/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/v2/trading/my-orders/markets/us/by-date/completed":
			_, _ = io.WriteString(w, `{"result":{"body":[{"orderedAt":"`+now+`","lastExecutedAt":"`+now+`","orderNo":1,"orderId":"completed-order-id","stockCode":"US20220809012","stockName":"TSLL","symbol":"TSLL","tradeType":"buy","status":"체결완료","orderQuantity":1,"executedQuantity":1,"userOrderDate":"`+today+`","orderPrice":{"krw":21208},"averageExecutionPrice":{"krw":21208}}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        21208,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	result, err := client.PlacePendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("PlacePendingOrder returned error: %v", err)
	}
	if result.Status != "filled_completed" {
		t.Fatalf("expected filled_completed, got %q", result.Status)
	}
	if result.OrderID != today+"/1" {
		t.Fatalf("expected completed order id, got %q", result.OrderID)
	}
	if result.FilledQuantity != 1 {
		t.Fatalf("expected filled quantity 1, got %v", result.FilledQuantity)
	}
}

func TestPlacePendingOrderReturnsFundingRequiredFromPrepare(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"title":"계좌 잔액이 부족해요","body":"구매를 위해 21,511원을 채울게요.","actions":["닫기","모바일에서 채우기"]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	_, err = client.PlacePendingOrder(context.Background(), intent)
	var branchErr *tradingflow.BranchRequiredError
	if !errors.As(err, &branchErr) {
		t.Fatalf("expected BranchRequiredError, got %v", err)
	}
	if branchErr.Branch != tradingflow.BranchFundingRequired {
		t.Fatalf("expected funding branch, got %q", branchErr.Branch)
	}
	if branchErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", branchErr.StatusCode)
	}
	if branchErr.BrokerMessage == "" {
		t.Fatal("expected broker message to be preserved")
	}
}

func TestPlacePendingOrderReturnsFundingRequiredFromFXFundingMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"환전에 필요한 원화 출금가능금액이 부족합니다."}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	_, err = client.PlacePendingOrder(context.Background(), intent)
	var branchErr *tradingflow.BranchRequiredError
	if !errors.As(err, &branchErr) {
		t.Fatalf("expected BranchRequiredError, got %v", err)
	}
	if branchErr.Branch != tradingflow.BranchFundingRequired {
		t.Fatalf("expected funding branch, got %q", branchErr.Branch)
	}
}

func TestPlacePendingOrderReturnsFXConsentRequiredFromPrepare(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"환전 후 주문하려면 외화 사용 동의가 필요해요."}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	_, err = client.PlacePendingOrder(context.Background(), intent)
	var branchErr *tradingflow.BranchRequiredError
	if !errors.As(err, &branchErr) {
		t.Fatalf("expected BranchRequiredError, got %v", err)
	}
	if branchErr.Branch != tradingflow.BranchFXConsentRequired {
		t.Fatalf("expected fx-consent branch, got %q", branchErr.Branch)
	}
	if branchErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", branchErr.StatusCode)
	}
}

func TestPlacePendingOrderReturnsFXConsentRequiredAfterPrepareNeedExchange(t *testing.T) {
	t.Parallel()

	createCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			_, _ = w.Write([]byte(`{"result":{"preparedOrderInfo":{"needExchange":0.68},"orderKey":"trade::session::test::place","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v1/trading/settings/toggle/find":
			if got := r.URL.Query().Get("categoryName"); got != "GETTING_BACK_KRW" {
				t.Fatalf("unexpected categoryName: %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"categoryName":"GETTING_BACK_KRW","turnedOn":false}}`))
		case "/api/v1/exchange/current-quote/for-buy":
			_, _ = w.Write([]byte(`{"result":{"rateQuoteId":"quote-123","buyCurrency":"USD","sellCurrency":"KRW","validFrom":"2026-03-13T06:47:41Z","validTill":"2026-03-13T06:52:40Z","usdRate":1500.21375,"displayUsdRate":1500.21}}`))
		case "/api/v2/wts/trading/order/create":
			createCalled = true
			_, _ = w.Write([]byte(`{"result":{"message":"주문 접수 되었어요."}}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        1000,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	_, err = client.PlacePendingOrder(context.Background(), intent)
	var branchErr *tradingflow.BranchRequiredError
	if !errors.As(err, &branchErr) {
		t.Fatalf("expected BranchRequiredError, got %v", err)
	}
	if branchErr.Branch != tradingflow.BranchFXConsentRequired {
		t.Fatalf("expected fx-consent branch, got %q", branchErr.Branch)
	}
	if branchErr.Source != tradingflow.BranchSourcePostPrepareConfirmation {
		t.Fatalf("expected post-prepare source, got %q", branchErr.Source)
	}
	if branchErr.FX == nil {
		t.Fatal("expected FX context")
	}
	if branchErr.FX.NeedExchangeUSD != 0.68 {
		t.Fatalf("expected needExchange 0.68, got %v", branchErr.FX.NeedExchangeUSD)
	}
	if branchErr.FX.EstimatedExchangeKRW != 1020 {
		t.Fatalf("expected estimated exchange KRW 1020, got %v", branchErr.FX.EstimatedExchangeKRW)
	}
	if branchErr.FX.USDExchangeRate != 1500.21375 {
		t.Fatalf("expected usd exchange rate 1500.21375, got %v", branchErr.FX.USDExchangeRate)
	}
	if branchErr.FX.RateQuoteID != "quote-123" {
		t.Fatalf("expected quote id quote-123, got %q", branchErr.FX.RateQuoteID)
	}
	if !branchErr.FX.GettingBackKRWKnown {
		t.Fatal("expected GETTING_BACK_KRW state to be known")
	}
	if branchErr.FX.GettingBackKRW {
		t.Fatal("expected GETTING_BACK_KRW to be false")
	}
	if createCalled {
		t.Fatal("order/create should not be called when post-prepare FX confirmation is required")
	}
}

func TestPlacePendingOrderAutoAcceptsFXConsentWhenConfigured(t *testing.T) {
	t.Parallel()

	var paths []string
	var orderKeys []string
	var toggleBody map[string]any
	pendingCalls := 0
	today := time.Now().Format("2006-01-02")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1479.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			paths = append(paths, r.URL.Path)
			orderKeys = append(orderKeys, r.Header.Get("X-Order-Key"))
			_, _ = w.Write([]byte(`{"result":{"preparedOrderInfo":{"needExchange":0.68},"orderKey":"trade::session::test::place","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v1/trading/settings/toggle/find":
			paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"result":{"categoryName":"GETTING_BACK_KRW","turnedOn":false}}`))
		case "/api/v1/exchange/current-quote/for-buy":
			paths = append(paths, r.URL.Path)
			_, _ = w.Write([]byte(`{"result":{"rateQuoteId":"quote-123","buyCurrency":"USD","sellCurrency":"KRW","validFrom":"2026-03-13T06:47:41Z","validTill":"2026-03-13T06:52:40Z","usdRate":1505.29,"displayUsdRate":1505.29}}`))
		case "/api/v2/wts/trading/order/create":
			paths = append(paths, r.URL.Path)
			orderKeys = append(orderKeys, r.Header.Get("X-Order-Key"))
			_, _ = w.Write([]byte(`{"result":{"message":"주문 접수 되었어요."}}`))
		case "/api/v1/trading/settings/toggle":
			paths = append(paths, r.URL.Path)
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &toggleBody); err != nil {
				t.Fatalf("toggle body was not valid json: %v", err)
			}
			_, _ = w.Write([]byte(`{"result":{"categoryName":"EXCHANGE_INFO_CHECK","turnedOn":true}}`))
		case "/api/v1/trading/orders/histories/all/pending":
			pendingCalls++
			if pendingCalls == 1 {
				_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"` + today + `","orderNo":16,"tradeType":"buy","orderPrice":1000,"orderUsdPrice":0.6758,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
		TradingPolicy: config.Trading{
			DangerousAutomation: config.DangerousAutomation{
				AcceptFXConsent: true,
			},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        1000,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	result, err := client.PlacePendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("PlacePendingOrder returned error: %v", err)
	}
	if result.Status != "accepted_pending" {
		t.Fatalf("expected accepted_pending result, got %q", result.Status)
	}
	if result.OrderID != today+"/16" {
		t.Fatalf("expected reconciled order id %s/16, got %q", today, result.OrderID)
	}
	if len(orderKeys) < 2 {
		t.Fatalf("expected prepare and create order-key entries, got %#v", orderKeys)
	}
	if orderKeys[0] != "" {
		t.Fatalf("prepare request should not include x-order-key: %#v", orderKeys)
	}
	if orderKeys[1] != "trade::session::test::place" {
		t.Fatalf("unexpected create x-order-key header: %#v", orderKeys)
	}
	if toggleBody["categoryName"] != "EXCHANGE_INFO_CHECK" {
		t.Fatalf("unexpected toggle category: %#v", toggleBody)
	}
	if toggleBody["turnedOn"] != true {
		t.Fatalf("expected EXCHANGE_INFO_CHECK toggle to be turned on: %#v", toggleBody)
	}

	createIndex := -1
	toggleIndex := -1
	for i, path := range paths {
		if path == "/api/v2/wts/trading/order/create" && createIndex == -1 {
			createIndex = i
		}
		if path == "/api/v1/trading/settings/toggle" && toggleIndex == -1 {
			toggleIndex = i
		}
	}
	if createIndex == -1 || toggleIndex == -1 {
		t.Fatalf("expected both create and toggle calls, got %v", paths)
	}
	if createIndex > toggleIndex {
		t.Fatalf("expected create to happen before toggle, got %v", paths)
	}
}

func TestPlacePendingOrderReturnsGenericPrepareRejected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":14.4,"closeKrw":21208,"volume":13409779}]}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/prepare":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"broker rejected the order"}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	_, err = client.PlacePendingOrder(context.Background(), intent)
	var rejectedErr *tradingflow.PrepareRejectedError
	if !errors.As(err, &rejectedErr) {
		t.Fatalf("expected PrepareRejectedError, got %v", err)
	}
	if rejectedErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", rejectedErr.StatusCode)
	}
	if rejectedErr.BrokerMessage != "broker rejected the order" {
		t.Fatalf("unexpected broker message: %q", rejectedErr.BrokerMessage)
	}
}

func TestAmendPendingOrderReturnsInteractiveAuthRequiredFromPrepare(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":13,"tradeType":"buy","orderPrice":600,"orderUsdPrice":0.4074,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/correct/prepare/2026-03-11/13":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::correct","authRequired":{"required":true,"simpleTrade":false,"verifier":{"type":"interactive"}}}}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	price := 700.0
	intent, err := orderintent.NormalizeAmend("13", nil, &price)
	if err != nil {
		t.Fatalf("NormalizeAmend returned error: %v", err)
	}

	_, err = client.AmendPendingOrder(context.Background(), intent)
	if err != tradingflow.ErrInteractiveAuthRequired {
		t.Fatalf("expected ErrInteractiveAuthRequired, got %v", err)
	}
}

func TestAmendPendingOrderReturnsInteractiveAuthRequiredFromCorrect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":13,"tradeType":"buy","orderPrice":600,"orderUsdPrice":0.4074,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/correct/prepare/2026-03-11/13":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::correct","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v2/wts/trading/order/correct/2026-03-11/13":
			_, _ = w.Write([]byte(`{"result":{"uuid":"challenge","modulus":"abc","exponent":"10001","keyboard":"<svg/>"}}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	price := 700.0
	intent, err := orderintent.NormalizeAmend("13", nil, &price)
	if err != nil {
		t.Fatalf("NormalizeAmend returned error: %v", err)
	}

	_, err = client.AmendPendingOrder(context.Background(), intent)
	if err != tradingflow.ErrInteractiveAuthRequired {
		t.Fatalf("expected ErrInteractiveAuthRequired, got %v", err)
	}
}

func TestAmendPendingOrderReturnsCompletedOrderFromHistory(t *testing.T) {
	t.Parallel()

	today := time.Now().Format("2006-01-02")
	now := time.Now().Format("2006-01-02 15:04:05.000")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/trading/orders/histories/all/pending":
			_, _ = w.Write([]byte(`{"result":[{"stockCode":"US20220809012","orderedDate":"2026-03-11","orderNo":13,"tradeType":"buy","orderPrice":600,"orderUsdPrice":0.4074,"quantity":1,"pendingQuantity":1,"orderPriceTypeCode":"00","isFractionalOrder":false,"isAfterMarketOrder":false,"status":"체결대기"}]}`))
		case "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case "/api/v1/exchange/usd/base-exchange-rate":
			_, _ = w.Write([]byte(`{"result":{"rate":1472.8}}`))
		case "/api/v2/wts/trading/order/correct/prepare/2026-03-11/13":
			_, _ = w.Write([]byte(`{"result":{"delayCancelExchange":false,"orderKey":"trade::session::test::correct","authRequired":{"required":false,"simpleTrade":true,"verifier":null}}}`))
		case "/api/v2/wts/trading/order/correct/2026-03-11/13":
			_, _ = w.Write([]byte(`{"result":{"message":"주문 수정 되었어요."}}`))
		case "/api/v2/trading/my-orders/markets/us/by-date/completed":
			_, _ = io.WriteString(w, `{"result":{"body":[{"orderedAt":"`+now+`","lastExecutedAt":"`+now+`","orderNo":14,"orderId":"completed-amend-order-id","stockCode":"US20220809012","stockName":"TSLL","symbol":"TSLL","tradeType":"buy","status":"체결완료","orderQuantity":1,"executedQuantity":1,"userOrderDate":"`+today+`","orderPrice":{"krw":700},"averageExecutionPrice":{"krw":700}}]}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260311.2121"},
			Storage: map[string]string{"localStorage:qr-tabId": "browser-tab-test123"},
		},
	})

	price := 700.0
	intent, err := orderintent.NormalizeAmend("13", nil, &price)
	if err != nil {
		t.Fatalf("NormalizeAmend returned error: %v", err)
	}

	result, err := client.AmendPendingOrder(context.Background(), intent)
	if err != nil {
		t.Fatalf("AmendPendingOrder returned error: %v", err)
	}
	if result.Status != "amended_completed" {
		t.Fatalf("expected amended_completed, got %q", result.Status)
	}
	if result.OriginalOrderID != "13" {
		t.Fatalf("expected original order id 13, got %q", result.OriginalOrderID)
	}
	if result.CurrentOrderID != today+"/14" {
		t.Fatalf("expected current order id %s/14, got %q", today, result.CurrentOrderID)
	}
}

func TestClassifyPlaceCreateFailureSurfacesBrokerMessage(t *testing.T) {
	err := classifyPlaceCreateFailure(&StatusError{
		StatusCode: 400,
		Endpoint:   "https://wts-cert-api.tossinvest.com/api/v2/wts/trading/order/create",
		Body:       `{"message":"장 운영시간이 아닙니다"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "장 운영시간이 아닙니다") {
		t.Fatalf("expected broker message surfaced, got %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected status code in message, got %v", err)
	}
}

func TestClassifyPlaceCreateFailurePassesThroughNonStatusError(t *testing.T) {
	orig := fmt.Errorf("network down")
	if got := classifyPlaceCreateFailure(orig); got != orig {
		t.Fatalf("expected passthrough, got %v", got)
	}
}
