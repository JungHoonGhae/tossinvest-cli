package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func insightsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v2/reasoning/stocks/"):
			w.Write([]byte(`{"result":{"sectionTitle":"왜 올랐을까?","sectionSummary":"더미 요약.",
			  "signalId":"STOCKS:000000:20260804","signalDirection":1,"keyword":"더미키워드",
			  "createdAt":"2026-08-04T13:00:00",
			  "relatedStocks":[{"stockCode":"A000000","stockName":"더미전자","symbol":"000000",
			    "market":"KSP","investmentType":"DUMMY","investmentTypeValue":"더미유형"}]}}`))
		case r.URL.Path == "/api/v1/dashboard/wts/overview/signals":
			if got := r.URL.Query().Get("codes"); got != "A000000" {
				t.Fatalf("codes = %q", got)
			}
			w.Write([]byte(`{"result":{"stockCode":"A000000","signals":[
			  {"signalLabel":"호재","signalInfo":"더미 시그널.","signalId":7001002,
			   "datetime":"2026-08-04T13:10:13"}]}}`))
		case r.URL.Path == "/api/v1/margin/cert/notice/receivable":
			if got := r.URL.Query().Get("currency"); got != "KRW" {
				t.Fatalf("currency = %q", got)
			}
			w.Write([]byte(`{"result":{"depositNoticeType":"NONE","receivableAmount":0,
			  "deadlineAt":null,"forcedLiquidatedAt":null,
			  "suspensionStartDate":null,"suspensionEndDate":null}}`))
		case r.URL.Path == "/api/v1/search-all/wts-auto-complete":
			w.Write([]byte(`{"result":[{"keyword":"더미전자","subKeyword":"","stockCode":"A000000",
			  "companyCode":"000000","companyName":"더미전자","market":"KSP","symbol":"000000"}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGetStockReasoning(t *testing.T) {
	server := insightsServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetStockReasoning(t.Context(), "A000000")
	if err != nil {
		t.Fatalf("GetStockReasoning() error = %v", err)
	}
	if got.Summary == "" || got.Title == "" {
		t.Errorf("Title/Summary empty: %+v", got)
	}
	if got.Direction != 1 {
		t.Errorf("Direction = %d, want 1", got.Direction)
	}
	if len(got.RelatedStock) != 1 || got.RelatedStock[0].ProductCode != "A000000" {
		t.Errorf("RelatedStock = %+v", got.RelatedStock)
	}
	// 서버 원문 유지 — 토스가 매핑을 공개하지 않아 번역하면 추측이 된다.
	if got.RelatedStock[0].InvestmentTypeValue != "더미유형" {
		t.Errorf("InvestmentTypeValue = %q", got.RelatedStock[0].InvestmentTypeValue)
	}
}

func TestGetStockSignals(t *testing.T) {
	server := insightsServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetStockSignals(t.Context(), "A000000")
	if err != nil {
		t.Fatalf("GetStockSignals() error = %v", err)
	}
	if len(got.Signals) != 1 {
		t.Fatalf("len(Signals) = %d, want 1", len(got.Signals))
	}
	if got.Signals[0].Label != "호재" {
		t.Errorf("Label = %q", got.Signals[0].Label)
	}
	if got.Signals[0].SignalID != 7001002 {
		t.Errorf("SignalID = %d", got.Signals[0].SignalID)
	}
}

func TestGetMarginNotice(t *testing.T) {
	server := insightsServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetMarginNotice(t.Context(), "krw")
	if err != nil {
		t.Fatalf("GetMarginNotice() error = %v", err)
	}
	if got.Currency != "KRW" {
		t.Errorf("Currency = %q, want KRW (uppercased)", got.Currency)
	}
	if got.NoticeType != "NONE" {
		t.Errorf("NoticeType = %q", got.NoticeType)
	}
	// nil 이어야 한다 — 0값 시각으로 채우면 연체 계좌처럼 읽힌다.
	if got.DeadlineAt != nil || got.ForcedLiquidatedAt != nil {
		t.Errorf("null timestamps materialised: %+v", got)
	}
}

func TestSearch(t *testing.T) {
	server := insightsServer(t)
	defer server.Close()

	got, err := testClientFor(server).Search(t.Context(), "더미")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got.Results) != 1 || got.Results[0].ProductCode != "A000000" {
		t.Fatalf("Results = %+v", got.Results)
	}
	if got.Query != "더미" {
		t.Errorf("Query = %q", got.Query)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	server := insightsServer(t)
	defer server.Close()

	if _, err := testClientFor(server).Search(t.Context(), "   "); err == nil {
		t.Fatal("want error for blank query, got nil")
	}
}
