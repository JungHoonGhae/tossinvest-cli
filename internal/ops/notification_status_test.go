package ops

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestNotificationStatusOperationReusesSettingsAndOwnsUniqueProbes(t *testing.T) {
	t.Parallel()
	responses := map[string]string{
		"/api/v1/user-alimies": `{"result":[` +
			`{"id":1,"type":"AI_ISSUE_SNS_RELEASE","enabled":true},` +
			`{"id":2,"type":"FOMC_LIVE","enabled":false},` +
			`{"id":3,"type":"REASONING_SUBSCRIPTION","enabled":true},` +
			`{"id":4,"type":null,"enabled":false}` +
			`]}`,
		"/api/v1/inbox-alimies/has-unread": `{"result":{"unread":true}}`,
		"/api/v1/reasoning/agreement":      `{"result":true}`,
		"/api/v1/reasoning-news/count":     `{"result":7}`,
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
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 2 || !slices.Contains(op.ProbeRefs, "notification-settings") {
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
	if len(probes) != 3 {
		t.Fatalf("probe count = %d, want 3", len(probes))
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

	for _, probe := range sharedWTSProbes() {
		if probe.Name != "notification-settings" {
			continue
		}
		if err := probe.Check(http.StatusOK, []byte(`{"result":[{"type":"AI_ISSUE_SNS_RELEASE","enabled":true}]}`)); err == nil {
			t.Fatal("notification-settings probe accepted missing canonical setting types")
		}
		if err := probe.Check(http.StatusOK, []byte(`{"result":[{"type":"AI_ISSUE_SNS_RELEASE"},{"type":"FOMC_LIVE","enabled":false},{"type":"REASONING_SUBSCRIPTION","enabled":false}]}`)); err == nil {
			t.Fatal("notification-settings probe accepted missing enabled boolean")
		}
		return
	}
	t.Fatal("notification-settings shared probe missing")
}

func newRequestPath(rawURL string) string {
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	return req.URL.Path
}
