package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// new-watchlists is the folder-aware watchlist API (관심종목 폴더 + 종목).
// 공식 API 에 없는 web 전용 표면이며, 비금융 mutation 이라 거래 권한 게이트와
// 별개로 동작한다 (가벼운 scope).
const watchlistBase = "/api/v1/new-watchlists"

type newWatchlistEnvelope struct {
	Result struct {
		MaxWatchlistCount int `json:"maxWatchlistCount"`
		Watchlists        []struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			ItemCount int    `json:"itemCount"`
			Items     []struct {
				Code     string `json:"code"`
				Name     string `json:"name"`
				ItemType string `json:"itemType"`
				Prices   struct {
					Base  *float64 `json:"base"`
					Close *float64 `json:"close"`
				} `json:"prices"`
			} `json:"items"`
		} `json:"watchlists"`
	} `json:"result"`
}

// ListWatchlistGroups returns watchlist folders with their items (관심종목 폴더 목록).
func (c *Client) ListWatchlistGroups(ctx context.Context) ([]domain.WatchlistGroup, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env newWatchlistEnvelope
	url := c.certBaseURL + watchlistBase + "?includePrice=true&lazyLoad=false"
	if err := c.getJSON(ctx, url, &env); err != nil {
		return nil, err
	}
	out := make([]domain.WatchlistGroup, 0, len(env.Result.Watchlists))
	for _, g := range env.Result.Watchlists {
		grp := domain.WatchlistGroup{ID: g.ID, Name: g.Name, Type: g.Type, ItemCount: g.ItemCount}
		for _, it := range g.Items {
			grp.Items = append(grp.Items, domain.WatchlistItem{
				Group: g.Name, Symbol: it.Code, Name: it.Name,
				Base: coalesceMoney(it.Prices.Base), Last: coalesceMoney(it.Prices.Close),
			})
		}
		out = append(out, grp)
	}
	return out, nil
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

type watchlistItemSpec struct {
	Code     string `json:"code"`
	ItemType string `json:"itemType"`
}

type addWatchlistItemsRequest struct {
	WatchlistIDs []int64             `json:"watchlistIds"`
	Items        []watchlistItemSpec `json:"items"`
}

type removeWatchlistItemsRequest struct {
	WatchlistID int64               `json:"watchlistId"`
	Items       []watchlistItemSpec `json:"items"`
}

// AddWatchlistItem adds a stock to a folder (symbol or product code).
func (c *Client) AddWatchlistItem(ctx context.Context, groupID int64, symbol string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	code, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(addWatchlistItemsRequest{
		WatchlistIDs: []int64{groupID},
		Items:        []watchlistItemSpec{{Code: code, ItemType: "STOCK"}},
	})
	return c.mutateJSON(ctx, http.MethodPost, c.certBaseURL+watchlistBase+"/items", body, nil)
}

// RemoveWatchlistItem removes a stock from a folder.
func (c *Client) RemoveWatchlistItem(ctx context.Context, groupID int64, symbol string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	code, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(removeWatchlistItemsRequest{
		WatchlistID: groupID,
		Items:       []watchlistItemSpec{{Code: code, ItemType: "STOCK"}},
	})
	return c.mutateJSON(ctx, http.MethodPost, c.certBaseURL+watchlistBase+"/items/remove", body, nil)
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
