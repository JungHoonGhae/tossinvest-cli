package ops

import (
	"context"
	"net/http"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestNotificationStatusOperationIsReadOnlyAndOwnsEveryProbe(t *testing.T) {
	t.Parallel()
	responses := map[string]string{
		"/api/v1/inbox-alimies/has-unread":              `{"result":{"unread":true}}`,
		"/api/v1/ai-issue/sns-release/alimy":            `{"result":{"enabled":true}}`,
		"/api/v1/fomc-live/alimy":                       `{"result":{"enabled":false}}`,
		"/api/v1/reasoning-contents/alimy/subscription": `{"result":{"enabled":true}}`,
		"/api/v1/reasoning/agreement":                   `{"result":true}`,
		"/api/v1/reasoning-news/count":                  `{"result":7}`,
	}
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	op, ok := NewCatalog().Get("notification_status")
	if !ok {
		t.Fatal("notification_status operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 5 {
		t.Fatalf("operation metadata = %#v", op)
	}
	gotAny, err := NewCatalog().Call(context.Background(), deps, "notification_status", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := gotAny.(domain.NotificationStatus)
	if !got.InboxUnread || !got.AIIssueSNSReleaseAlertEnabled || got.FOMCLiveAlertEnabled ||
		!got.ReasoningContentsAlertEnabled || !got.ReasoningAgreement || got.ReasoningNewsCount != 7 {
		t.Fatalf("status = %#v", got)
	}

	probes := append([]ProbeSpec{*op.Probe}, op.ExtraProbes...)
	if len(probes) != len(responses) {
		t.Fatalf("probe count = %d, want %d", len(probes), len(responses))
	}
	for _, probe := range probes {
		body, ok := responses[newRequestPath(probe.URL)]
		if !ok {
			t.Errorf("unexpected probe %q URL %s", probe.Name, probe.URL)
			continue
		}
		if err := probe.Check(http.StatusOK, []byte(body)); err != nil {
			t.Errorf("%s rejected verified schema: %v", probe.Name, err)
		}
	}
}

func newRequestPath(rawURL string) string {
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	return req.URL.Path
}
