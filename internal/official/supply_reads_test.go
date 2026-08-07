package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 더미 값 — 실시세 아님.
func supplyServer(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case path:
			_, _ = w.Write([]byte(body))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func supplyClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

// 당일 잠정 기록은 개인 잠정치·외국인 보유·CFD 가 null 이다. 0 으로 접으면
// "순매수 0" 과 "아직 집계 안 됨" 이 구분되지 않는다 — 수급에서 정반대 신호다.
func TestSupplyInvestorKeepsNullDistinctFromZero(t *testing.T) {
	body := `{"result":{"nextUntil":"2026-01-01","records":[
	  {"date":"2026-01-05","updatedAt":"2026-01-05T18:00:00Z",
	   "individual":null,"foreignerHolding":null,"cfd":null,
	   "foreigner":{"buyVolume":"100","sellVolume":"40","netBuyVolume":"60"},
	   "institution":{"buyVolume":"10","sellVolume":"10","netBuyVolume":"0",
	     "breakdown":{"financialInvestment":{"buyVolume":"1","sellVolume":"0","netBuyVolume":"1"},"insurance":null,"trust":null,"bank":null,
	                  "otherFinancialInstitution":null,"pensionFund":{"buyVolume":"10","sellVolume":"4","netBuyVolume":"6"},"privateEquityFund":null}}}]}}`
	srv := supplyServer(t, "/api/v1/stocks/005930/investor-trading", body)
	defer srv.Close()

	got, err := supplyClient(t, srv).Supply(context.Background(), "005930", domain.SupplyInvestor, 0, "")
	if err != nil {
		t.Fatalf("Supply: %v", err)
	}
	if got.NextUntil != "2026-01-01" {
		t.Errorf("NextUntil = %q", got.NextUntil)
	}
	r := got.Records[0]
	if r.Individual != nil {
		t.Error("null individual materialised")
	}
	if r.ForeignerHolding != nil || r.CFD != nil {
		t.Error("null holding/cfd materialised")
	}
	// 순매수 0 은 값이다 — nil 이 되면 안 된다.
	if r.Institution == nil || r.Institution.NetBuy != 0 {
		t.Errorf("institution net-buy zero lost: %+v", r.Institution)
	}
	if r.InstitutionSplit == nil || r.InstitutionSplit.PensionFund == nil || r.InstitutionSplit.PensionFund.NetBuy != 6 {
		t.Errorf("breakdown wrong: %+v", r.InstitutionSplit)
	}
	if r.Foreigner == nil || r.Foreigner.NetBuy != 60 {
		t.Errorf("foreigner = %+v", r.Foreigner)
	}
	// 다른 시리즈 필드는 건드리지 않는다.
	if r.ShortVolume != nil || r.MarginLoan != nil {
		t.Error("foreign series fields set on investor record")
	}
}

func TestSupplyShortSellingNullRates(t *testing.T) {
	body := `{"result":{"nextUntil":null,"records":[
	  {"date":"2026-01-05","shortSellingVolume":"1000","shortSellingAmount":"5000000",
	   "shortSellingVolumeRate":null,"shortSellingAmountRate":"4.2"}]}}`
	srv := supplyServer(t, "/api/v1/stocks/005930/short-selling", body)
	defer srv.Close()

	got, err := supplyClient(t, srv).Supply(context.Background(), "005930", domain.SupplyShort, 0, "")
	if err != nil {
		t.Fatalf("Supply: %v", err)
	}
	if got.NextUntil != "" {
		t.Errorf("null nextUntil = %q, want empty", got.NextUntil)
	}
	r := got.Records[0]
	if r.ShortVolume == nil || *r.ShortVolume != 1000 {
		t.Errorf("ShortVolume = %v", r.ShortVolume)
	}
	// 비중이 null 인 날이 실제로 있다 — 0% 로 보이면 안 된다.
	if r.ShortVolumeRate != nil {
		t.Errorf("null rate materialised as %v", *r.ShortVolumeRate)
	}
	if r.ShortAmountRate == nil || *r.ShortAmountRate != 4.2 {
		t.Errorf("ShortAmountRate = %v", r.ShortAmountRate)
	}
}

func TestSupplyPassesCountAndUntil(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":{"nextUntil":null,"records":[]}}`))
	}))
	defer srv.Close()

	if _, err := supplyClient(t, srv).Supply(context.Background(), "005930", domain.SupplyProgram, 30, "2026-01-01"); err != nil {
		t.Fatalf("Supply: %v", err)
	}
	if gotQuery != "count=30&until=2026-01-01" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestSupplyRejectsUnknownKind(t *testing.T) {
	srv := supplyServer(t, "/none", `{}`)
	defer srv.Close()
	if _, err := supplyClient(t, srv).Supply(context.Background(), "005930", domain.SupplyKind("bogus"), 0, ""); err == nil {
		t.Fatal("want error for unknown kind")
	}
}
