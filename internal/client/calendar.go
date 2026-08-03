package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Economic calendar (다가오는 경제 일정). One endpoint, no parameters.
//
// Measured against the live service: from/to, date, month and size are all
// accepted and all ignored — the response is always the same forward window of
// roughly ten days. So there is nothing to expose as a flag, and a caller who
// wants history has to keep its own record.

type economicEventRaw struct {
	ID struct {
		UniqueName string `json:"uniqueName"`
		Group      string `json:"group"`
	} `json:"id"`
	Date  string `json:"date"`
	Time  string `json:"time"`
	Title string `json:"title"`
}

type economicCalendarRaw struct {
	Events    []economicEventRaw `json:"events"`
	AISummary *struct {
		Title string `json:"title"`
	} `json:"aiSummary"`
}

// GetEconomicCalendar returns the upcoming scheduled releases the web
// dashboard shows, plus Toss's one-line AI framing of the window. WTS-only.
func (c *Client) GetEconomicCalendar(ctx context.Context) (domain.EconomicCalendar, error) {
	if err := c.requireSession(); err != nil {
		return domain.EconomicCalendar{}, err
	}

	var env quoteEnvelope[economicCalendarRaw]
	url := c.infoBaseURL + "/api/v2/dashboard/wts/overview/calendar/economic-events"
	if err := c.getJSON(ctx, url, &env); err != nil {
		return domain.EconomicCalendar{}, err
	}

	out := domain.EconomicCalendar{
		Events:    make([]domain.EconomicEvent, 0, len(env.Result.Events)),
		FetchedAt: time.Now(),
	}
	for _, e := range env.Result.Events {
		out.Events = append(out.Events, domain.EconomicEvent{
			Date:  e.Date,
			Time:  normalizeEventTime(e.Time),
			Title: e.Title,
			Group: e.ID.Group,
			ID:    e.ID.UniqueName,
		})
	}
	if env.Result.AISummary != nil {
		out.Summary = env.Result.AISummary.Title
	}
	return out, nil
}

// normalizeEventTime trims the server's nanosecond precision to HH:MM. The
// feed uses 23:59:59.999999999 to mean "sometime that day", which as a clock
// time is noise; callers that want the raw value can read the API directly.
func normalizeEventTime(t string) string {
	if len(t) < 5 {
		return ""
	}
	hhmm := t[:5]
	if hhmm == "23:59" {
		return "" // "종일" — no useful clock time
	}
	return hhmm
}
