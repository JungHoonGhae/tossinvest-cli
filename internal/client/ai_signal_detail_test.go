package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetAISignalDetailMapsVerifiedReasoningContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/dashboard/wts/overview/ai-signals/detail" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("productCode"); got != "A005930" {
			t.Errorf("productCode = %q", got)
		}
		if got := r.URL.Query().Get("productType"); got != "STOCKS" {
			t.Errorf("productType = %q", got)
		}
		_, _ = w.Write([]byte(`{"result":{
			"signalId":"signal-1","traceId":"trace-1","createdAt":"2026-09-03T00:00:00Z",
			"signalDirection":1,"hasRelatedReasoning":true,
			"reasoning":{"description":"핵심 설명","issue":{
				"assetCode":"A005930","assetName":"예시 종목","assetType":"STOCKS",
				"description":{"data":["첫 문장","둘째 문장"]},
				"investmentType":"MARKET","logoImageUrl":"https://example.test/logo.png",
				"originCodes":["origin-1"],"profitLossRate":3.25},
				"keywords":["반도체","수급"],
				"news":{"data":[{"id":"news-1","title":"관련 뉴스","agencyName":"예시 통신사",
				"source":"source","faviconUrl":"https://example.test/favicon.png","createdAt":"2026-09-03T00:01:00Z"}]}},
			"relatedReasoning":{"callout":"연관 흐름","details":[{
				"signalId":"related-1","assetCode":"A000660","assetName":"연관 종목",
				"description":{"data":["연관 설명"]},
				"relationship":{"subjectName":"예시 종목","relation":"공급","objectName":"연관 종목"},
				"relatedStocks":[{"stockCode":"A000660","name":"연관 종목","symbol":"000660",
				"investmentType":"RELATED","investmentTypeValue":"연관","companyCode":"C1",
				"companyName":"연관 회사","logoImageUrl":"https://example.test/related.png","status":"NORMAL"}]}]},
			"terms":{"personalizedServiceAgreed":true,"serviceAgreed":true}
		}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAISignalDetail(context.Background(), "005930", "stock")
	if err != nil {
		t.Fatalf("GetAISignalDetail: %v", err)
	}
	if !got.Found || got.ProductCode != "A005930" || got.ProductType != "STOCKS" || got.SignalID != "signal-1" {
		t.Fatalf("identity = %#v", got)
	}
	if got.Issue.AssetName != "예시 종목" || len(got.Issue.Description) != 2 || len(got.Keywords) != 2 {
		t.Fatalf("issue = %#v, keywords = %#v", got.Issue, got.Keywords)
	}
	if len(got.News) != 1 || got.News[0].ID != "news-1" || got.News[0].Agency != "예시 통신사" {
		t.Fatalf("news = %#v", got.News)
	}
	if got.RelatedCallout != "연관 흐름" || len(got.Related) != 1 || got.Related[0].Relationship.Relation != "공급" {
		t.Fatalf("related = %#v", got.Related)
	}
	if len(got.Related[0].Stocks) != 1 || got.Related[0].Stocks[0].Name != "연관 종목" || got.Related[0].Stocks[0].CompanyCode != "C1" {
		t.Fatalf("related stocks = %#v", got.Related[0].Stocks)
	}
	if !got.Terms.ServiceAgreed || !got.Terms.PersonalizedServiceAgreed {
		t.Fatalf("terms = %#v", got.Terms)
	}
}

func TestGetAISignalDetailPreservesNoCurrentSignal(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":null}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetAISignalDetail(context.Background(), "A005930", "STOCKS")
	if err != nil {
		t.Fatal(err)
	}
	if got.Found || got.ProductCode != "A005930" || got.ProductType != "STOCKS" {
		t.Fatalf("no-signal result = %#v", got)
	}
}

func TestGetAISignalDetailRejectsUnobservedProductTypeBeforeRequest(t *testing.T) {
	t.Parallel()
	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested.Store(true)
	}))
	t.Cleanup(server.Close)

	_, err := testClientFor(server).GetAISignalDetail(context.Background(), "A005930", "bond")
	if err == nil || !strings.Contains(err.Error(), "stocks or equity_etf") {
		t.Fatalf("error = %v", err)
	}
	if requested.Load() {
		t.Fatal("unobserved product type must be rejected before making a request")
	}
}
