package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// All literal values below are synthetic dummy data — fabricated shapes that
// mirror Toss responses without corresponding to any real account or person.

func TestGetDividends(t *testing.T) {
	var sawAccountKey, sawYear string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/v1/account/list"):
			w.Write([]byte(`{"result":{"accountList":[{"accountNo":"00000000000","key":"7"}],"primaryKey":"7"}}`))
		case strings.Contains(r.URL.Path, "/api/v1/dividends/accounts/annual/history"):
			sawAccountKey = r.Header.Get("accountKey")
			sawYear = r.URL.Query().Get("year")
			w.Write([]byte(`{"result":{
				"summary":{"totalAmount":{"krw":1000,"usd":0.7},"paidAmount":{"krw":600,"usd":0.4},"estimatedAmount":{"krw":400,"usd":0.3}},
				"regionSummary":{"kr":{"totalAmount":{"krw":100,"usd":null},"paidAmount":{"krw":100,"usd":null},"estimatedAmount":{"krw":0,"usd":null}},"us":{"totalAmount":{"krw":900,"usd":0.7},"paidAmount":{"krw":500,"usd":0.4},"estimatedAmount":{"krw":400,"usd":0.3}}},
				"calendar":{"year":2026,"monthlySchedule":[{"month":1,"summary":{"totalAmount":{"krw":250,"usd":0.2},"paidAmount":{"krw":250,"usd":0.2},"estimatedAmount":{"krw":0,"usd":null}},"details":[{"productCode":"USDUMMY01","productName":"DUMMY","quantity":10,"amount":{"krw":250,"usd":0.2}}]}]}
			}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	d, err := testClientFor(srv).GetDividends(context.Background(), 2026, false)
	if err != nil {
		t.Fatalf("GetDividends error: %v", err)
	}
	if sawAccountKey != "7" {
		t.Errorf("accountKey header = %q, want 7", sawAccountKey)
	}
	if sawYear != "2026" {
		t.Errorf("year param = %q, want 2026", sawYear)
	}
	if d.Year != 2026 || d.Summary.Total.KRW != 1000 {
		t.Errorf("unexpected summary: %+v", d.Summary)
	}
	if len(d.Regions) != 2 || d.Regions[0].Region != "kr" || d.Regions[1].Region != "us" {
		t.Errorf("unexpected regions: %+v", d.Regions)
	}
	if len(d.Monthly) != 1 || d.Monthly[0].Month != 1 || len(d.Monthly[0].Stocks) != 1 {
		t.Errorf("unexpected monthly: %+v", d.Monthly)
	}
	if d.Monthly[0].Stocks[0].Name != "DUMMY" {
		t.Errorf("unexpected stock: %+v", d.Monthly[0].Stocks[0])
	}
}

func TestGetDividendsByPaymentDateCarriesTax(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/v1/account/list"):
			w.Write([]byte(`{"result":{"accountList":[{"key":"1"}],"primaryKey":"1"}}`))
		case strings.Contains(r.URL.Path, "/by-payment-date"):
			w.Write([]byte(`{"result":{"summary":{"totalAmount":{"krw":1000,"usd":0.7},"paidAmount":{"krw":1000,"usd":0.7},"estimatedAmount":{"krw":0,"usd":null},"totalTax":{"krw":150,"usd":0.1},"totalCommission":{"krw":0,"usd":0}},"regionSummary":{},"calendar":{"year":2026,"monthlySchedule":[]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	d, err := testClientFor(srv).GetDividends(context.Background(), 2026, true)
	if err != nil {
		t.Fatalf("GetDividends error: %v", err)
	}
	if !d.ByPaymentDate {
		t.Error("ByPaymentDate not set")
	}
	if d.Summary.Tax == nil || d.Summary.Tax.KRW != 150 {
		t.Errorf("expected tax 150, got %+v", d.Summary.Tax)
	}
}

func TestGetCommunityRankings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/community/top-rankings/TOP_10_PROFIT_ROSS_AMOUNT") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"result":{"items":[{"profitLossAmountKrw":5000,"profitLossRateKrw":0.25,"target":{"nickname":"dummy","userProfileId":42},"type":"TOP_10_PROFIT_ROSS_AMOUNT"}]}}`))
	}))
	defer srv.Close()

	r, err := testClientFor(srv).GetCommunityRankings(context.Background(), "profit")
	if err != nil {
		t.Fatalf("GetCommunityRankings error: %v", err)
	}
	if r.Type != "TOP_10_PROFIT_ROSS_AMOUNT" || len(r.Users) != 1 {
		t.Fatalf("unexpected ranking: %+v", r)
	}
	u := r.Users[0]
	if u.Rank != 1 || u.Nickname != "dummy" || u.ProfitAmountKRW != 5000 || u.ProfitRate != 0.25 {
		t.Errorf("unexpected user: %+v", u)
	}
}

func TestCommunityRankingTypeAliases(t *testing.T) {
	cases := map[string]string{
		"":           "INFLUENCER",
		"influencer": "INFLUENCER",
		"profit":     "TOP_10_PROFIT_ROSS_AMOUNT",
		"followers":  "TOP_10_FOLLOWING_INCREASE",
		"INFLUENCER": "INFLUENCER",
	}
	for in, want := range cases {
		got, err := CommunityRankingType(in)
		if err != nil || got != want {
			t.Errorf("CommunityRankingType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := CommunityRankingType("nope"); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestGetEarningCallHome(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/earning-call/home") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"result":{"majorCompanies":{"currentOrFuture":[{"eventId":1,"eventTitle":"Q","status":"UPCOMING","liveAt":"2026-06-23T23:00:00+09:00","companyCode":"DUMMY","companyName":"더미","subContentText":"sub"}]}}}`))
	}))
	defer srv.Close()

	ec, err := testClientFor(srv).GetEarningCallHome(context.Background())
	if err != nil {
		t.Fatalf("GetEarningCallHome error: %v", err)
	}
	if len(ec.Events) != 1 || ec.Events[0].CompanyName != "더미" || ec.Events[0].Category != "sub" {
		t.Errorf("unexpected events: %+v", ec.Events)
	}
}

func TestGetEarningCallDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/earning-call/events/42/info" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"result":{"eventId":42,"marketCountry":"US","category":"EARNINGS_CALL","defaultSummarizationCategory":"SUMMARY","status":"ENDED","title":"Q2 call","liveAt":"2026-08-01T06:00:00+09:00","wentLiveAt":"2026-08-01T06:01:00+09:00","audioUrl":"https://example.invalid/audio","transcriptUrl":"https://example.invalid/transcript","slideFileUrl":"https://example.invalid/slides","companyCode":"DUMMY","companyName":"Dummy Inc.","companyLogoImageUrl":"https://example.invalid/logo","representativeStockSymbol":"DUM","representativeStockGuid":"guid-42","representativeStockCode":"USDUMMY","reportId":"report-42","reportItem":"Q2","mtsLandingPath":"/stocks/USDUMMY","consensusGapRate":1.25,"isGapRateVisible":true,"stockChangeRate":2.5}}`))
	}))
	defer srv.Close()

	detail, err := testClientFor(srv).GetEarningCallDetail(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetEarningCallDetail error: %v", err)
	}
	if detail.EventID != 42 || detail.CompanyName != "Dummy Inc." || detail.AudioURL == nil || *detail.AudioURL == "" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.ConsensusGapRate == nil || *detail.ConsensusGapRate != 1.25 || detail.StockChangeRate == nil || *detail.StockChangeRate != 2.5 {
		t.Fatalf("optional rates were not preserved: %+v", detail)
	}
}

