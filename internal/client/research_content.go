package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// resultJSON distinguishes a missing envelope from an explicitly unavailable result.
func (c *Client) resultJSON(ctx context.Context, endpoint string) (json.RawMessage, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("response is missing result")
	}
	return envelope.Result, nil
}

type transcriptParagraphRaw struct {
	EventID      int64    `json:"eventId"`
	ParagraphID  string   `json:"paragraphId"`
	OriginalText string   `json:"originalText"`
	Start        *float64 `json:"elapsedStartSeconds"`
	End          *float64 `json:"elapsedEndSeconds"`
	Translated   *struct {
		ID    string                  `json:"id"`
		Lines []domain.TranscriptLine `json:"translation"`
	} `json:"translated"`
	Summarized *struct {
		ID      string `json:"id"`
		Summary struct {
			Category       string `json:"category"`
			MappedCategory string `json:"mappedCategory"`
			Text           string `json:"text"`
			OriginalText   string `json:"originalText"`
		} `json:"summarization"`
	} `json:"summarized"`
}

func (c *Client) GetEarningTranscript(ctx context.Context, eventID int64) (domain.EarningTranscript, error) {
	out := domain.EarningTranscript{EventID: eventID, Paragraphs: []domain.TranscriptParagraph{}, FetchedAt: time.Now().UTC()}
	if eventID <= 0 {
		return out, fmt.Errorf("event id must be a positive integer")
	}
	data, err := c.resultJSON(ctx, fmt.Sprintf("%s/api/v1/company-events/%d/transcripts/paragraph-inferences", c.certBaseURL, eventID))
	if err != nil {
		return out, err
	}
	var rows []transcriptParagraphRaw
	if err = json.Unmarshal(data, &rows); err != nil {
		return out, fmt.Errorf("invalid transcript response: %w", err)
	}
	if rows == nil {
		return out, fmt.Errorf("transcript result must be an explicit array")
	}
	for _, r := range rows {
		if r.EventID != eventID || r.ParagraphID == "" || r.Start == nil || r.End == nil || *r.Start < 0 || *r.End < *r.Start {
			return out, fmt.Errorf("invalid transcript paragraph identity or time range")
		}
		p := domain.TranscriptParagraph{ID: r.ParagraphID, OriginalText: r.OriginalText, StartSeconds: *r.Start, EndSeconds: *r.End}
		if r.Translated != nil {
			p.Translation = &domain.TranscriptTranslation{ID: r.Translated.ID, Lines: r.Translated.Lines}
		}
		if r.Summarized != nil {
			s := r.Summarized.Summary
			p.Summary = &domain.TranscriptSummary{ID: r.Summarized.ID, Category: s.Category, MappedCategory: s.MappedCategory, Text: s.Text, OriginalText: s.OriginalText}
		}
		out.Paragraphs = append(out.Paragraphs, p)
	}
	out.Available = len(out.Paragraphs) > 0
	return out, nil
}

type earningReportRaw struct {
	EventID  int64 `json:"eventId"`
	Analysis *struct {
		ReportID int64                        `json:"reportId"`
		Overall  domain.EarningAnalysisItem   `json:"overall"`
		Pros     []domain.EarningAnalysisItem `json:"pros"`
		Cons     []domain.EarningAnalysisItem `json:"cons"`
	} `json:"analysis"`
	WatchPoint *struct {
		Points  []domain.EarningWatchPoint `json:"points"`
		Sources []struct {
			Type     string `json:"type"`
			ID       string `json:"sourceId"`
			Title    string `json:"title"`
			Provider struct {
				Name       string  `json:"name"`
				FaviconURL string  `json:"faviconUrl"`
				Subtitle   *string `json:"subTitle"`
			} `json:"provider"`
			CreatedAt string `json:"createdAt"`
		} `json:"sources"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	} `json:"watchPoint"`
}

func (c *Client) GetEarningReport(ctx context.Context, eventID int64) (domain.EarningReport, error) {
	out := domain.EarningReport{EventID: eventID, FetchedAt: time.Now().UTC()}
	if eventID <= 0 {
		return out, fmt.Errorf("event id must be a positive integer")
	}
	data, err := c.resultJSON(ctx, fmt.Sprintf("%s/api/v2/company-events/%d/report", c.certBaseURL, eventID))
	if err != nil {
		return out, err
	}
	var raw earningReportRaw
	if err = json.Unmarshal(data, &raw); err != nil {
		return out, fmt.Errorf("invalid earnings report: %w", err)
	}
	if raw.EventID != eventID {
		return out, fmt.Errorf("earnings report event identity mismatch")
	}
	// Missing fields are contract drift; explicit null is an unavailable section.
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(data, &fields); err != nil {
		return out, err
	}
	if fields["analysis"] == nil || fields["watchPoint"] == nil {
		return out, fmt.Errorf("earnings report is missing sections")
	}
	if a := raw.Analysis; a != nil {
		out.Analysis = &domain.EarningAnalysis{ReportID: a.ReportID, Overall: a.Overall, Pros: a.Pros, Cons: a.Cons}
	}
	if p := raw.WatchPoint; p != nil {
		out.WatchPoints = &domain.EarningWatchPoints{Points: p.Points, Sources: []domain.EarningSource{}, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
		for _, s := range p.Sources {
			out.WatchPoints.Sources = append(out.WatchPoints.Sources, domain.EarningSource{Type: s.Type, ID: s.ID, Title: s.Title, Provider: s.Provider.Name, FaviconURL: s.Provider.FaviconURL, Subtitle: s.Provider.Subtitle, CreatedAt: s.CreatedAt})
		}
	}
	return out, nil
}

func (c *Client) GetWatchlistNews(ctx context.Context, folderID int64) (domain.WatchlistNews, error) {
	out := domain.WatchlistNews{FolderID: folderID, News: []domain.WatchlistNewsItem{}, FetchedAt: time.Now().UTC()}
	if folderID <= 0 {
		return out, fmt.Errorf("folder id must be a positive integer")
	}
	// Do not let a missing/foreign folder silently fall back to all-watchlist news.
	if _, err := c.GetWatchlistGroup(ctx, folderID); err != nil {
		return out, err
	}
	data, err := c.resultJSON(ctx, fmt.Sprintf("%s/api/v1/new-watchlists/recommend/news?watchlistId=%d", c.certBaseURL, folderID))
	if err != nil {
		return out, err
	}
	var raw struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		PageSize    int    `json:"pageSize"`
		News        []struct {
			ID            string                      `json:"newsId"`
			Title         string                      `json:"title"`
			Agency        string                      `json:"agencyName"`
			Type          string                      `json:"newsType"`
			CreatedAt     string                      `json:"createdAt"`
			ImageURL      string                      `json:"imageUrl"`
			RelatedStocks []domain.WatchlistNewsStock `json:"relatedStockDtos"`
		} `json:"newsList"`
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		return out, fmt.Errorf("invalid watchlist news: %w", err)
	}
	if raw.News == nil {
		return out, fmt.Errorf("watchlist newsList must be an explicit array")
	}
	out.Title = raw.Title
	out.Description = raw.Description
	out.PageSize = raw.PageSize
	for _, n := range raw.News {
		out.News = append(out.News, domain.WatchlistNewsItem{ID: n.ID, Title: n.Title, Agency: n.Agency, Type: n.Type, CreatedAt: n.CreatedAt, ImageURL: n.ImageURL, RelatedStocks: n.RelatedStocks})
	}
	return out, nil
}
