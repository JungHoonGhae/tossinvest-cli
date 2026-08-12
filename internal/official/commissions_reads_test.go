package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptCommissionsUnit verifies the pure adapter for Commissions.
func TestAdaptCommissionsUnit(t *testing.T) {
	raw := []apiCommission{
		{
			MarketCountry:  "KR",
			CommissionRate: "0.00015",
			StartDate:      "2026-01-01",
			EndDate:        "2026-12-31",
		},
	}
	got := adaptCommissions("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	// spec 1.2.14 의 example 그대로: "0.00015" 는 0.015% 를 뜻하는 **소수 비율**이다.
	// 어댑터는 스케일을 건드리지 않고 그대로 통과시킨다 — 퍼센트 변환은 출력에서
	// formatPercent(×100)가 한다. 여기서 100 을 곱하기 시작하면 표시가 100배로 뛴다.
	if got.CommissionRate != 0.00015 {
		t.Fatalf("CommissionRate: want 0.00015 (unscaled), got %v", got.CommissionRate)
	}
	if got.TaxRate != 0 {
		t.Fatalf("TaxRate: expected 0 (not in response), got %v", got.TaxRate)
	}
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
}

// TestAdaptCommissionsEmpty verifies empty response returns zero rate.
func TestAdaptCommissionsEmpty(t *testing.T) {
	got := adaptCommissions("005930", nil)
	if got.CommissionRate != 0 {
		t.Fatalf("CommissionRate: want 0 for empty, got %v", got.CommissionRate)
	}
}

// TestCommissionsIntegration tests Commissions() against an httptest server.
func TestCommissionsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/commissions":
			if r.URL.Query().Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", r.URL.Query().Get("symbol"))
			}
			_, _ = w.Write([]byte(`{"result":[{"marketCountry":"KR","commissionRate":"0.015","startDate":"2026-01-01","endDate":"2026-12-31"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)

	got, err := c.Commissions(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if got.CommissionRate != 0.015 {
		t.Fatalf("CommissionRate: want 0.015, got %v", got.CommissionRate)
	}
}
