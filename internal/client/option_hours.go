package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type optionSessionRaw struct {
	USADate                  string  `json:"usaDate"`
	StartDateTime            *string `json:"startDateTime"`
	EndDateTime              *string `json:"endDateTime"`
	PreMarketStartDateTime   *string `json:"preMarketStartDateTime"`
	PreMarketEndDateTime     *string `json:"preMarketEndDateTime"`
	AfterMarketStartDateTime *string `json:"afterMarketStartDateTime"`
	AfterMarketEndDateTime   *string `json:"afterMarketEndDateTime"`
}

type optionHoursRaw struct {
	PrevBizDay optionSessionRaw `json:"prevBizDay"`
	Today      optionSessionRaw `json:"today"`
	NextBizDay optionSessionRaw `json:"nextBizDay"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r optionSessionRaw) toDomain() domain.OptionSession {
	return domain.OptionSession{
		Date:             r.USADate,
		Start:            deref(r.StartDateTime),
		End:              deref(r.EndDateTime),
		PreMarketStart:   deref(r.PreMarketStartDateTime),
		PreMarketEnd:     deref(r.PreMarketEndDateTime),
		AfterMarketStart: deref(r.AfterMarketStartDateTime),
		AfterMarketEnd:   deref(r.AfterMarketEndDateTime),
	}
}

// GetOptionTradingHours returns the US-options session windows for the
// previous, current, and next business day. 공식 Open API 에 없는 web 전용 표면.
//
// The previous/next days are the point of this endpoint: they say which days
// actually trade, which is what you need around holidays. Equity hours live in
// GetTradingHours and can differ.
func (c *Client) GetOptionTradingHours(ctx context.Context) (domain.OptionTradingHours, error) {
	if err := c.requireSession(); err != nil {
		return domain.OptionTradingHours{}, err
	}

	var envelope quoteEnvelope[optionHoursRaw]
	url := c.infoBaseURL + "/api/v1/usa-market/get-option-biz-day-by-overtime"
	if err := c.getJSON(ctx, url, &envelope); err != nil {
		return domain.OptionTradingHours{}, err
	}

	return domain.OptionTradingHours{
		Previous:  envelope.Result.PrevBizDay.toDomain(),
		Today:     envelope.Result.Today.toDomain(),
		Next:      envelope.Result.NextBizDay.toDomain(),
		FetchedAt: time.Now().UTC(),
	}, nil
}
