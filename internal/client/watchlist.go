package client

import (
	"context"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// ListWatchlist returns all watchlist items across all folders.
// This is a convenience wrapper that delegates to ListAllWatchlistItems,
// which uses the new-watchlists API (GET /api/v1/new-watchlists).
//
// The previous implementation used the legacy POST /api/v2/dashboard/asset/sections/all
// endpoint which no longer returns watchlist data.
func (c *Client) ListWatchlist(ctx context.Context) ([]domain.WatchlistItem, error) {
	return c.ListAllWatchlistItems(ctx)
}
