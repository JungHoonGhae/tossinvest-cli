package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGetSectorDetailCombinesOverviewStocksETFsAndNews(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/api/v2/dashboard/wts/overview/tics/7/overview":
			if r.Method != http.MethodGet {
				t.Errorf("overview method = %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"result":{"ticsId":7,"name":"반도체","summary":"산업군 요약","description":"산업군 설명","depth":1,"companyCount":12,"etfCount":3}}`))
		case "/api/v2/dashboard/wts/overview/tics/7/stocks":
			if r.Method != http.MethodPost {
				t.Errorf("stocks method = %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "{}" {
				t.Errorf("stocks body = %q", body)
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"rank":1,"code":"A000001","name":"예시 종목","logoImageUrl":"https://example.test/stock.png","analystOpinion":"BUY","changeRate":2.5,"marketCapKrw":1000,"marketCapUsd":1,"tradingValueKrw":500,"tradingValueUsd":0.5,"volume":20,"price":{"base":100,"baseKrw":100,"close":102.5,"closeKrw":102.5,"priceType":"KRW"}}],"totalCount":1}}`))
		case "/api/v2/dashboard/wts/overview/tics/7/etfs":
			if r.Method != http.MethodPost {
				t.Errorf("etfs method = %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"result":{"etfs":[{"rank":1,"code":"ETF001","symbol":"EXM","name":"예시 ETF","detailName":"예시 상세","logoImageUrl":"https://example.test/etf.png","changeRate":1.2,"expenseRatio":0.1,"leverageFactor":1,"topHolding":{"name":"예시 종목","weight":12.3},"tradingValueKrw":200,"tradingValueUsd":0.2,"price":{"base":50,"baseKrw":null,"close":51,"closeKrw":null,"priceType":"USD"}}],"totalCount":1}}`))
		case "/api/v2/dashboard/wts/overview/tics/7/news":
			if r.Method != http.MethodGet {
				t.Errorf("news method = %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"result":{"body":[{"id":"n1","title":"산업군 뉴스","summary":"뉴스 요약","source":"src","createdAt":"2026-09-03T00:00:00Z","updatedAt":"2026-09-03T00:30:00Z","imageUrls":["https://example.test/news.png"]}],"totalCount":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetSectorDetail(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetSectorDetail: %v", err)
	}
	if requests.Load() != 4 {
		t.Fatalf("requests = %d, want 4", requests.Load())
	}
	if got.ID != 7 || got.Name != "반도체" || got.CompanyCount != 12 || got.ETFCount != 3 {
		t.Fatalf("overview = %#v", got)
	}
	if got.StockTotalCount != 1 || got.ETFTotalCount != 1 || got.NewsTotalCount != 1 {
		t.Fatalf("total counts = stocks:%d etfs:%d news:%d", got.StockTotalCount, got.ETFTotalCount, got.NewsTotalCount)
	}
	if len(got.Stocks) != 1 || got.Stocks[0].ProductCode != "A000001" || got.Stocks[0].Price.Close == nil || *got.Stocks[0].Price.Close != 102.5 {
		t.Fatalf("stocks = %#v", got.Stocks)
	}
	if len(got.ETFs) != 1 || got.ETFs[0].Symbol != "EXM" || got.ETFs[0].TopHolding == nil || got.ETFs[0].TopHolding.Weight != 12.3 || got.ETFs[0].Price.CloseKRW != nil {
		t.Fatalf("etfs = %#v", got.ETFs)
	}
	if len(got.News) != 1 || got.News[0].ID != "n1" || len(got.News[0].ImageURLs) != 1 {
		t.Fatalf("news = %#v", got.News)
	}
}

func TestGetSectorDetailRejectsInvalidIDBeforeRequest(t *testing.T) {
	t.Parallel()
	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested.Store(true)
	}))
	t.Cleanup(server.Close)

	_, err := testClientFor(server).GetSectorDetail(context.Background(), 0)
	if err == nil {
		t.Fatal("expected invalid sector id error")
	}
	if requested.Load() {
		t.Fatal("invalid id must be rejected before making a request")
	}
}
