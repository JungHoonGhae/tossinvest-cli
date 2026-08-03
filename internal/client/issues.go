package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Market issue board (렌즈 이슈) — GET /api/v1/lens/issues on the info host.
//
// The feed clusters news into topics and ranks them. Unlike `market news`
// (flat headlines) and `market briefing` (AI category grouping), the ranking
// itself is the payload: what the market is talking about most, and which way
// each topic is moving.
//
// No parameters: the endpoint always returns the current board.

type issueRaw struct {
	Rank        int    `json:"rank"`
	RankStatus  string `json:"rankStatus"`
	Topic       string `json:"topic"`
	TopicTitle  string `json:"topicTitle"`
	SourceCount int    `json:"sourceCount"`
	IssueCat    string `json:"issueCategory"`
	Sources     []struct {
		SourceName string `json:"sourceName"`
		Title      string `json:"title"`
		CreatedAt  string `json:"createdAt"`
	} `json:"sources"`
}

type issuesRaw struct {
	Issues    []issueRaw `json:"issues"`
	UpdatedAt string     `json:"updatedAt"`
}

// GetMarketIssues returns the ranked issue board. WTS-only.
func (c *Client) GetMarketIssues(ctx context.Context) (domain.MarketIssues, error) {
	if err := c.requireSession(); err != nil {
		return domain.MarketIssues{}, err
	}
	var env quoteEnvelope[issuesRaw]
	if err := c.getJSON(ctx, c.infoBaseURL+"/api/v1/lens/issues", &env); err != nil {
		return domain.MarketIssues{}, err
	}
	out := domain.MarketIssues{
		Issues:    make([]domain.MarketIssue, 0, len(env.Result.Issues)),
		UpdatedAt: env.Result.UpdatedAt,
		FetchedAt: time.Now(),
	}
	for _, r := range env.Result.Issues {
		issue := domain.MarketIssue{
			Rank: r.Rank, RankStatus: r.RankStatus, Topic: r.Topic,
			Title: r.TopicTitle, Category: r.IssueCat, SourceCount: r.SourceCount,
		}
		for _, s := range r.Sources {
			issue.Sources = append(issue.Sources, domain.IssueSource{
				Name: s.SourceName, Title: s.Title, CreatedAt: s.CreatedAt,
			})
		}
		out.Issues = append(out.Issues, issue)
	}
	return out, nil
}
