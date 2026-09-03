package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// GetNotificationStatus summarizes the generic preference list alongside the
// two status signals that are not part of that list.
func (c *Client) GetNotificationStatus(ctx context.Context) (domain.NotificationStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.NotificationStatus{}, err
	}
	var inbox quoteEnvelope[struct {
		Unread *bool `json:"unread"`
	}]
	var settings domain.NotificationSettings
	var reasoningAgreement quoteEnvelope[*bool]
	var reasoningNewsCount quoteEnvelope[*int]
	if err := runReadBatch(
		readTask{label: "notification settings", run: func() error {
			var err error
			settings, err = c.GetNotificationSettings(ctx)
			return err
		}},
		readTask{label: "inbox unread status", run: func() error {
			if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/inbox-alimies/has-unread", &inbox); err != nil {
				return err
			}
			if inbox.Result.Unread == nil {
				return fmt.Errorf("response missing result.unread boolean")
			}
			return nil
		}},
		readTask{label: "reasoning agreement", run: func() error {
			if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/reasoning/agreement", &reasoningAgreement); err != nil {
				return err
			}
			if reasoningAgreement.Result == nil {
				return fmt.Errorf("response missing result boolean")
			}
			return nil
		}},
		readTask{label: "reasoning news count", run: func() error {
			if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/reasoning-news/count", &reasoningNewsCount); err != nil {
				return err
			}
			if reasoningNewsCount.Result == nil {
				return fmt.Errorf("response missing numeric result")
			}
			return nil
		}},
	); err != nil {
		return domain.NotificationStatus{}, err
	}

	result := domain.NotificationStatus{
		InboxUnread:        *inbox.Result.Unread,
		ReasoningAgreement: *reasoningAgreement.Result,
		ReasoningNewsCount: *reasoningNewsCount.Result,
		FetchedAt:          time.Now().UTC(),
	}
	seen := map[string]bool{}
	for _, setting := range settings.Settings {
		switch setting.Type {
		case "AI_ISSUE_SNS_RELEASE":
			result.AIIssueSNSReleaseAlertEnabled = setting.Enabled
			seen[setting.Type] = true
		case "FOMC_LIVE":
			result.FOMCLiveAlertEnabled = setting.Enabled
			seen[setting.Type] = true
		case "REASONING_SUBSCRIPTION":
			result.ReasoningContentsAlertEnabled = setting.Enabled
			seen[setting.Type] = true
		}
	}
	var missing []string
	for _, required := range []string{"AI_ISSUE_SNS_RELEASE", "FOMC_LIVE", "REASONING_SUBSCRIPTION"} {
		if !seen[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return domain.NotificationStatus{}, fmt.Errorf("notification settings response missing required types: %s", strings.Join(missing, ", "))
	}
	return result, nil
}
