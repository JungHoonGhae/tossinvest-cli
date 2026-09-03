package domain

import "time"

// SecuritiesTransferAccount is an account available in the Securities stock
// transfer flow. AccountID is retained for internal contract fidelity but is
// never serialized; it is not meaningful without a future separately gated
// transfer workflow.
type SecuritiesTransferAccount struct {
	BankCode  string `json:"bank_code"`
	AccountNo string `json:"account_no"`
	AccountID string `json:"-"`
}

// SecuritiesTransferAccounts separates the user's own source accounts from
// recent destination accounts so callers cannot mistake one for the other.
// AccountScope is a session-bound opaque label, not the account key sent to WTS.
type SecuritiesTransferAccounts struct {
	AccountScope   string                      `json:"account_scope"`
	OwnAccounts    []SecuritiesTransferAccount `json:"own_accounts"`
	RecentAccounts []SecuritiesTransferAccount `json:"recent_accounts"`
	FetchedAt      time.Time                   `json:"fetched_at"`
}
