package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func TestGetOptionTradingHours(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/usa-market/get-option-biz-day-by-overtime" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"prevBizDay": map[string]any{
					"usaDate": "2026-07-31", "startDateTime": "2026-07-31T22:30:00.000+09:00",
					"endDateTime": "2026-08-01T05:00:00.000+09:00",
					// 옵션엔 프리/애프터 세션이 없어 null 로 온다.
					"preMarketStartDateTime": nil, "preMarketEndDateTime": nil,
					"afterMarketStartDateTime": nil, "afterMarketEndDateTime": nil,
				},
				"today": map[string]any{
					"usaDate": "2026-08-03", "startDateTime": "2026-08-03T22:30:00.000+09:00",
					"endDateTime": "2026-08-04T05:00:00.000+09:00",
				},
				"nextBizDay": map[string]any{
					"usaDate": "2026-08-04", "startDateTime": "2026-08-04T22:30:00.000+09:00",
					"endDateTime": "2026-08-05T05:00:00.000+09:00",
				},
			},
		})
	}))
	defer server.Close()

	got, err := testClientFor(server).GetOptionTradingHours(t.Context())
	if err != nil {
		t.Fatalf("GetOptionTradingHours() error = %v", err)
	}
	if got.Previous.Date != "2026-07-31" || got.Today.Date != "2026-08-03" || got.Next.Date != "2026-08-04" {
		t.Errorf("dates = %s/%s/%s, want 07-31/08-03/08-04", got.Previous.Date, got.Today.Date, got.Next.Date)
	}
	// null 을 빈 문자열로 받아야 한다 — 없는 세션을 정규장으로 채우면 안 된다.
	if got.Previous.PreMarketStart != "" || got.Previous.AfterMarketEnd != "" {
		t.Errorf("absent extended session should stay empty, got %q/%q", got.Previous.PreMarketStart, got.Previous.AfterMarketEnd)
	}
	// 세션이 자정을 넘긴다 — 종료가 다음 날짜다.
	if got.Today.End != "2026-08-04T05:00:00.000+09:00" {
		t.Errorf("Today.End = %q, want the next-day close", got.Today.End)
	}
}
