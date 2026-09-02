package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const accountOverviewPath = "/api/v1/dashboard/all-accounts"

type accountOverviewEnvelope struct {
	Result []struct {
		Data *struct {
			AccountOverviews      []accountOverviewDTO `json:"accountOverviews"`
			MinorAccountOverviews []accountOverviewDTO `json:"minorAccountOverviews"`
			TotalAssetAmount      int64                `json:"totalAssetAmount"`
		} `json:"data"`
		Error json.RawMessage `json:"error"`
	} `json:"result"`
}

type accountOverviewDTO struct {
	AccountName       string `json:"accountName"`
	AccountNo         string `json:"accountNo"`
	PendingOrderCount int    `json:"pendingOrderCount"`
	TotalAssetAmount  int64  `json:"totalAssetAmount"`
}

// GetAccountOverview returns the mobile app's all-account asset rollup.
//
// Android 5.275.0 statically verifies the complete request contract: the
// information API host, POST path, default SUMMARY_WITH_MINOR section, and the
// result[0].data response model. Keep this separate from GetAccountSummary,
// whose endpoint describes only the selected account.
func (c *Client) GetAccountOverview(ctx context.Context) (domain.AccountOverview, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccountOverview{}, err
	}

	var envelope accountOverviewEnvelope
	body := json.RawMessage(`{"sections":["SUMMARY_WITH_MINOR"]}`)
	if err := c.postJSON(ctx, c.infoBaseURL+accountOverviewPath, body, &envelope); err != nil {
		return domain.AccountOverview{}, err
	}
	if len(envelope.Result) == 0 {
		return domain.AccountOverview{}, fmt.Errorf("account overview result is empty")
	}
	if envelope.Result[0].Data == nil {
		return domain.AccountOverview{}, fmt.Errorf("account overview data is missing")
	}

	data := envelope.Result[0].Data
	return domain.AccountOverview{
		Accounts:         mapAccountOverviewItems(data.AccountOverviews),
		MinorAccounts:    mapAccountOverviewItems(data.MinorAccountOverviews),
		TotalAssetAmount: data.TotalAssetAmount,
	}, nil
}

func mapAccountOverviewItems(items []accountOverviewDTO) []domain.AccountOverviewItem {
	result := make([]domain.AccountOverviewItem, 0, len(items))
	for _, item := range items {
		result = append(result, domain.AccountOverviewItem{
			AccountName:       item.AccountName,
			AccountNo:         item.AccountNo,
			PendingOrderCount: item.PendingOrderCount,
			TotalAssetAmount:  item.TotalAssetAmount,
		})
	}
	return result
}
