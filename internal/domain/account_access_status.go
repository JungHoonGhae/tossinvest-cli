package domain

import "time"

// AccountLastLogin describes the most recent Toss Securities login context.
type AccountLastLogin struct {
	Channel   string `json:"channel,omitempty"`
	OSName    string `json:"os_name,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// AccountMarginRestriction reports whether margin trading is currently
// frozen for the selected Securities account.
type AccountMarginRestriction struct {
	Frozen    bool   `json:"frozen"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// AccountAccessStatus combines the user-global last-login context with
// read-only risk signals for one Securities account. AccountScope is opaque
// and session-bound.
type AccountAccessStatus struct {
	AccountScope         string                   `json:"account_scope"`
	LastLogin            AccountLastLogin         `json:"last_login"`
	Margin               AccountMarginRestriction `json:"margin_restriction"`
	AccidentAccountCount int                      `json:"accident_account_count"`
	FetchedAt            time.Time                `json:"fetched_at"`
}
