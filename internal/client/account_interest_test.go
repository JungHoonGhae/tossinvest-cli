package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
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

func TestGetAccountInterestEnrichesEmptyYearWithAvailableYears(t *testing.T) {
	accountListCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			accountListCalls++
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0, "monthlySchedule": []any{},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			if got := r.Header.Get("accountKey"); got != "1" {
				t.Errorf("years accountKey = %q, want 1", got)
			}
			json.NewEncoder(w).Encode(map[string]any{"result": []int{2024, 2025, 2026}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	got, err := testClientFor(server).GetAccountInterest(t.Context(), 2023)
	if err != nil {
		t.Fatalf("GetAccountInterest() error = %v", err)
	}
	if len(got.AvailableYears) != 3 || got.AvailableYears[0] != 2024 || got.AvailableYears[2] != 2026 {
		t.Fatalf("AvailableYears = %v, want [2024 2025 2026]", got.AvailableYears)
	}
	if accountListCalls != 1 {
		t.Fatalf("account list calls = %d, want 1 shared account lookup", accountListCalls)
	}
}

func TestGetAccountInterestKeepsEmptyReportWhenYearEnrichmentFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0, "monthlySchedule": []any{},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	got, err := testClientFor(server).GetAccountInterest(t.Context(), 2023)
	if err != nil {
		t.Fatalf("main interest report must survive optional year lookup: %v", err)
	}
	if got.Year != 2023 || got.Total != 0 || len(got.AvailableYears) != 0 {
		t.Fatalf("interest report = %+v, want empty 2023 report without available years", got)
	}
}

func TestGetAccountInterestEnrichesMonthsWithoutPayments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0,
				"monthlySchedule": []map[string]any{{"month": 1, "totalInterest": 0, "details": []any{}}},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			json.NewEncoder(w).Encode(map[string]any{"result": []int{2023, 2024}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	got, err := testClientFor(server).GetAccountInterest(t.Context(), 2023)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Monthly) != 1 || len(got.Monthly[0].Payments) != 0 {
		t.Fatalf("empty monthly row changed: %+v", got.Monthly)
	}
	if len(got.AvailableYears) != 2 || got.AvailableYears[1] != 2024 {
		t.Fatalf("months without payments must still enrich available years: %v", got.AvailableYears)
	}
}

func TestGetAccountInterestCachesSuccessfulYearEnrichment(t *testing.T) {
	yearCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0, "monthlySchedule": []any{},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			yearCalls++
			json.NewEncoder(w).Encode(map[string]any{"result": []int{2023, 2024}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := testClientFor(server)
	for range 2 {
		got, err := c.GetAccountInterest(t.Context(), 2023)
		if err != nil || len(got.AvailableYears) != 2 {
			t.Fatalf("cached enrichment failed: got=%+v err=%v", got, err)
		}
	}
	if yearCalls != 1 {
		t.Fatalf("year endpoint calls = %d, want 1 successful cached lookup", yearCalls)
	}
}

func TestGetAccountInterestBoundsOptionalYearEnrichmentLatency(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0, "monthlySchedule": []any{},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			select {
			case <-release:
				json.NewEncoder(w).Encode(map[string]any{"result": []int{2023}})
			case <-r.Context().Done():
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = 2 * time.Second
	c := New(Config{
		HTTPClient: httpClient, APIBaseURL: server.URL, InfoBaseURL: server.URL, CertBaseURL: server.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "x"}},
	})
	started := time.Now()
	got, err := c.GetAccountInterest(t.Context(), 2023)
	close(release)
	if err != nil {
		t.Fatalf("main report must survive slow optional enrichment: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 1500*time.Millisecond {
		t.Fatalf("optional enrichment delayed main report for %s", elapsed)
	}
	if got.Year != 2023 || len(got.AvailableYears) != 0 {
		t.Fatalf("unexpected empty report after enrichment timeout: %+v", got)
	}
}

type interestYearsCallResult struct {
	years []int
	err   error
}

func blockingInterestServer(t *testing.T) (*httptest.Server, <-chan struct{}, chan<- struct{}, *atomic.Int32) {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var yearCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/by-payment-date":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"year": 2023, "totalInterest": 0, "monthlySchedule": []any{},
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			yearCalls.Add(1)
			startedOnce.Do(func() { close(started) })
			select {
			case <-release:
				json.NewEncoder(w).Encode(map[string]any{"result": []int{2023, 2024}})
			case <-r.Context().Done():
			}
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	return server, started, release, &yearCalls
}

func TestInterestYearSingleflightKeepsOptionalWaiterDeadlineIndependent(t *testing.T) {
	server, started, release, calls := blockingInterestServer(t)
	defer server.Close()
	c := testClientFor(server)

	explicitDone := make(chan interestYearsCallResult, 1)
	go func() {
		years, err := c.GetInterestYears(context.Background())
		explicitDone <- interestYearsCallResult{years: years, err: err}
	}()
	<-started

	optionalDone := make(chan error, 1)
	go func() {
		_, err := c.GetAccountInterest(context.Background(), 2023)
		optionalDone <- err
	}()
	select {
	case err := <-optionalDone:
		if err != nil {
			t.Fatalf("optional report failed: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		close(release)
		<-explicitDone
		t.Fatal("optional waiter inherited the explicit lookup's longer wait")
	}
	select {
	case result := <-explicitDone:
		close(release)
		t.Fatalf("explicit lookup finished before its shared request was released: %+v", result)
	default:
	}
	close(release)
	result := <-explicitDone
	if result.err != nil || len(result.years) != 2 {
		t.Fatalf("explicit lookup after release = %+v", result)
	}
	if calls.Load() != 1 {
		t.Fatalf("year endpoint calls = %d, want one shared request", calls.Load())
	}
}

func TestInterestYearSingleflightDoesNotImposeOptionalTimeoutOnExplicitWaiter(t *testing.T) {
	server, started, release, calls := blockingInterestServer(t)
	defer server.Close()
	c := testClientFor(server)

	optionalDone := make(chan error, 1)
	go func() {
		_, err := c.GetAccountInterest(context.Background(), 2023)
		optionalDone <- err
	}()
	<-started
	explicitDone := make(chan interestYearsCallResult, 1)
	go func() {
		years, err := c.GetInterestYears(context.Background())
		explicitDone <- interestYearsCallResult{years: years, err: err}
	}()

	select {
	case err := <-optionalDone:
		if err != nil {
			t.Fatalf("optional report failed: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		close(release)
		t.Fatal("optional waiter exceeded its enrichment deadline")
	}
	select {
	case result := <-explicitDone:
		close(release)
		t.Fatalf("explicit waiter inherited the optional timeout: %+v", result)
	default:
	}
	close(release)
	result := <-explicitDone
	if result.err != nil || len(result.years) != 2 {
		t.Fatalf("explicit lookup after release = %+v", result)
	}
	if calls.Load() != 1 {
		t.Fatalf("year endpoint calls = %d, want one shared request", calls.Load())
	}
}

func TestInterestYearSharedRequestCancelsAfterLastWaiterLeaves(t *testing.T) {
	started := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/account/list":
			json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"accountList": []map[string]any{{"key": "1", "type": "BROKERAGE"}},
				"primaryKey":  "1",
			}})
		case "/api/v1/interest/accounts/annual/history/years":
			close(started)
			<-r.Context().Done()
			close(requestCanceled)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := testClientFor(server).GetInterestYears(ctx)
		done <- err
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetInterestYears error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller did not return after cancellation")
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("shared HTTP request survived after its last waiter left")
	}
}
