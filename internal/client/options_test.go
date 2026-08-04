package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실시세 아님.
func optionsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US00000000001"}]}}`))
		case "/api/v1/option-maturity-date/get-all":
			if got := r.URL.Query().Get("underlyingGuid"); got != "US00000000001" {
				t.Fatalf("underlyingGuid = %q", got)
			}
			w.Write([]byte(`{"result":{"items":[
			  {"maturityDate":"2026-01-02","maturityDateTime":"2026-01-02T03:50:00.000+09:00",
			   "liquidationDateTime":"2026-01-02T14:50:00-05:00",
			   "displayLiquidationDateTime":"거래 종료","displayCorporateActionName":null},
			  {"maturityDate":"2026-01-09","displayLiquidationDateTime":"","displayCorporateActionName":"더미액션"}]}}`))
		case "/api/v1/option-both-chain/get-all":
			if got := r.URL.Query().Get("maturityDate"); got != "2026-01-09" {
				t.Fatalf("maturityDate = %q", got)
			}
			w.Write([]byte(`{"result":[
			  {"strikePrice":100.0,"callGuid":"OPT_C1","putGuid":"OPT_P1","callOpenInterest":5,"putOpenInterest":0},
			  {"strikePrice":105.0,"callGuid":"OPT_C2","putGuid":"OPT_P2","callOpenInterest":0,"putOpenInterest":7}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGetOptionExpiries(t *testing.T) {
	server := optionsServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetOptionExpiries(t.Context(), "DUMMY")
	if err != nil {
		t.Fatalf("GetOptionExpiries() error = %v", err)
	}
	if len(got.Expiries) != 2 {
		t.Fatalf("len(Expiries) = %d, want 2", len(got.Expiries))
	}
	// 서버 표시 문자열 그대로 — 타임스탬프만으로는 구분 안 되는 상태를 담는다.
	if got.Expiries[0].DisplayLiquidation != "거래 종료" {
		t.Errorf("DisplayLiquidation = %q", got.Expiries[0].DisplayLiquidation)
	}
	// null 은 빈 문자열, 값이 있으면 그대로.
	if got.Expiries[0].CorporateActionName != "" {
		t.Errorf("null corporate action materialised: %q", got.Expiries[0].CorporateActionName)
	}
	if got.Expiries[1].CorporateActionName != "더미액션" {
		t.Errorf("CorporateActionName = %q", got.Expiries[1].CorporateActionName)
	}
}

func TestGetOptionChain(t *testing.T) {
	server := optionsServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetOptionChain(t.Context(), "DUMMY", "2026-01-09")
	if err != nil {
		t.Fatalf("GetOptionChain() error = %v", err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(got.Rows))
	}
	// 서버가 행사가 오름차순으로 준다 — 재정렬하면 앱과 어긋난다.
	if got.Rows[0].StrikePrice != 100 || got.Rows[1].StrikePrice != 105 {
		t.Errorf("order changed: %+v", got.Rows)
	}
	if got.Rows[0].CallOpenInterest != 5 || got.Rows[1].PutOpenInterest != 7 {
		t.Errorf("open interest wrong: %+v", got.Rows)
	}
}

func TestGetOptionChainRequiresMaturity(t *testing.T) {
	server := optionsServer(t)
	defer server.Close()

	if _, err := testClientFor(server).GetOptionChain(t.Context(), "DUMMY", ""); err == nil {
		t.Fatal("want error for empty maturity date, got nil")
	}
}
