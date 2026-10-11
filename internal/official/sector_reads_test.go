package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 더미 값 — 실제 섹터·종목·금액 아님.
func sectorServer(t *testing.T, calls *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		*calls = append(*calls, r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case r.URL.Path == "/api/v1/sectors":
			_, _ = w.Write([]byte(`{"result":{"sectors":[
			  {"sectorId":"100","name":"더미대","depth":0,"parentSectorId":null},
			  {"sectorId":"110","name":"더미중","depth":1,"parentSectorId":"100"}]}}`))
		case r.URL.Path == "/api/v1/sectors/rankings":
			_, _ = w.Write([]byte(`{"result":{"rankings":[
			  {"rank":1,"sectorId":"100","name":"더미대","changeRate":"0.0123","tradingAmount":null,"marketCapitalization":"1000","currency":"KRW"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/stocks"):
			_, _ = w.Write([]byte(`{"result":{"stocks":[
			  {"symbol":"000001","name":"더미주","marketCapitalization":"500","tradingAmount":null,"volume":"0","currency":"KRW"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/etfs"):
			_, _ = w.Write([]byte(`{"result":{"etfs":[{"symbol":"000002","name":"더미ETF"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"result":{"sectorId":"110","name":"더미중","depth":1,"parentSectorId":"100","summary":null,
			  "performance":{"changeRates":[{"duration":"1d","changeRate":"-0.01"},{"duration":"1y","changeRate":null}]}}}`))
		}
	}))
}

func TestSectorReads(t *testing.T) {
	var calls []string
	srv := sectorServer(t, &calls)
	defer srv.Close()
	c := universeClient(t, srv)
	ctx := context.Background()

	cat, err := c.Sectors(ctx)
	if err != nil || len(cat.Sectors) != 2 || cat.Sectors[0].ParentID != "" || cat.Sectors[1].ParentID != "100" {
		t.Fatalf("Sectors: %v %+v", err, cat)
	}

	// 소문자 입력은 서버 enum 에 맞게 정규화된다. market 생략은 KR.
	rk, err := c.SectorRankings(ctx, "top_gainers", "", "1MO")
	if err != nil || rk.Market != "KR" || rk.Duration != "1mo" || len(rk.Items) != 1 {
		t.Fatalf("SectorRankings: %v %+v", err, rk)
	}
	// null 은 nil 로, 값은 소수 비율 그대로 — 100배 변환하지 않는다.
	if rk.Items[0].TradingAmount != nil || rk.Items[0].ChangeRate == nil || *rk.Items[0].ChangeRate != 0.0123 {
		t.Errorf("nullable crossed: %+v", rk.Items[0])
	}
	if _, err := c.SectorRankings(ctx, "BOGUS", "KR", "1d"); err == nil {
		t.Error("bogus ranking type accepted")
	}

	p, err := c.Sector(ctx, "110", "us")
	if err != nil || p.Market != "US" || p.ParentID != "100" || len(p.Returns) != 2 || p.Returns[1].ChangeRate != nil {
		t.Fatalf("Sector: %v %+v", err, p)
	}

	st, err := c.SectorStocks(ctx, "110", "KR")
	if err != nil || len(st.Stocks) != 1 || st.Stocks[0].Volume == nil || *st.Stocks[0].Volume != 0 || st.Stocks[0].TradingAmount != nil {
		t.Fatalf("SectorStocks: %v %+v", err, st)
	}

	et, err := c.SectorETFs(ctx, "110", "KR")
	if err != nil || len(et.ETFs) != 1 || et.ETFs[0].Symbol != "000002" {
		t.Fatalf("SectorETFs: %v %+v", err, et)
	}

	want := []string{
		"/api/v1/sectors?",
		"/api/v1/sectors/rankings?duration=1mo&marketCountry=KR&type=TOP_GAINERS",
		"/api/v1/sectors/110?marketCountry=US",
		"/api/v1/sectors/110/stocks?marketCountry=KR",
		"/api/v1/sectors/110/etfs?marketCountry=KR",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
}
