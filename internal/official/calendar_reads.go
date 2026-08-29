package official

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// businessDayRaw covers both markets' shapes at once.
//
// KR nests its sessions under `integrated` and adds 단일가 auction times; US
// puts them at the top level and adds `dayMarket`. Decoding both into one
// struct is safe because the two never populate the same field — the absent
// side simply stays nil.
type businessDayRaw struct {
	Date       string `json:"date"`
	Integrated *struct {
		PreMarket     *sessionRaw `json:"preMarket"`
		RegularMarket *sessionRaw `json:"regularMarket"`
		AfterMarket   *sessionRaw `json:"afterMarket"`
	} `json:"integrated"`
	PreMarket     *sessionRaw `json:"preMarket"`
	DayMarket     *sessionRaw `json:"dayMarket"`
	RegularMarket *sessionRaw `json:"regularMarket"`
	AfterMarket   *sessionRaw `json:"afterMarket"`
}

type sessionRaw struct {
	StartTime                   string `json:"startTime"`
	EndTime                     string `json:"endTime"`
	SinglePriceAuctionStartTime string `json:"singlePriceAuctionStartTime"`
	SinglePriceAuctionEndTime   string `json:"singlePriceAuctionEndTime"`
}

// sessions flattens whichever shape the server used, in trading order.
func (b businessDayRaw) sessions() []domain.TradingSession {
	type named struct {
		name string
		raw  *sessionRaw
	}
	var order []named
	if b.Integrated != nil {
		order = []named{
			{"pre_market", b.Integrated.PreMarket},
			{"regular_market", b.Integrated.RegularMarket},
			{"after_market", b.Integrated.AfterMarket},
		}
	} else {
		order = []named{
			{"pre_market", b.PreMarket},
			{"day_market", b.DayMarket},
			{"regular_market", b.RegularMarket},
			{"after_market", b.AfterMarket},
		}
	}
	var out []domain.TradingSession
	for _, n := range order {
		if n.raw == nil || (n.raw.StartTime == "" && n.raw.EndTime == "") {
			continue
		}
		out = append(out, domain.TradingSession{
			Name:                    n.name,
			Start:                   n.raw.StartTime,
			End:                     n.raw.EndTime,
			SinglePriceAuctionStart: n.raw.SinglePriceAuctionStartTime,
			SinglePriceAuctionEnd:   n.raw.SinglePriceAuctionEndTime,
		})
	}
	return out
}

func (b businessDayRaw) toDomain() domain.BusinessDay {
	s := b.sessions()
	// 두 시장이 휴장을 다르게 표현한다 — KR 은 integrated 를 통째로 null 로,
	// US 는 세션을 전부 비워서. 세션이 하나도 없으면 휴장으로 본다.
	return domain.BusinessDay{Date: b.Date, Holiday: len(s) == 0, Sessions: s}
}

// MarketCalendar fetches the trading-hours calendar for a market.
//
// Endpoint: GET /api/v1/market-calendar/{country} ("KR" | "US")
// Query:    date (optional) "YYYY-MM-DD"; defaults to today on the server.
//
// The two markets return structurally different payloads; this normalizes them
// into one shape (see businessDayRaw). Until 2026-08 the response was passed
// through as a decoded map because nothing computed over it — `market
// business-days` now does.
func (c *Client) MarketCalendar(ctx context.Context, country, date string) (domain.TradingCalendar, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country != "KR" && country != "US" {
		return domain.TradingCalendar{}, fmt.Errorf("market calendar country must be KR or US, got %q", country)
	}
	q := url.Values{}
	if date != "" {
		q.Set("date", date)
	}
	// c.get 이 `result` 봉투를 이미 벗겨서 넣어준다.
	var r struct {
		PreviousBusinessDay businessDayRaw `json:"previousBusinessDay"`
		Today               businessDayRaw `json:"today"`
		NextBusinessDay     businessDayRaw `json:"nextBusinessDay"`
	}
	if err := c.get(ctx, "/api/v1/market-calendar/"+country, q, &r); err != nil {
		return domain.TradingCalendar{}, err
	}
	return domain.TradingCalendar{
		Country:   country,
		Previous:  r.PreviousBusinessDay.toDomain(),
		Today:     r.Today.toDomain(),
		Next:      r.NextBusinessDay.toDomain(),
		FetchedAt: time.Now().UTC(),
	}, nil
}
