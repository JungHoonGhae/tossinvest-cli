package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func orderHistoryContractClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(Config{
		HTTPClient: server.Client(), APIBaseURL: server.URL,
		InfoBaseURL: server.URL, CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
			Headers: map[string]string{"App-Version": "v260914.0000", "Browser-Tab-Id": "test-tab"},
		},
	})
}

func TestOrderHistoryRejectsMissingLists(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		completed  bool
		wantError  bool
	}{
		{"pending missing result", `{}`, false, true},
		{"pending null result", `{"result":null}`, false, true},
		{"pending changed wrapper", `{"data":[]}`, false, true},
		{"pending empty list", `{"result":[]}`, false, false},
		{"completed missing result", `{}`, true, true},
		{"completed null result", `{"result":null}`, true, true},
		{"completed missing body", `{"result":{}}`, true, true},
		{"completed null body", `{"result":{"body":null}}`, true, true},
		{"completed changed wrapper", `{"result":{"orders":[]}}`, true, true},
		{"completed empty list", `{"result":{"body":[]}}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := orderHistoryContractClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}
				_, _ = w.Write([]byte(tc.body))
			})
			var err error
			if tc.completed {
				date := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
				_, err = c.ListCompletedOrdersRange(context.Background(), "us", date, date, 1, 1)
			} else {
				_, err = c.ListPendingOrders(context.Background())
			}
			if (err != nil) != tc.wantError {
				t.Fatalf("want error=%t, got %v", tc.wantError, err)
			}
		})
	}
}

func TestFindOrderStopsOnInvalidPendingHistory(t *testing.T) {
	t.Parallel()
	var completedCalls atomic.Int32
	c := orderHistoryContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/completed") {
			completedCalls.Add(1)
			_, _ = w.Write([]byte(`{"result":{"body":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":null}`))
	})
	_, err := c.FindOrder(context.Background(), "synthetic-order-id", "us")
	if err == nil || strings.Contains(err.Error(), "was not found") {
		t.Fatalf("invalid history must fail before order lookup, got %v", err)
	}
	if got := completedCalls.Load(); got != 0 {
		t.Fatalf("completed history queried after invalid pending response: %d", got)
	}
}
