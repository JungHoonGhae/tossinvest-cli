package client

import (
	"context"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// GetAccountAccessStatus returns the user-global last-login context alongside
// verified read-only risk signals for one Securities account. An empty account
// key selects the primary account for the account-scoped signals.
func (c *Client) GetAccountAccessStatus(ctx context.Context, accountKey string) (domain.AccountAccessStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccountAccessStatus{}, err
	}
	resolvedAccountKey, err := c.resolveAccountKey(ctx, accountKey)
	if err != nil {
		return domain.AccountAccessStatus{}, err
	}

	var lastLogin quoteEnvelope[struct {
		Channel   string `json:"channel"`
		OSName    string `json:"osName"`
		AgentName string `json:"agentName"`
		Timestamp string `json:"timestamp"`
	}]
	var margin quoteEnvelope[struct {
		Frozen    bool    `json:"isFrozen"`
		StartDate *string `json:"startDate"`
		EndDate   *string `json:"endDate"`
	}]
	var accidentCount quoteEnvelope[int]
	if err := runReadBatch(
		readTask{label: "last login info", run: func() error {
			return c.getJSON(ctx, c.apiBaseURL+"/api/v1/user/last-login-info", &lastLogin)
		}},
		readTask{label: "margin freeze status", run: func() error {
			return c.getJSONWithAccountKey(ctx, c.certBaseURL+"/api/v1/margin/cert/frozen-account", resolvedAccountKey, &margin)
		}},
		readTask{label: "accident account count", run: func() error {
			return c.getJSONWithAccountKey(ctx, c.apiBaseURL+"/api/v2/account/unlock/accident-account/count", resolvedAccountKey, &accidentCount)
		}},
	); err != nil {
		return domain.AccountAccessStatus{}, err
	}

	return domain.AccountAccessStatus{
		AccountScope: c.accountScope(resolvedAccountKey),
		LastLogin: domain.AccountLastLogin{
			Channel:   lastLogin.Result.Channel,
			OSName:    lastLogin.Result.OSName,
			AgentName: lastLogin.Result.AgentName,
			Timestamp: lastLogin.Result.Timestamp,
		},
		Margin: domain.AccountMarginRestriction{
			Frozen:    margin.Result.Frozen,
			StartDate: optionalStringValue(margin.Result.StartDate),
			EndDate:   optionalStringValue(margin.Result.EndDate),
		},
		AccidentAccountCount: accidentCount.Result,
		FetchedAt:            time.Now().UTC(),
	}, nil
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
