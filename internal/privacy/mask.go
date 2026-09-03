// Package privacy owns presentation-independent redaction rules for sensitive
// account identity data. CLI renderers and machine-facing ops/MCP adapters use
// the same functions so choosing JSON cannot silently bypass masking.
package privacy

import (
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// AccountNumber keeps only the last three identifying characters. Short or
// malformed values remain fully private; hyphen separators remain visible.
func AccountNumber(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	identifying := 0
	for _, ch := range runes {
		if ch != '-' {
			identifying++
		}
	}
	reveal := 3
	if identifying <= reveal {
		reveal = 0
	}
	hide := identifying - reveal
	masked := make([]rune, 0, len(runes))
	seen := 0
	for _, ch := range runes {
		if ch == '-' {
			masked = append(masked, ch)
			continue
		}
		if seen < hide {
			masked = append(masked, '*')
		} else {
			masked = append(masked, ch)
		}
		seen++
	}
	return string(masked)
}

// Name keeps the first rune and hides the remainder. A one-rune name is fully
// hidden because revealing it would reveal the whole value.
func Name(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}

func RedactAccountOverview(value domain.AccountOverview) domain.AccountOverview {
	out := value
	out.Accounts = redactOverviewItems(value.Accounts)
	out.MinorAccounts = redactOverviewItems(value.MinorAccounts)
	return out
}

func redactOverviewItems(items []domain.AccountOverviewItem) []domain.AccountOverviewItem {
	out := make([]domain.AccountOverviewItem, len(items))
	copy(out, items)
	for i := range out {
		out[i].AccountNo = AccountNumber(out[i].AccountNo)
	}
	return out
}

func RedactOpenBankingStatus(value domain.OpenBankingStatus) domain.OpenBankingStatus {
	out := value
	out.HolderName = Name(value.HolderName)
	if value.ConnectedAccount != nil {
		account := *value.ConnectedAccount
		account.AccountNo = AccountNumber(account.AccountNo)
		out.ConnectedAccount = &account
	}
	return out
}

func RedactSecuritiesTransferAccounts(value domain.SecuritiesTransferAccounts) domain.SecuritiesTransferAccounts {
	out := value
	out.OwnAccounts = redactSecuritiesTransferAccountItems(value.OwnAccounts)
	out.RecentAccounts = redactSecuritiesTransferAccountItems(value.RecentAccounts)
	return out
}

func redactSecuritiesTransferAccountItems(items []domain.SecuritiesTransferAccount) []domain.SecuritiesTransferAccount {
	out := make([]domain.SecuritiesTransferAccount, len(items))
	copy(out, items)
	for i := range out {
		out[i].AccountNo = AccountNumber(out[i].AccountNo)
		out[i].AccountID = ""
	}
	return out
}
