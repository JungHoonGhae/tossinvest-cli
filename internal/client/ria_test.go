package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func riaServer(t *testing.T, limitStatus, optStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"accountList": []map[string]any{{"key": "1", "displayName": "테스트계좌", "markets": []string{"US"}}},
					"primaryKey":  "1",
				},
			})
		case "/api/v1/ria-calculator/report":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"transferIncomeTax": map[string]any{
						"estimatedTransferIncomeTax": 3000, "estimatedTaxSaving": 1200, "finalTransferIncomeTax": 1800,
					},
					"transferIncomeTaxDetail": map[string]any{
						"totalTransferIncomeAmount": 20000, "normalAccountTransferIncome": 15000,
						"riaAccountTransferIncome": 5000, "baseDeduction": 2500,
						"riaDeduction": map[string]any{
							"deductionRate": 0.2, "normalAccountOverseasBuyAmount": 9000,
							"normalAccountOverseasSellAmount": 8000, "riaAccountOverseasSellAmount": 4000,
							"preAdjustmentDeduction": 1000, "totalAmount": 1500,
							"quarterlyProfitLoss": []map[string]any{
								{"quarter": "Q1", "weight": 1, "totalProfitLoss": 400, "weightedTotalProfitLoss": 400},
								// 서버는 하반기를 반기로 준다 — 라벨을 파싱하면 안 된다.
								{"quarter": "H2", "weight": 0.5, "totalProfitLoss": 800, "weightedTotalProfitLoss": 400},
							},
						},
						"profitAfterDeduction": 16000, "transferIncomeTaxRate": 0.2,
						"transferIncomeTax": 1636, "localTaxRate": 0.02, "localTax": 164,
					},
				},
			})
		case "/api/v1/ria-calculator/limit":
			if limitStatus != 200 {
				w.WriteHeader(limitStatus)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"totalLimit": 50000, "remainingLimit": 20000,
					"overseasStockSellAmount": 30000, "settlementDate": nil, "settled": true,
				},
			})
		case "/api/v1/ria-calculator/tax-savings/optimized":
			if optStatus != 200 {
				w.WriteHeader(optStatus)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"estimatedMaxTaxSaving": 0, "zeroReasonCode": "NO_PROFITABLE_STOCKS"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGetRIAReport(t *testing.T) {
	server := riaServer(t, 200, 200)
	defer server.Close()

	got, err := testClientFor(server).GetRIAReport(t.Context())
	if err != nil {
		t.Fatalf("GetRIAReport() error = %v", err)
	}
	if got.EstimatedTaxSaving != 1200 || got.FinalTransferIncomeTax != 1800 {
		t.Errorf("saving/final = %v/%v, want 1200/1800", got.EstimatedTaxSaving, got.FinalTransferIncomeTax)
	}
	// 일반계좌와 RIA계좌 양도소득을 합치면 RIA 공제 근거가 사라진다.
	if got.NormalAccountTransferIncome != 15000 || got.RIAAccountTransferIncome != 5000 {
		t.Errorf("income split = %v/%v, want 15000/5000", got.NormalAccountTransferIncome, got.RIAAccountTransferIncome)
	}
	if len(got.Deduction.QuarterlyProfitLoss) != 2 {
		t.Fatalf("quarters = %d, want 2", len(got.Deduction.QuarterlyProfitLoss))
	}
	// 가중치를 금액에 접어 넣으면 왜 그 공제액인지 못 보여준다.
	q := got.Deduction.QuarterlyProfitLoss[1]
	if q.Quarter != "H2" || q.Weight != 0.5 || q.TotalProfitLoss != 800 || q.WeightedProfitLoss != 400 {
		t.Errorf("second period = %+v, want H2/0.5/800/400", q)
	}
	if got.Limit == nil || got.Limit.RemainingLimit != 20000 {
		t.Errorf("limit = %+v, want remaining 20000", got.Limit)
	}
	// 절세 여지가 0일 때 서버 원문 사유 코드를 보존해야 한다.
	if got.MaxTaxSaving == nil || *got.MaxTaxSaving != 0 || got.ZeroReasonCode != "NO_PROFITABLE_STOCKS" {
		t.Errorf("optimized = %v/%q, want 0/NO_PROFITABLE_STOCKS", got.MaxTaxSaving, got.ZeroReasonCode)
	}
}

// limit·optimized 는 부가 정보다. RIA 약정이 없는 계좌에서 이들이 실패해도
// 리포트 본체는 나와야 한다.
func TestGetRIAReportSurvivesEnrichmentFailure(t *testing.T) {
	server := riaServer(t, 500, 500)
	defer server.Close()

	got, err := testClientFor(server).GetRIAReport(t.Context())
	if err != nil {
		t.Fatalf("GetRIAReport() error = %v, want nil (enrichments are best-effort)", err)
	}
	if got.EstimatedTaxSaving != 1200 {
		t.Errorf("EstimatedTaxSaving = %v, want 1200", got.EstimatedTaxSaving)
	}
	if got.Limit != nil || got.MaxTaxSaving != nil {
		t.Errorf("failed enrichments should stay nil, got limit=%v saving=%v", got.Limit, got.MaxTaxSaving)
	}
}
