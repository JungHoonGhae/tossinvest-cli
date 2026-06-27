package official

import (
	"context"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiAccount is the official API shape for one account entry.
// Endpoint: GET /api/v1/accounts
// Schema (openapi.latest.json component "Account"):
//
//	accountNo   string  — human account number (e.g. "123-45")
//	accountSeq  integer — identifier used in subsequent API calls
//	accountType string  — enum: BROKERAGE | ...
type apiAccount struct {
	AccountNo   string `json:"accountNo"`
	AccountSeq  int    `json:"accountSeq"`
	AccountType string `json:"accountType"`
}

// Accounts fetches the authenticated user's account list from the official API
// and adapts it to []domain.Account.
func (c *Client) Accounts(ctx context.Context) ([]domain.Account, error) {
	var raw []apiAccount
	if err := c.get(ctx, "/api/v1/accounts", nil, &raw); err != nil {
		return nil, err
	}
	return adaptAccounts(raw), nil
}

// adaptAccounts converts a slice of official API account records to the
// canonical domain representation. It is a pure function (no I/O).
//
// Mapping rationale (cross-referenced with internal/client/account.go WTS adapter):
//
//   - accountSeq (int) → ID (string):
//     WTS uses item.Key as the stable account identifier passed to subsequent
//     calls. The official equivalent is accountSeq (an integer key). We
//     convert int→string to match the domain.Account.ID type.
//
//   - accountNo (string) → DisplayName:
//     WTS puts item.DisplayName in domain.Account.DisplayName. The official
//     accountNo is the human-readable account number and is the closest
//     analogue to the displayed identity.
//
//   - accountType (string enum) → Type:
//     WTS copies item.Type verbatim. We do the same with the official enum
//     value (e.g. "BROKERAGE") to avoid an unnecessary translation layer.
//
//   - Name, Markets, Currency, Primary: not present in the official accounts
//     endpoint; left as zero values. Currency and Primary can be enriched by
//     callers if needed.
func adaptAccounts(raw []apiAccount) []domain.Account {
	out := make([]domain.Account, 0, len(raw))
	for _, a := range raw {
		out = append(out, domain.Account{
			ID:          strconv.Itoa(a.AccountSeq), // official accountSeq → stable call-time key
			DisplayName: a.AccountNo,                // official accountNo  → human display value
			Type:        a.AccountType,              // official accountType → raw enum (e.g. BROKERAGE)
			// Name     — not available from official API
			// Markets  — not available from official API
			// Currency — not available from official API
			// Primary  — not available from official API (no primaryKey concept in this endpoint)
		})
	}
	return out
}
