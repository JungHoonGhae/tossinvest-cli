package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestGetPrimeStatusMember(t *testing.T) {
	// Only /api/v1/prime/* requests carry the accountKey header — it's set by
	// getJSONWithAccountKey, which primaryAccountKey's own /account/list call
	// (plain getJSON, no account key known yet) never goes through. Capture
	// per-path so the assertion below only checks the two calls that should
	// actually carry it.
	gotAccountKeyHeaders := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccountKeyHeaders[r.URL.Path] = r.Header.Get("accountKey")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"accountList": []map[string]any{
						{"key": "1", "displayName": "테스트계좌", "name": "종합매매", "type": "BROKERAGE", "markets": []string{"KR", "US"}},
					},
					"primaryKey": "1",
				},
			})
		case "/api/v1/prime/users/info":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"isMember":        true,
					"userId":          "u-test-1",
					"primeType":       "STANDARD",
					"benefitsStartAt": "2026-07-01",
					"benefitsEndAt":   "2026-07-31",
					"cycleNumber":     3,
				},
			})
		case "/api/v1/prime/users/benefits/cumulative":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"exchangeBenefit":    1000,
					"interestBenefitKrw": 2000,
					"interestBenefitUsd": 300,
					"totalKrw":           3300,
				},
			})
		case "/api/v1/prime/users/benefits":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"month":           "2026-07",
					"benefitsStartAt": "2026-07-01",
					"benefitsEndAt":   "2026-07-31",
					"exchange":        map[string]any{"nonPrimeFee": 25, "primeFee": 15, "benefitFee": 10},
					"interestKrw":     map[string]any{"status": "ACTIVE", "nonPrimeInterest": 1, "primeInterest": 2, "benefitInterest": 3},
					"interestUsd":     map[string]any{"status": "ACTIVE", "nonPrimeInterest": 1, "primeInterest": 2, "benefitInterest": 3},
					"baseRate":        3.5,
					"monthlyTotalKrw": 12345.6,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := testClientFor(server)
	status, err := c.GetPrimeStatus(t.Context())
	if err != nil {
		t.Fatalf("GetPrimeStatus() error = %v", err)
	}
	if !status.IsMember {
		t.Error("IsMember = false, want true")
	}
	if status.UserID == nil || *status.UserID != "u-test-1" {
		t.Errorf("UserID = %v, want u-test-1", status.UserID)
	}
	if status.PrimeType == nil || *status.PrimeType != "STANDARD" {
		t.Errorf("PrimeType = %v, want STANDARD", status.PrimeType)
	}
	if status.CycleNumber == nil || *status.CycleNumber != 3 {
		t.Errorf("CycleNumber = %v, want 3", status.CycleNumber)
	}
	if status.Month != "2026-07" {
		t.Errorf("Month = %q, want 2026-07", status.Month)
	}
	wantExchange := domain.PrimeExchangeFee{NonPrimeFee: 25, PrimeFee: 15, BenefitFee: 10}
	if status.Exchange != wantExchange {
		t.Errorf("Exchange = %+v, want %+v", status.Exchange, wantExchange)
	}
	if status.BaseRate != 3.5 {
		t.Errorf("BaseRate = %v, want 3.5", status.BaseRate)
	}
	if status.MonthlyTotalKRW != 12345.6 {
		t.Errorf("MonthlyTotalKRW = %v, want 12345.6", status.MonthlyTotalKRW)
	}
	// 이번 달치와 누적은 다른 값이다 — 섞이면 사용자가 혜택을 두 배로 읽는다.
	wantCum := domain.PrimeCumulative{Exchange: 1000, InterestKRW: 2000, InterestUSD: 300, TotalKRW: 3300}
	if status.Cumulative != wantCum {
		t.Errorf("Cumulative = %+v, want %+v", status.Cumulative, wantCum)
	}
	for _, path := range []string{"/api/v1/prime/users/info", "/api/v1/prime/users/benefits"} {
		if got := gotAccountKeyHeaders[path]; got != "1" {
			t.Errorf("%s: accountKey header = %q, want %q", path, got, "1")
		}
	}
	// /api/v1/account/list goes through primaryAccountKey's plain getJSON
	// (no account key known yet at that point) — it must NOT carry the header.
	if got := gotAccountKeyHeaders["/api/v1/account/list"]; got != "" {
		t.Errorf("/api/v1/account/list: accountKey header = %q, want empty", got)
	}
}

func TestGetPrimeStatusNonMember(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"accountList": []map[string]any{
						{"key": "1", "displayName": "테스트계좌", "name": "종합매매", "type": "BROKERAGE", "markets": []string{"KR"}},
					},
					"primaryKey": "1",
				},
			})
		case "/api/v1/prime/users/info":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"isMember":        false,
					"userId":          nil,
					"primeType":       nil,
					"benefitsStartAt": nil,
					"benefitsEndAt":   nil,
					"cycleNumber":     nil,
				},
			})
		case "/api/v1/prime/users/benefits/cumulative":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"exchangeBenefit":    1000,
					"interestBenefitKrw": 2000,
					"interestBenefitUsd": 300,
					"totalKrw":           3300,
				},
			})
		case "/api/v1/prime/users/benefits":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"month":           "2026-07",
					"benefitsStartAt": nil,
					"benefitsEndAt":   nil,
					"exchange":        map[string]any{"nonPrimeFee": 25, "primeFee": 15, "benefitFee": 25},
					"interestKrw":     map[string]any{"status": "ACTIVE", "nonPrimeInterest": 1, "primeInterest": 2, "benefitInterest": 1},
					"interestUsd":     map[string]any{"status": "ACTIVE", "nonPrimeInterest": 1, "primeInterest": 2, "benefitInterest": 1},
					"baseRate":        3.5,
					"monthlyTotalKrw": 0,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := testClientFor(server)
	status, err := c.GetPrimeStatus(t.Context())
	if err != nil {
		t.Fatalf("GetPrimeStatus() error = %v", err)
	}
	if status.IsMember {
		t.Error("IsMember = true, want false")
	}
	if status.UserID != nil {
		t.Errorf("UserID = %v, want nil", status.UserID)
	}
	if status.PrimeType != nil {
		t.Errorf("PrimeType = %v, want nil", status.PrimeType)
	}
	if status.CycleNumber != nil {
		t.Errorf("CycleNumber = %v, want nil", status.CycleNumber)
	}
	// Non-members still get a real benefit comparison (verified live).
	if status.Exchange.NonPrimeFee != 25 || status.Exchange.PrimeFee != 15 {
		t.Errorf("Exchange = %+v, want non_prime=25 prime=15", status.Exchange)
	}
}
