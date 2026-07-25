package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Market news feed (피드 → 뉴스). One endpoint, one parameter: the scope.
//
// Measured against the live service:
//   - `size` is honoured but capped at 50; `page`, `pagingParam` and `after`
//     are ignored (the first item never changes), so there is no pagination —
//     only the newest ≤50 items are reachable.
//   - there is no search: query / keyword / q / searchWord are all ignored.
//   - the response carries its own `title` for the scope, which we surface
//     instead of hardcoding Korean labels.

// MaxNewsLimit is the server's ceiling; asking for more silently returns 50.
const MaxNewsLimit = 50

// newsScopes maps friendly aliases to the server enum, following the
// communityRankingTypes precedent. The server names are a poor CLI vocabulary —
// PERSONALIZE_HOLD is opaque, and HOT is actively misleading (it is the
// "최신 뉴스" tab, not the trending one; trending is SOARING_STOCK).
var newsScopes = map[string]string{
	"all":         "ALL_HIGHLIGHT",
	"recommended": "PERSONALIZED",
	"watchlist":   "PERSONALIZE_WATCH",
	"holdings":    "PERSONALIZE_HOLD",
	"latest":      "HOT",
	"soaring":     "SOARING_STOCK",
}

// DefaultNewsScope is used when the caller does not choose one: the widest
// scope, and the only one that does not depend on account context.
const DefaultNewsScope = "all"

// NewsScopeAliases lists the aliases for help text and validation errors.
func NewsScopeAliases() []string {
	out := make([]string, 0, len(newsScopes))
	for k := range newsScopes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// NewsScope resolves an alias (or a raw server enum) to the value the API
// wants. Raw passthrough means a scope Toss adds later is usable without a
// release — the alias table is a convenience, not a gate.
func NewsScope(alias string) (string, error) {
	a := strings.ToLower(strings.TrimSpace(alias))
	if a == "" {
		return newsScopes[DefaultNewsScope], nil
	}
	if v, ok := newsScopes[a]; ok {
		return v, nil
	}
	// Raw enum: accept anything shaped like one and let the server judge — it
	// returns 400 for an unknown scope, which is a clear enough answer.
	upper := strings.ToUpper(strings.TrimSpace(alias))
	if upper == alias && strings.ContainsAny(upper, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return upper, nil
	}
	return "", fmt.Errorf("unknown news scope %q (use: %s — or a raw server enum)",
		alias, strings.Join(NewsScopeAliases(), " | "))
}

type newsRelatedStockRaw struct {
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	Market      string  `json:"market"`
	Fluctuation float64 `json:"fluctuation"`
}

type newsItemRaw struct {
	NewsID        string                `json:"newsId"`
	Title         string                `json:"title"`
	Summary       string                `json:"summary"`
	CreatedAt     string                `json:"createdAt"`
	Source        string                `json:"source"`
	NewsType      string                `json:"newsType"`
	Nation        string                `json:"nation"`
	RelatedStocks []newsRelatedStockRaw `json:"relatedStocks"`
}

type marketNewsRaw struct {
	Type  string        `json:"type"`
	Title string        `json:"title"`
	News  []newsItemRaw `json:"news"`
}

// GetMarketNews fetches one scope of the news feed. scope is a server enum
// (resolve an alias with NewsScope first). limit is clamped to MaxNewsLimit;
// 0 means the server default. WTS-only.
func (c *Client) GetMarketNews(ctx context.Context, scope string, limit int) (domain.MarketNews, error) {
	if err := c.requireSession(); err != nil {
		return domain.MarketNews{}, err
	}
	if scope == "" {
		scope = newsScopes[DefaultNewsScope]
	}
	req := map[string]any{"type": scope}
	if limit > 0 {
		if limit > MaxNewsLimit {
			limit = MaxNewsLimit
		}
		req["size"] = limit
	}
	body, err := json.Marshal(req)
	if err != nil {
		return domain.MarketNews{}, err
	}

	var env quoteEnvelope[marketNewsRaw]
	endpoint := c.infoBaseURL + "/api/v1/dashboard/wts/news"
	if err := c.postJSON(ctx, endpoint, body, &env); err != nil {
		return domain.MarketNews{}, err
	}

	out := domain.MarketNews{
		Type:      env.Result.Type,
		Title:     env.Result.Title,
		FetchedAt: time.Now(),
	}
	if out.Type == "" {
		out.Type = scope
	}
	for _, n := range env.Result.News {
		item := domain.NewsItem{
			ID:        n.NewsID,
			Title:     n.Title,
			Summary:   n.Summary,
			Source:    n.Source,
			Type:      n.NewsType,
			Nation:    n.Nation,
			CreatedAt: n.CreatedAt,
		}
		for _, s := range n.RelatedStocks {
			item.Stocks = append(item.Stocks, domain.NewsRelatedStock{
				Code:        s.StockCode,
				Name:        s.StockName,
				Market:      s.Market,
				Fluctuation: s.Fluctuation,
			})
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}
