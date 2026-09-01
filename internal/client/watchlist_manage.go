package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// new-watchlists is the folder-aware watchlist API (관심종목 폴더 + 종목).
// 공식 API 에 없는 web 전용 표면이며, 비금융 mutation 이라 거래 권한 게이트와
// 별개로 동작한다 (가벼운 scope).
const (
	watchlistBase            = "/api/v1/new-watchlists"
	watchlistItemsPath       = watchlistBase + "/items"
	watchlistItemsRemovePath = watchlistBase + "/items/remove"
)

type newWatchlistEnvelope struct {
	Result struct {
		MaxWatchlistCount int `json:"maxWatchlistCount"`
		Watchlists        []struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Ordering  int    `json:"ordering"`
			Type      string `json:"type"`
			ItemCount int    `json:"itemCount"`
			Items     []struct {
				Code     string `json:"code"`
				Symbol   string `json:"symbol"`
				Name     string `json:"name"`
				ItemType string `json:"itemType"`
				Ordering int    `json:"ordering"`
				Prices   struct {
					Base     *float64 `json:"base"`
					Close    *float64 `json:"close"`
					Currency string   `json:"currency"`
				} `json:"prices"`
			} `json:"items"`
		} `json:"watchlists"`
	} `json:"result"`
}

// ListWatchlistGroups returns watchlist folders (관심종목 폴더 목록).
// Uses GET /api/v1/new-watchlists/groups/simple?includeItemInfo=true for lightweight
// metadata with item counts. Without includeItemInfo=true, the itemCount field is omitted.
func (c *Client) ListWatchlistGroups(ctx context.Context) ([]domain.WatchlistGroup, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env newWatchlistEnvelope
	url := c.certBaseURL + watchlistBase + "/groups/simple?includeItemInfo=true"
	if err := c.getJSON(ctx, url, &env); err != nil {
		return nil, err
	}
	return mapWatchlistGroups(env), nil
}

// GetWatchlistGroupItems returns items for a specific folder (per-group lazy loading).
// Uses GET /api/v1/new-watchlists/groups?ids={id}&includePrice=true — the same
// endpoint the web frontend calls when a user selects a folder.
func (c *Client) GetWatchlistGroupItems(ctx context.Context, groupID int64) ([]domain.WatchlistItem, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env newWatchlistEnvelope
	url := fmt.Sprintf("%s%s/groups?ids=%d&includePrice=true", c.certBaseURL, watchlistBase, groupID)
	if err := c.getJSON(ctx, url, &env); err != nil {
		return nil, err
	}
	groups := mapWatchlistGroups(env)
	for _, g := range groups {
		if g.ID == groupID {
			return g.Items, nil
		}
	}
	return nil, fmt.Errorf("watchlist folder %d not found", groupID)
}

// ListAllWatchlistItems returns all watchlist items from every folder, flattened
// and ordered by group ordering then item ordering.
// Uses the bulk endpoint GET /api/v1/new-watchlists?includePrice=true&lazyLoad=false.
func (c *Client) ListAllWatchlistItems(ctx context.Context) ([]domain.WatchlistItem, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env newWatchlistEnvelope
	url := c.certBaseURL + watchlistBase + "?includePrice=true&lazyLoad=false"
	if err := c.getJSON(ctx, url, &env); err != nil {
		return nil, err
	}
	groups := mapWatchlistGroups(env)
	items := make([]domain.WatchlistItem, 0)
	for _, g := range groups {
		items = append(items, g.Items...)
	}
	return items, nil
}

// mapWatchlistGroups converts the raw API envelope into domain objects,
// fixing symbol/currency mapping and enforcing ordering.
func mapWatchlistGroups(env newWatchlistEnvelope) []domain.WatchlistGroup {
	out := make([]domain.WatchlistGroup, 0, len(env.Result.Watchlists))
	for _, g := range env.Result.Watchlists {
		grp := domain.WatchlistGroup{
			ID: g.ID, Name: g.Name, Ordering: g.Ordering,
			Type: g.Type, ItemCount: g.ItemCount,
			Items: make([]domain.WatchlistItem, 0, len(g.Items)),
		}
		// Sort items by ordering before mapping.
		sort.SliceStable(g.Items, func(i, j int) bool {
			return g.Items[i].Ordering < g.Items[j].Ordering
		})
		for _, it := range g.Items {
			sym := it.Symbol
			if sym == "" {
				sym = it.Code
			}
			grp.Items = append(grp.Items, domain.WatchlistItem{
				Group: g.Name, Symbol: sym, Name: it.Name,
				Currency: it.Prices.Currency,
				Base:     coalesceMoney(it.Prices.Base),
				Last:     coalesceMoney(it.Prices.Close),
			})
		}
		out = append(out, grp)
	}
	// Sort groups by ordering.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Ordering < out[j].Ordering
	})
	return out
}

// CreateWatchlistGroup creates a new folder and returns it.
func (c *Client) CreateWatchlistGroup(ctx context.Context, name string) (domain.WatchlistGroup, error) {
	if err := c.requireSession(); err != nil {
		return domain.WatchlistGroup{}, err
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	var env struct {
		Result struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			ItemCount int    `json:"itemCount"`
		} `json:"result"`
	}
	if err := c.mutateJSON(ctx, http.MethodPost, c.certBaseURL+watchlistBase+"/groups", body, &env); err != nil {
		return domain.WatchlistGroup{}, err
	}
	return domain.WatchlistGroup{ID: env.Result.ID, Name: env.Result.Name, Type: env.Result.Type, ItemCount: env.Result.ItemCount}, nil
}

// RenameWatchlistGroup renames a folder.
func (c *Client) RenameWatchlistGroup(ctx context.Context, groupID int64, name string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	url := fmt.Sprintf("%s%s/groups/%d", c.certBaseURL, watchlistBase, groupID)
	return c.mutateJSON(ctx, http.MethodPatch, url, body, nil)
}

// DeleteWatchlistGroup deletes a folder.
func (c *Client) DeleteWatchlistGroup(ctx context.Context, groupID int64) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s/groups/%d", c.certBaseURL, watchlistBase, groupID)
	return c.mutateJSON(ctx, http.MethodDelete, url, nil, nil)
}

// AddWatchlistItem adds a stock to a folder (symbol or product code).
func (c *Client) AddWatchlistItem(ctx context.Context, groupID int64, symbol string) error {
	return c.watchlistItemOp(ctx, c.certBaseURL+watchlistItemsPath, groupID, symbol)
}

// RemoveWatchlistItem removes a stock from a folder.
func (c *Client) RemoveWatchlistItem(ctx context.Context, groupID int64, symbol string) error {
	return c.watchlistItemOp(ctx, c.certBaseURL+watchlistItemsRemovePath, groupID, symbol)
}

func (c *Client) watchlistItemOp(ctx context.Context, endpoint string, groupID int64, symbol string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	code, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"watchlistId": groupID,
		"items":       []map[string]string{{"code": code, "itemType": "STOCK"}},
	})
	return c.mutateJSON(ctx, http.MethodPost, endpoint, body, nil)
}

// mutateJSON issues a non-GET request with session auth (X-XSRF-TOKEN included
// via applySession) and optionally decodes the response.
func (c *Client) mutateJSON(ctx context.Context, method, endpoint string, body []byte, target any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	c.applySession(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, data)
	}
	if target != nil && len(data) > 0 {
		return json.Unmarshal(data, target)
	}
	return nil
}
