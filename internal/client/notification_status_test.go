package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestGetNotificationStatusMapsVerifiedSignals(t *testing.T) {
	t.Parallel()
	wantPaths := map[string]string{
		"/api/v1/inbox-alimies/has-unread":              `{"result":{"unread":true}}`,
		"/api/v1/ai-issue/sns-release/alimy":            `{"result":{"enabled":true}}`,
		"/api/v1/fomc-live/alimy":                       `{"result":{"enabled":false}}`,
		"/api/v1/reasoning-contents/alimy/subscription": `{"result":{"enabled":true}}`,
		"/api/v1/reasoning/agreement":                   `{"result":true}`,
		"/api/v1/reasoning-news/count":                  `{"result":7}`,
	}
	seen := make(map[string]int)
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, ok := wantPaths[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		seen[r.URL.Path]++
		mu.Unlock()
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetNotificationStatus(context.Background())
	if err != nil {
		t.Fatalf("GetNotificationStatus: %v", err)
	}
	if !got.InboxUnread || !got.AIIssueSNSReleaseAlertEnabled || got.FOMCLiveAlertEnabled ||
		!got.ReasoningContentsAlertEnabled || !got.ReasoningAgreement || got.ReasoningNewsCount != 7 || got.FetchedAt.IsZero() {
		t.Fatalf("status = %#v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for path := range wantPaths {
		if seen[path] != 1 {
			t.Errorf("%s called %d times, want 1", path, seen[path])
		}
	}
}
