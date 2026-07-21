package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestListAccumulationPlans(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/autotrade/plan/find" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		// dummy data — never real account values
		_, _ = w.Write([]byte(`{"result":[{"id":1,"symbol":"DUMMY","stockCode":"A000000","stockName":"Dummy Corp","countryCode":"KR","currency":"KRW","planType":"AMOUNT","iteration":"DAILY","iterateTarget":1,"investAmt":10000,"investQty":0,"tradeStatus":"PROGRESS","isPaused":false,"investStartDate":"2026-01-01","investEndDate":"9999-12-31","proceededRound":3,"succeededRound":3,"totalExecutedAmount":30000,"totalExecutedQuantity":1.5,"createdAt":"2026-01-01T00:00:00","updatedAt":"2026-01-04T00:00:00"}]}`))
	}))
	defer srv.Close()

	c := New(Config{
		APIBaseURL: srv.URL,
		Session:    &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	got, err := c.ListAccumulationPlans(context.Background())
	if err != nil {
		t.Fatalf("ListAccumulationPlans: %v", err)
	}
	if len(got.Plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(got.Plans))
	}
	p := got.Plans[0]
	if p.Symbol != "DUMMY" || p.StockName != "Dummy Corp" || p.IsPaused {
		t.Errorf("plan: %+v", p)
	}
	if p.PlanType != "AMOUNT" || p.Iteration != "DAILY" || p.InvestAmount != 10000 {
		t.Errorf("schedule fields: %+v", p)
	}
	if p.SucceededRound != 3 || p.TotalExecutedAmount != 30000 {
		t.Errorf("progress fields: %+v", p)
	}
}

func TestListAccumulationPlansNoSession(t *testing.T) {
	c := New(Config{})
	if _, err := c.ListAccumulationPlans(context.Background()); err == nil {
		t.Fatal("want error without a session")
	}
}

func TestGetAccumulationPlansByStock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/growth/autotrade/plan/stock/A005930" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		// dummy data — never real account values
		_, _ = w.Write([]byte(`{"result":[{"id":2,"symbol":"005930","stockCode":"A005930","stockName":"Dummy Electronics","countryCode":"KR","currency":"KRW","planType":"QUANTITY","iteration":"WEEKLY","iterateTarget":2,"investAmt":0,"investQty":1,"isPaused":true}]}`))
	}))
	defer srv.Close()

	c := New(Config{
		CertBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	// "005930" normalizes to the product code "A005930" and already looks
	// like one, so no search round-trip is needed — this exercises that path.
	got, err := c.GetAccumulationPlansByStock(context.Background(), "005930")
	if err != nil {
		t.Fatalf("GetAccumulationPlansByStock: %v", err)
	}
	if len(got.Plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(got.Plans))
	}
	p := got.Plans[0]
	if p.StockCode != "A005930" || !p.IsPaused || p.PlanType != "QUANTITY" || p.InvestQuantity != 1 {
		t.Errorf("plan: %+v", p)
	}
}

func TestGetAccumulationPlansByStockNoSession(t *testing.T) {
	c := New(Config{})
	if _, err := c.GetAccumulationPlansByStock(context.Background(), "A005930"); err == nil {
		t.Fatal("want error without a session")
	}
}
