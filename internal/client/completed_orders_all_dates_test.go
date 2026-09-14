package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestCompletedOrdersAllDatesUsesCursorAndRowMarkets(t *testing.T) {
	t.Parallel()
	c := orderHistoryContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/account/list" {
			_, _ = w.Write([]byte(`{"result":{"primaryKey":"test-account","accountList":[]}}`))
			return
		}
		if r.URL.Path != "/api/v3/trading/my-orders/completed" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("accountKey") != "test-account" {
			t.Error("missing account scope")
		}
		q := r.URL.Query()
		if q.Get("executedOnly") != "false" || q.Get("size") != "2" || q.Has("market") || q.Has("range.from") {
			t.Errorf("unexpected query: %v", q)
		}
		switch q.Get("number") {
		case "1":
			if q.Has("key") {
				t.Error("first page must omit cursor")
			}
			_, _ = w.Write([]byte(`{"result":{"body":[],"lastPage":false,"pagingParam":{"number":1,"size":2,"key":"synthetic-cursor"}}}`))
		case "2":
			if q.Get("key") != "synthetic-cursor" {
				t.Error("next page must use the server cursor")
			}
			_, _ = w.Write([]byte(`{"result":{"body":[{"orderId":"dummy-us","stockCode":"AAPL","market":"us","status":"취소"},{"orderId":"dummy-kr","stockCode":"A005930","market":"kr","status":"체결완료"}],"lastPage":true,"pagingParam":{"number":2,"size":2,"key":null}}}`))
		default:
			t.Errorf("unexpected page: %s", q.Get("number"))
		}
	})
	orders, err := c.ListCompletedOrdersAllDates(context.Background(), "all", 2, 2)
	if err != nil || len(orders) != 2 {
		t.Fatalf("expected requested page, got %d orders, %v", len(orders), err)
	}
	if orders[0].Market != "us" || orders[1].Market != "kr" || orders[0].Status != "취소" {
		t.Fatalf("mixed markets/cancelled status were not preserved: %+v", orders)
	}
}

func TestCompletedOrdersAllDatesRejectsInvalidPageContracts(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{}`, `{"result":{"body":[],"lastPage":true}}`,
		`{"result":{"body":[],"pagingParam":{"number":1,"size":1}}}`,
		`{"result":{"body":[],"lastPage":false,"pagingParam":{"number":1,"size":1}}}`,
		`{"result":{"body":[{"orderId":"dummy"}],"lastPage":true,"pagingParam":{"number":1,"size":1}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			c := orderHistoryContractClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/account/list" {
					_, _ = w.Write([]byte(`{"result":{"primaryKey":"test-account","accountList":[]}}`))
					return
				}
				_, _ = fmt.Fprint(w, body)
			})
			if _, err := c.ListCompletedOrdersAllDates(context.Background(), "us", 1, 1); err == nil {
				t.Fatal("accepted an incomplete page contract")
			}
		})
	}
}

func TestCompletedOrdersAllDatesStopsAtEndOrRepeatedCursor(t *testing.T) {
	t.Parallel()
	for _, lastPage := range []bool{true, false} {
		t.Run(fmt.Sprint(lastPage), func(t *testing.T) {
			c := orderHistoryContractClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/account/list" {
					_, _ = w.Write([]byte(`{"result":{"primaryKey":"test-account","accountList":[]}}`))
					return
				}
				number := r.URL.Query().Get("number")
				if lastPage && number != "1" {
					t.Error("read past the final page")
				}
				_, _ = fmt.Fprintf(w, `{"result":{"body":[],"lastPage":%t,"pagingParam":{"number":%s,"size":1,"key":"same-cursor"}}}`, lastPage, number)
			})
			orders, err := c.ListCompletedOrdersAllDates(context.Background(), "kr", 1, 3)
			if lastPage {
				if err != nil || orders == nil || len(orders) != 0 {
					t.Fatalf("page past end: orders=%v error=%v", orders, err)
				}
			} else if err == nil {
				t.Fatal("accepted a repeated cursor")
			}
		})
	}
}