func TestGetEarningCallDetailRejectsInvalidID(t *testing.T) {
	if _, err := New(Config{}).GetEarningCallDetail(context.Background(), 0); err == nil {
		t.Fatal("expected invalid event id error")
	}
}

func TestGetIndexDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/indicator/index"):
			w.Write([]byte(`{"result":{"majorIndicatorInfos":[{"code":"COMP.NAI","displayName":"나스닥","nation":"us","price":{"latestPrice":26517.93,"basePrice":26021.65}}]}}`))
		case strings.Contains(r.URL.Path, "/api/v1/index-prices/COMP.NAI"):
			w.Write([]byte(`{"result":{"open":26410.62,"high":26559.74,"low":26188.68,"close":26517.93,"volume":17780101967,"base":26021.65,"high52w":27190.20,"low52w":19334.98}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// resolve by alias
	q, err := testClientFor(srv).GetIndexDetail(context.Background(), "nasdaq")
	if err != nil {
		t.Fatalf("GetIndexDetail error: %v", err)
	}
	if q.Code != "COMP.NAI" || q.Name != "나스닥" || q.Close != 26517.93 || q.High52w != 27190.20 {
		t.Errorf("unexpected quote: %+v", q)
	}
	if q.Change == 0 || q.ChangeRate == 0 {
		t.Errorf("expected computed change, got %+v", q)
	}

	// unknown index -> error
	if _, err := testClientFor(srv).GetIndexDetail(context.Background(), "no-such-index"); err == nil {
		t.Error("expected error for unknown index")
	}
}

func TestGetSectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/tics/all") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"result":{"ticsItems":[{"id":1,"title":"운송","companyCount":420,"fluctuations":{"oneDayRate":-0.4,"oneMonthRate":2.2,"threeMonthsRate":10.6,"oneYearRate":52},"subItems":[{"id":3,"title":"항공사","companyCount":39,"fluctuations":{"oneDayRate":1,"oneMonthRate":14.8,"threeMonthsRate":17,"oneYearRate":31},"subItems":null}]}]}}`))
	}))
	defer srv.Close()

	s, err := testClientFor(srv).GetSectors(context.Background())
	if err != nil {
		t.Fatalf("GetSectors error: %v", err)
	}
	if len(s.Items) != 1 || s.Items[0].Title != "운송" || s.Items[0].OneYearRate != 52 {
		t.Fatalf("unexpected sectors: %+v", s.Items)
	}
	if len(s.Items[0].SubSectors) != 1 || s.Items[0].SubSectors[0].Title != "항공사" || s.Items[0].SubSectors[0].OneMonthRate != 14.8 {
		t.Errorf("unexpected sub-sectors: %+v", s.Items[0].SubSectors)
	}
}

func TestGetNewsBriefing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/reasoning/personalized" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"result":{"createdAt":"2026-06-19T00:00:00Z","items":[{"category":{"keywords":["a","b"],"type":"수급"},"news":[{"title":"헤드라인","agencyName":"통신사","source":"src","createdAt":"2026-06-19T00:00:00Z"}]}]}}`))
	}))
	defer srv.Close()

	b, err := testClientFor(srv).GetNewsBriefing(context.Background())
	if err != nil {
		t.Fatalf("GetNewsBriefing error: %v", err)
	}
	if len(b.Items) != 1 || b.Items[0].CategoryType != "수급" || len(b.Items[0].Keywords) != 2 {
		t.Fatalf("unexpected items: %+v", b.Items)
	}
	if len(b.Items[0].News) != 1 || b.Items[0].News[0].Title != "헤드라인" || b.Items[0].News[0].Agency != "통신사" {
		t.Errorf("unexpected news: %+v", b.Items[0].News)
	}
}
