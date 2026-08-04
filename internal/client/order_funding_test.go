package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌 데이터 아님.
func TestGetOrderFunding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"accountList": []map[string]any{{"key": "1", "displayName": "테스트계좌"}},
					"primaryKey":  "1",
				},
			})
		case "/api/v2/trading/order/buy-control/required-deposit-amount":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"buyable": false, "receivableCurrency": "USD",
					"krwAmount": 1000, "usdAmount": 2,
					"usdReceivableKrwEquivalentAmount": 2700,
					"krwMoneyOutAbleAmount":            900,
					"requiredDepositAmount":            5000,
					"requiredExchangeAmount":           3,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	got, err := testClientFor(server).GetOrderFunding(t.Context())
	if err != nil {
		t.Fatalf("GetOrderFunding() error = %v", err)
	}
	if got.Buyable {
		t.Error("Buyable = true, want false")
	}
	// 부족분이 이 엔드포인트의 존재 이유다 — 잔고로 대체할 수 없다.
	if got.RequiredDepositAmount != 5000 || got.RequiredExchangeAmount != 3 {
		t.Errorf("required deposit/exchange = %v/%v, want 5000/3", got.RequiredDepositAmount, got.RequiredExchangeAmount)
	}
	if got.KRWAmount != 1000 || got.USDAmount != 2 || got.KRWWithdrawable != 900 {
		t.Errorf("balances = %v/%v/%v, want 1000/2/900", got.KRWAmount, got.USDAmount, got.KRWWithdrawable)
	}
	if got.ReceivableCurrency != "USD" {
		t.Errorf("ReceivableCurrency = %q, want USD", got.ReceivableCurrency)
	}
}
