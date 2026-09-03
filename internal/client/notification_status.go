package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// GetNotificationStatus returns specialized read-only notification and AI
// content signals whose contracts are separate from the generic preference list.
func (c *Client) GetNotificationStatus(ctx context.Context) (domain.NotificationStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.NotificationStatus{}, err
	}
	var inbox quoteEnvelope[struct {
		Unread bool `json:"unread"`
	}]
	var aiIssue quoteEnvelope[struct {
		Enabled bool `json:"enabled"`
	}]
	var fomcLive quoteEnvelope[struct {
		Enabled bool `json:"enabled"`
	}]
	var reasoningContents quoteEnvelope[struct {
		Enabled bool `json:"enabled"`
	}]
	var reasoningAgreement quoteEnvelope[bool]
	var reasoningNewsCount quoteEnvelope[int]
	if err := runReadBatch(
		readTask{label: "inbox unread status", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/inbox-alimies/has-unread", &inbox)
		}},
		readTask{label: "AI issue social-release alert", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/ai-issue/sns-release/alimy", &aiIssue)
		}},
		readTask{label: "FOMC live alert", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/fomc-live/alimy", &fomcLive)
		}},
		readTask{label: "reasoning contents alert", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/reasoning-contents/alimy/subscription", &reasoningContents)
		}},
		readTask{label: "reasoning agreement", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/reasoning/agreement", &reasoningAgreement)
		}},
		readTask{label: "reasoning news count", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/reasoning-news/count", &reasoningNewsCount)
		}},
	); err != nil {
		return domain.NotificationStatus{}, err
	}

	return domain.NotificationStatus{
		InboxUnread:                   inbox.Result.Unread,
		AIIssueSNSReleaseAlertEnabled: aiIssue.Result.Enabled,
		FOMCLiveAlertEnabled:          fomcLive.Result.Enabled,
		ReasoningContentsAlertEnabled: reasoningContents.Result.Enabled,
		ReasoningAgreement:            reasoningAgreement.Result,
		ReasoningNewsCount:            reasoningNewsCount.Result,
		FetchedAt:                     time.Now().UTC(),
	}, nil
}
