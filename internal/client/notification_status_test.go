package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGetNotificationStatusSummarizesExistingSettingsAndUniqueSignals(t *testing.T) {
	t.Parallel()
	wantPaths := map[string]string{
		"/api/v1/user-alimies": `{"result":[` +
			`{"id":1,"type":"AI_ISSUE_SNS_RELEASE","enabled":true},` +
			`{"id":2,"type":"FOMC_LIVE","enabled":false},` +
			`{"id":3,"type":"REASONING_SUBSCRIPTION","enabled":true}` +
			`]}`,
		"/api/v1/inbox-alimies/has-unread": `{"result":{"unread":true}}`,
		"/api/v1/reasoning/agreement":      `{"result":true}`,
		"/api/v1/reasoning-news/count":     `{"result":7}`,
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

func TestGetNotificationStatusFailsWithDeterministicDependencyLabel(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/user-alimies" {
			http.Error(w, "synthetic settings failure", http.StatusBadGateway)
			return
		}
		// These dependencies succeed so the aggregate error must identify the
		// failed canonical settings read rather than whichever goroutine ended last.
		if r.URL.Path == "/api/v1/inbox-alimies/has-unread" {
			_, _ = w.Write([]byte(`{"result":{"unread":false}}`))
			return
		}
		if r.URL.Path == "/api/v1/reasoning/agreement" {
			_, _ = w.Write([]byte(`{"result":false}`))
			return
		}
		if r.URL.Path == "/api/v1/reasoning-news/count" {
			_, _ = w.Write([]byte(`{"result":0}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	_, err := testClientFor(server).GetNotificationStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "notification settings") || !strings.Contains(err.Error(), "502") {
		t.Fatalf("error=%v", err)
	}
}

func TestGetNotificationStatusRejectsMissingCanonicalSettingTypes(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user-alimies":
			_, _ = w.Write([]byte(`{"result":[{"type":"AI_ISSUE_SNS_RELEASE","enabled":false}]}`))
		case "/api/v1/inbox-alimies/has-unread":
			_, _ = w.Write([]byte(`{"result":{"unread":false}}`))
		case "/api/v1/reasoning/agreement":
			_, _ = w.Write([]byte(`{"result":false}`))
		case "/api/v1/reasoning-news/count":
			_, _ = w.Write([]byte(`{"result":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := testClientFor(server).GetNotificationStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "FOMC_LIVE, REASONING_SUBSCRIPTION") {
		t.Fatalf("error=%v", err)
	}
}

func TestGetNotificationStatusRejectsMissingBooleanAndCountFields(t *testing.T) {
	t.Parallel()
	for name, brokenPath := range map[string]string{
		"inbox unread": "/api/v1/inbox-alimies/has-unread",
		"agreement":    "/api/v1/reasoning/agreement",
		"news count":   "/api/v1/reasoning-news/count",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				responses := map[string]string{
					"/api/v1/user-alimies":             `{"result":[{"type":"AI_ISSUE_SNS_RELEASE","enabled":false},{"type":"FOMC_LIVE","enabled":false},{"type":"REASONING_SUBSCRIPTION","enabled":false}]}`,
					"/api/v1/inbox-alimies/has-unread": `{"result":{"unread":false}}`,
					"/api/v1/reasoning/agreement":      `{"result":false}`,
					"/api/v1/reasoning-news/count":     `{"result":0}`,
				}
				if r.URL.Path == brokenPath {
					_, _ = w.Write([]byte(`{"result":{}}`))
					return
				}
				_, _ = w.Write([]byte(responses[r.URL.Path]))
			}))
			t.Cleanup(server.Close)

			if _, err := testClientFor(server).GetNotificationStatus(context.Background()); err == nil {
				t.Fatalf("missing field at %s was accepted", brokenPath)
			}
		})
	}
}
