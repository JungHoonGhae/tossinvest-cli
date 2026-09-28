package ops

import "context"

func researchOperations() []Operation {
	return []Operation{
		{ID: "earning_call_transcript", Method: "GET", Path: "wts:market/earnings/{event_id}/transcript", Backend: "wts", Category: "market", Summary: "Earnings paragraphs with timestamps, original text, nullable translation and summary.",
			Params: []Param{{Name: "event_id", Type: "integer", Required: true, Desc: "positive event ID from earning_calls"}},
			Probe:  &ProbeSpec{Name: "earning-call-transcript", Method: "GET", URL: probeCert + "/api/v1/company-events/228692/transcripts/paragraph-inferences", Check: statusAndPath("result", "array")},
			handler: func(ctx context.Context, d *Deps, a map[string]any) (any, error) {
				id, err := argInt(a, "event_id")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetEarningTranscript(ctx, int64(id))
			},
		},
		{ID: "earning_call_report", Method: "GET", Path: "wts:market/earnings/{event_id}/report", Backend: "wts", Category: "market", Summary: "Earnings analysis, watch points and source IDs. Unavailable sections remain null.",
			Params: []Param{{Name: "event_id", Type: "integer", Required: true, Desc: "positive event ID from earning_calls"}},
			Probe:  &ProbeSpec{Name: "earning-call-report", Method: "GET", URL: probeCert + "/api/v2/company-events/228692/report", Check: statusAndPath("result.eventId", "number")},
			handler: func(ctx context.Context, d *Deps, a map[string]any) (any, error) {
				id, err := argInt(a, "event_id")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetEarningReport(ctx, int64(id))
			},
		},
		{ID: "watchlist_news", Method: "GET", Path: "wts:watchlist/news", Backend: "wts", Category: "portfolio", Summary: "Recommended news for one existing watchlist folder, with related stock moves. One server-limited page.",
			Params:    []Param{{Name: "folder_id", Type: "integer", Required: true, Desc: "positive folder ID from watchlist_groups"}},
			ProbeRefs: []string{"watchlist-group"},
			Probe:     &ProbeSpec{Name: "watchlist-news", Method: "GET", URL: probeCert + "/api/v1/new-watchlists/recommend/news?watchlistId={watchlistGroupId}", WatchlistGroupScoped: true, Check: statusAndPath("result.newsList", "array")},
			handler: func(ctx context.Context, d *Deps, a map[string]any) (any, error) {
				id, err := argInt(a, "folder_id")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetWatchlistNews(ctx, int64(id))
			},
		},
	}
}
