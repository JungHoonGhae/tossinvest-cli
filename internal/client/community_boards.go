package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type communityBoardRaw struct {
	ID            int64    `json:"id"`
	SubjectType   string   `json:"subjectType"`
	SubjectID     string   `json:"subjectId"`
	Title         string   `json:"title"`
	About         string   `json:"about"`
	Rules         []string `json:"rules"`
	FollowerCount int      `json:"followerCount"`
	CommentCount  int      `json:"commentCount"`
	IsMember      bool     `json:"isMember"`
	IsManager     bool     `json:"isManager"`
	CreatedAt     string   `json:"createdAt"`
}

type communityBoardsRaw struct {
	Results []communityBoardRaw `json:"results"`
}

// GetPopularBoards returns the community lounges ranked by follower count.
// 공식 Open API 에 없는 web 전용 표면.
//
// The endpoint is named popular-follower and the server returns them already
// ordered, so no sorting is applied here — re-sorting would silently disagree
// with what the app shows.
func (c *Client) GetPopularBoards(ctx context.Context) (domain.CommunityBoards, error) {
	if err := c.requireSession(); err != nil {
		return domain.CommunityBoards{}, err
	}

	var envelope quoteEnvelope[communityBoardsRaw]
	url := c.certBaseURL + "/api/v1/boards/popular-follower"
	if err := c.getJSON(ctx, url, &envelope); err != nil {
		return domain.CommunityBoards{}, err
	}

	out := domain.CommunityBoards{FetchedAt: time.Now().UTC()}
	for _, b := range envelope.Result.Results {
		out.Boards = append(out.Boards, domain.CommunityBoard{
			ID:            b.ID,
			SubjectType:   b.SubjectType,
			SubjectID:     b.SubjectID,
			Title:         b.Title,
			About:         b.About,
			Rules:         b.Rules,
			FollowerCount: b.FollowerCount,
			CommentCount:  b.CommentCount,
			IsMember:      b.IsMember,
			IsManager:     b.IsManager,
			CreatedAt:     b.CreatedAt,
		})
	}
	return out, nil
}
