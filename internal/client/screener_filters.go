package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type filterRangeRaw struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type filterBaseRaw struct {
	BasedAt string `json:"basedAt"`
}

// GetScreenerFilterRanges returns each filter's observed value span and the
// date the data is based on. 공식 Open API 에 없는 web 전용 표면.
//
// The range request body is `{"filter":{"id":…},"nation":…}` — the filter is a
// nested object, NOT a flat `filterId`. The flat shape (which /filters/base
// does use) returns 400, which is why this endpoint sat unresolved: the two
// sibling endpoints disagree on their request shape.
//
// Some filters need conditions this surface cannot express — a period, for
// `주가등락률` and friends. The server rejects those with its own code; it is
// carried through per-filter rather than failing the whole call, because a
// caller asking for ten filters should still get the eight that answered.
func (c *Client) GetScreenerFilterRanges(ctx context.Context, filterIDs []string, nation string) (domain.ScreenerFilterRanges, error) {
	if err := c.requireSession(); err != nil {
		return domain.ScreenerFilterRanges{}, err
	}
	if len(filterIDs) == 0 {
		return domain.ScreenerFilterRanges{}, fmt.Errorf("at least one filter id is required")
	}
	n := strings.ToLower(strings.TrimSpace(nation))
	if n == "" {
		n = "kr"
	}

	out := domain.ScreenerFilterRanges{Nation: n, FetchedAt: time.Now().UTC()}
	for _, id := range filterIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		entry := domain.ScreenerFilterRange{FilterID: id, Nation: n}

		body, err := json.Marshal(map[string]any{
			"filter": map[string]string{"id": id},
			"nation": n,
		})
		if err != nil {
			return domain.ScreenerFilterRanges{}, err
		}
		var rangeEnv quoteEnvelope[filterRangeRaw]
		if err := c.postJSON(ctx, c.certBaseURL+"/api/v1/screener/filters/range", body, &rangeEnv); err != nil {
			entry.Unavailable = serverReason(err)
			out.Filters = append(out.Filters, entry)
			continue
		}
		entry.Min, entry.Max = rangeEnv.Result.Min, rangeEnv.Result.Max

		// basedAt is a separate call and takes the FLAT shape.
		baseBody, err := json.Marshal(map[string]string{"filterId": id, "nation": n})
		if err != nil {
			return domain.ScreenerFilterRanges{}, err
		}
		var baseEnv quoteEnvelope[filterBaseRaw]
		if err := c.postJSON(ctx, c.certBaseURL+"/api/v1/screener/filters/base", baseBody, &baseEnv); err == nil {
			entry.BasedAt = baseEnv.Result.BasedAt
		}
		out.Filters = append(out.Filters, entry)
	}
	return out, nil
}

// serverReason pulls Toss's own error code out of a status error so the caller
// sees `screener.invalid.filter-condition-period` rather than a generic
// failure. Toss ships no mapping for these codes, so it is passed through
// verbatim — translating would be guessing.
func serverReason(err error) string {
	var se *StatusError
	if !errors.As(err, &se) {
		return err.Error()
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(se.Body), &env) == nil {
		if env.Error.Code != "" {
			return env.Error.Code
		}
		if env.Error.Message != "" {
			return env.Error.Message
		}
	}
	return err.Error()
}
