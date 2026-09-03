package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	securitiesTransferMyAccountsPath     = "/api/v1/securities-transfer/my-accounts"
	securitiesTransferRecentAccountsPath = "/api/v1/securities-transfer/recent-accounts"
)

type securitiesTransferAccountRaw struct {
	BankCode  string `json:"bankCode"`
	AccountNo string `json:"accountNo"`
	AccountID string `json:"accountId"`
}

// GetSecuritiesTransferAccounts returns read-only account choices from the
// Securities stock-transfer flow. It does not initiate a transfer.
func (c *Client) GetSecuritiesTransferAccounts(ctx context.Context) (domain.SecuritiesTransferAccounts, error) {
	if err := c.requireSession(); err != nil {
		return domain.SecuritiesTransferAccounts{}, err
	}
	accountKey, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.SecuritiesTransferAccounts{}, err
	}

	var own quoteEnvelope[[]securitiesTransferAccountRaw]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+securitiesTransferMyAccountsPath, accountKey, &own); err != nil {
		return domain.SecuritiesTransferAccounts{}, fmt.Errorf("securities transfer own accounts: %w", err)
	}
	var recent quoteEnvelope[[]securitiesTransferAccountRaw]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+securitiesTransferRecentAccountsPath, accountKey, &recent); err != nil {
		return domain.SecuritiesTransferAccounts{}, fmt.Errorf("securities transfer recent accounts: %w", err)
	}

	out := domain.SecuritiesTransferAccounts{
		OwnAccounts:    make([]domain.SecuritiesTransferAccount, 0, len(own.Result)),
		RecentAccounts: make([]domain.SecuritiesTransferAccount, 0, len(recent.Result)),
		FetchedAt:      time.Now().UTC(),
	}
	for _, item := range own.Result {
		out.OwnAccounts = append(out.OwnAccounts, domain.SecuritiesTransferAccount{
			BankCode: item.BankCode, AccountNo: item.AccountNo, AccountID: item.AccountID,
		})
	}
	for _, item := range recent.Result {
		out.RecentAccounts = append(out.RecentAccounts, domain.SecuritiesTransferAccount{
			BankCode: item.BankCode, AccountNo: item.AccountNo,
		})
	}
	return out, nil
}
