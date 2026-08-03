package client

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Market calendar (증시 캘린더).
//
// Two endpoints, both discovered from the /calendar page's own chunks:
//
//   - POST /api/v4/calendar/monthly/{YYYY-MM} — the month's events. POST with an
//     empty body; a GET returns 405. The month is a PATH segment, not a query
//     parameter, which is why probing query strings suggested "no range
//     support" (see docs/reverse-engineering/capture-workflow.md).
//   - GET /api/v1/nova-calendar/ai/summary/weekly — the AI note for the current
//     week. Not month-scoped, so it is only meaningful for the present month.
//
// The dashboard widget endpoint (…/overview/calendar/economic-events) returns a
// fixed ~10-day slice of the same data with no earnings and no forecasts. It is
// a teaser for the real page, not a smaller version of it.

var monthRE = regexp.MustCompile(`^\d{4}-\d{2}$`)

// stockURLRE lifts the symbol out of an earnings entry's landing URL
// ("/stocks/A377300" → "A377300").
var stockURLRE = regexp.MustCompile(`/stocks/([A-Za-z0-9]+)`)

// calendarKinds maps Toss's group enum to a stable alias. An unmapped group
// becomes "other" rather than being dropped, so a new category Toss ships is
// still visible (with its raw name preserved on the event).
var calendarKinds = map[string]string{
	"ECONOMIC":                  "economic",
	"KRX_EARNINGS_ANNOUNCEMENT": "earnings_kr",
	"USD_EARNINGS_ANNOUNCEMENT": "earnings_us",
	"HOLIDAY":                   "holiday",
}

// CalendarKinds lists the aliases for help text and validation.
func CalendarKinds() []string {
	return []string{"economic", "earnings_kr", "earnings_us", "holiday"}
}

type calendarMonthlyRaw struct {
	Events []struct {
		ID struct {
			Group string `json:"group"`
		} `json:"id"`
		Date string `json:"date"`
		View struct {
			Title    string `json:"title"`
			Subtitle *struct {
				Text string `json:"text"`
			} `json:"subtitle"`
			LandingOption *struct {
				URL string `json:"url"`
			} `json:"landingOption"`
			UpcomingLive *struct {
				LiveAt string `json:"liveAt"`
			} `json:"upcomingLive"`
			EconomicIndicatorValue *struct {
				Unit       string   `json:"unit"`
				Actual     *float64 `json:"actual"`
				Forecast   *float64 `json:"forecast"`
				Historical *float64 `json:"historical"`
			} `json:"economicIndicatorValue"`
		} `json:"view"`
	} `json:"events"`
}

type novaSummaryRaw struct {
	Title    string `json:"title"`
	Contents string `json:"contents"`
}

// GetMarketCalendar returns one month of scheduled market events. month is
// "YYYY-MM"; an empty month means the current one. WTS-only.
func (c *Client) GetMarketCalendar(ctx context.Context, month string) (domain.MarketCalendar, error) {
	if err := c.requireSession(); err != nil {
		return domain.MarketCalendar{}, err
	}
	month = strings.TrimSpace(month)
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if !monthRE.MatchString(month) {
		return domain.MarketCalendar{}, fmt.Errorf("month must look like YYYY-MM, got %q", month)
	}

	var env quoteEnvelope[calendarMonthlyRaw]
	url := c.infoBaseURL + "/api/v4/calendar/monthly/" + month
	if err := c.postJSON(ctx, url, json.RawMessage(`{}`), &env); err != nil {
		return domain.MarketCalendar{}, err
	}

	out := domain.MarketCalendar{
		Month:     month,
		Events:    make([]domain.CalendarEvent, 0, len(env.Result.Events)),
		FetchedAt: time.Now(),
	}
	for _, e := range env.Result.Events {
		kind, ok := calendarKinds[e.ID.Group]
		if !ok {
			kind = "other"
		}
		ev := domain.CalendarEvent{
			Date:  e.Date,
			Title: e.View.Title,
			Kind:  kind,
			Group: e.ID.Group,
		}
		if e.View.Subtitle != nil {
			ev.Note = e.View.Subtitle.Text
		}
		if lo := e.View.LandingOption; lo != nil {
			if m := stockURLRE.FindStringSubmatch(lo.URL); m != nil {
				ev.Symbol = m[1]
			}
		}
		if ul := e.View.UpcomingLive; ul != nil {
			ev.LiveAt = ul.LiveAt
		}
		if iv := e.View.EconomicIndicatorValue; iv != nil {
			ev.Indicator = &domain.CalendarIndicator{
				Unit:       iv.Unit,
				Forecast:   iv.Forecast,
				Actual:     iv.Actual,
				Historical: iv.Historical,
			}
		}
		out.Events = append(out.Events, ev)
	}

	// The AI note covers the current week, so attaching it to a month the user
	// navigated away from would misdate it. Its absence is not an error —
	// the calendar itself is the answer.
	if month == time.Now().Format("2006-01") {
		var sum quoteEnvelope[novaSummaryRaw]
		if err := c.getJSON(ctx, c.infoBaseURL+"/api/v1/nova-calendar/ai/summary/weekly", &sum); err != nil {
			out.Warnings = append(out.Warnings, "주간 AI 요약: "+err.Error())
		} else {
			out.Summary = sum.Result.Title
			out.SummaryDetail = sum.Result.Contents
		}
	}
	return out, nil
}
