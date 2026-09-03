package domain

import "time"

// NotificationStatus collects specialized read-only notification and AI
// content signals that are not present in the generic notification list.
type NotificationStatus struct {
	InboxUnread                   bool      `json:"inbox_unread"`
	AIIssueSNSReleaseAlertEnabled bool      `json:"ai_issue_sns_release_alert_enabled"`
	FOMCLiveAlertEnabled          bool      `json:"fomc_live_alert_enabled"`
	ReasoningContentsAlertEnabled bool      `json:"reasoning_contents_alert_enabled"`
	ReasoningAgreement            bool      `json:"reasoning_agreement"`
	ReasoningNewsCount            int       `json:"reasoning_news_count"`
	FetchedAt                     time.Time `json:"fetched_at"`
}
