package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func commissionServer(t *testing.T, usOpt any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/trading/commission-info" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"commissionInfoKr": map[string]any{"commissionRate": 0.011},
				"commissionInfoUs": map[string]any{
					"commissionRate":         0.2,
					"hasCommissionReduction": true,
					"reductionEndDate":       "2026-12-31",
					"reductionEndDateTime":   "2026-12-31T23:59:59",
				},
				"commissionInfoUsOpt": usOpt,
			},
		})
	}))
}

func TestGetCommissionSchedule(t *testing.T) {
	// 옵션 티어는 우대 없음인데 서버가 sentinel 종료일을 준다 — 실제 응답에서
	// 관측된 형태다. 그대로 흘리면 "우대 진행 중" 으로 오독된다.
	server := commissionServer(t, map[string]any{
		"commissionPerContract":  2.49,
		"hasCommissionReduction": false,
		"reductionEndDateTime":   "9999-12-31T00:50:00.000+00:00",
	})
	defer server.Close()

	got, err := testClientFor(server).GetCommissionSchedule(t.Context())
	if err != nil {
		t.Fatalf("GetCommissionSchedule() error = %v", err)
	}
	// 서버가 주는 rate 는 이미 퍼센트 단위다 (0.011 = 0.011%). 스케일을
	// 건드리면 안 된다 — 여기서 100배 곱셈이 끼면 이 단언이 깨진다.
	if got.Korea.RatePercent != 0.011 {
		t.Errorf("Korea.RatePercent = %v, want 0.011", got.Korea.RatePercent)
	}
	if got.US.RatePercent != 0.2 {
		t.Errorf("US.RatePercent = %v, want 0.2", got.US.RatePercent)
	}
	if !got.US.HasReduction || got.US.ReductionEndAt != "2026-12-31T23:59:59" {
		t.Errorf("US reduction = %v/%q, want true/2026-12-31T23:59:59", got.US.HasReduction, got.US.ReductionEndAt)
	}
	if got.USOptions == nil {
		t.Fatal("USOptions = nil, want a tier")
	}
	if got.USOptions.PerContract != 2.49 {
		t.Errorf("USOptions.PerContract = %v, want 2.49", got.USOptions.PerContract)
	}
	if got.USOptions.RatePercent != 0 {
		t.Errorf("USOptions.RatePercent = %v, want 0 (options charge per contract)", got.USOptions.RatePercent)
	}
	if got.USOptions.ReductionEndAt != "" {
		t.Errorf("USOptions.ReductionEndAt = %q, want empty (no reduction in effect)", got.USOptions.ReductionEndAt)
	}
}

// 옵션 미신청 계좌는 null 을 받는다. 0원 수수료 티어로 렌더하면 안 되므로
// 포인터가 nil 로 남는지 확인한다.
func TestGetCommissionScheduleNoOptionsTier(t *testing.T) {
	server := commissionServer(t, nil)
	defer server.Close()

	got, err := testClientFor(server).GetCommissionSchedule(t.Context())
	if err != nil {
		t.Fatalf("GetCommissionSchedule() error = %v", err)
	}
	if got.USOptions != nil {
		t.Errorf("USOptions = %+v, want nil", got.USOptions)
	}
}
