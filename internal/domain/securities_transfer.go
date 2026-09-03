package domain

import "time"

// SecuritiesTransferAccount is an account available in the Securities stock
// transfer flow. AccountID is present only on the user's own accounts.
type SecuritiesTransferAccount struct {
	BankCode  string `json:"bank_code"`
	AccountNo string `json:"account_no"`
	AccountID string `json:"account_id,omitempty"`
}

// SecuritiesTransferAccounts separates the user's own source accounts from
// recent destination accounts so callers cannot mistake one for the other.
type SecuritiesTransferAccounts struct {
	OwnAccounts    []SecuritiesTransferAccount `json:"own_accounts"`
	RecentAccounts []SecuritiesTransferAccount `json:"recent_accounts"`
	FetchedAt      time.Time                   `json:"fetched_at"`
}
