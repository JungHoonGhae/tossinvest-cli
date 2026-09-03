package domain

import "time"

// NotificationStatus combines canonical notification preferences with the
// inbox and AI-agreement signals that are not part of that preference list.
type NotificationStatus struct {
	InboxUnread                   bool `json:"inbox_unread"`
	AIIssueSNSReleaseAlertEnabled bool `json:"ai_issue_sns_release_alert_enabled"`
	FOMCLiveAlertEnabled          bool `json:"fomc_live_alert_enabled"`
	ReasoningContentsAlertEnabled bool `json:"reasoning_contents_alert_enabled"`
	ReasoningAgreement            bool `json:"reasoning_agreement"`
	// ReasoningNewsCount is retained for output compatibility. It is a global
	// content count, not a signed-in account's notification preference.
	// Deprecated: use the preference booleans for account notification state.
	ReasoningNewsCount int       `json:"reasoning_news_count"`
	FetchedAt          time.Time `json:"fetched_at"`
}
