package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteNotificationStatusKeepsAccurateDeprecatedFieldWithoutPresentingItAsActionable(t *testing.T) {
	t.Parallel()
	status := domain.NotificationStatus{
		InboxUnread:                   true,
		AIIssueSNSReleaseAlertEnabled: true,
		FOMCLiveAlertEnabled:          false,
		ReasoningContentsAlertEnabled: true,
		ReasoningAgreement:            true,
		ReasoningNewsCount:            7,
	}
	var table bytes.Buffer
	if err := WriteNotificationStatus(&table, FormatTable, status); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Inbox unread:", "AI issue SNS release alert:", "FOMC live alert:",
		"Reasoning contents alert:", "Reasoning agreement:",
	} {
		if !strings.Contains(table.String(), want) {
			t.Fatalf("table missing %q: %s", want, table.String())
		}
	}
	if strings.Contains(table.String(), "Reasoning news count:") {
		t.Fatalf("table retained non-notification corpus count: %s", table.String())
	}

	var jsonOut bytes.Buffer
	if err := WriteNotificationStatus(&jsonOut, FormatJSON, status); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	wantJSON := map[string]any{
		"inbox_unread": true, "ai_issue_sns_release_alert_enabled": true,
		"fomc_live_alert_enabled": false, "reasoning_contents_alert_enabled": true,
		"reasoning_agreement": true,
	}
	for key, want := range wantJSON {
		if got, ok := decoded[key]; !ok || got != want {
			t.Errorf("JSON %s = %#v, want %#v", key, got, want)
		}
	}
	if got, ok := decoded["reasoning_news_count"]; !ok || got != float64(7) {
		t.Fatalf("JSON compatibility field = %#v, present=%t", got, ok)
	}

	var csvOut bytes.Buffer
	if err := WriteNotificationStatus(&csvOut, FormatCSV, status); err != nil {
		t.Fatal(err)
	}
	wantCSV := "inbox_unread,ai_issue_sns_release_alert_enabled,fomc_live_alert_enabled,reasoning_contents_alert_enabled,reasoning_agreement,reasoning_news_count\ntrue,true,false,true,true,7"
	if got := strings.TrimSpace(csvOut.String()); got != wantCSV {
		t.Fatalf("CSV = %q, want %q", got, wantCSV)
	}
}
