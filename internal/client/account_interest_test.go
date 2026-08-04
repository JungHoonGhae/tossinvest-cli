package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func interestServer(t *testing.T, gotAccountKey map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotAccountKey != nil {
			gotAccountKey[r.URL.Path] = r.Header.Get("accountKey")
		}
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
		case "/api/v1/interest/accounts/annual/history/years":
			json.NewEncoder(w).Encode(map[string]any{"result": []int{2024, 2025, 2026}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			if got := r.URL.Query().Get("year"); got != "2025" {
				t.Errorf("year query = %q, want 2025", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"year":          2025,
					"totalInterest": 1300,
					"monthlySchedule": []map[string]any{
						{"month": 1, "totalInterest": 0, "details": []any{}},
						{"month": 2, "totalInterest": 1300, "details": []map[string]any{
							{
								"date": "2025-02-11", "totalAmount": 1500, "tax": 200,
								"paymentAmount": 1300,
								// 산정기간이 지급월과 다르다 — 둘을 섞으면 안 된다.
								"startDate": "2024-11-01", "endDate": "2025-01-31",
								"estimated": false,
							},
							{
								"date": "2025-02-28", "totalAmount": 400, "tax": 0,
								"paymentAmount": 400,
								"startDate":     "2025-02-01", "endDate": "2025-02-28",
								"estimated": true,
							},
						}},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGetAccountInterest(t *testing.T) {
	keys := map[string]string{}
	server := interestServer(t, keys)
	defer server.Close()

	got, err := testClientFor(server).GetAccountInterest(t.Context(), 2025)
	if err != nil {
		t.Fatalf("GetAccountInterest() error = %v", err)
	}
	if got.Year != 2025 || got.Total != 1300 {
		t.Errorf("Year/Total = %d/%v, want 2025/1300", got.Year, got.Total)
	}
	if len(got.Monthly) != 2 {
		t.Fatalf("Monthly len = %d, want 2 (empty months are kept here; output drops them)", len(got.Monthly))
	}
	pays := got.Monthly[1].Payments
	if len(pays) != 2 {
		t.Fatalf("February payments = %d, want 2", len(pays))
	}
	// 세전액과 실지급액은 다른 값이다. 하나로 뭉개면 세금이 사라진다.
	if pays[0].Amount != 1500 || pays[0].Tax != 200 || pays[0].PaymentAmount != 1300 {
		t.Errorf("payment[0] = %+v, want amount=1500 tax=200 net=1300", pays[0])
	}
	// 산정기간은 지급월과 다르다.
	if pays[0].StartDate != "2024-11-01" || pays[0].EndDate != "2025-01-31" {
		t.Errorf("payment[0] accrual = %s~%s, want 2024-11-01~2025-01-31", pays[0].StartDate, pays[0].EndDate)
	}
	if pays[0].Estimated || !pays[1].Estimated {
		t.Errorf("estimated flags = %t/%t, want false/true", pays[0].Estimated, pays[1].Estimated)
	}
	// 계좌 스코프 헤더는 조회에만 붙는다 — /account/list 는 키를 알기 전에 부른다.
	if keys["/api/v1/interest/accounts/annual/history/by-payment-date"] != "1" {
		t.Errorf("by-payment-date accountKey = %q, want 1", keys["/api/v1/interest/accounts/annual/history/by-payment-date"])
	}
	if keys["/api/v1/account/list"] != "" {
		t.Errorf("/account/list accountKey = %q, want empty", keys["/api/v1/account/list"])
	}
}

func TestGetInterestYears(t *testing.T) {
	server := interestServer(t, nil)
	defer server.Close()

	got, err := testClientFor(server).GetInterestYears(t.Context())
	if err != nil {
		t.Fatalf("GetInterestYears() error = %v", err)
	}
	if len(got) != 3 || got[0] != 2024 || got[2] != 2026 {
		t.Errorf("years = %v, want [2024 2025 2026]", got)
	}
}
