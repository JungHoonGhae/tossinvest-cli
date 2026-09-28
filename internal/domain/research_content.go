package domain

import "time"

type TranscriptLine struct {
	Index   int    `json:"index"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}
type TranscriptTranslation struct {
	ID    string           `json:"id"`
	Lines []TranscriptLine `json:"lines"`
}
type TranscriptSummary struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	MappedCategory string `json:"mapped_category"`
	Text           string `json:"text"`
	OriginalText   string `json:"original_text"`
}
type TranscriptParagraph struct {
	ID           string                 `json:"id"`
	OriginalText string                 `json:"original_text"`
	StartSeconds float64                `json:"start_seconds"`
	EndSeconds   float64                `json:"end_seconds"`
	Translation  *TranscriptTranslation `json:"translation"`
	Summary      *TranscriptSummary     `json:"summary"`
}

// Empty paragraphs mean no transcript is available; they do not imply a failed call.
type EarningTranscript struct {
	EventID    int64                 `json:"event_id"`
	Available  bool                  `json:"available"`
	Paragraphs []TranscriptParagraph `json:"paragraphs"`
	FetchedAt  time.Time             `json:"fetched_at"`
}
type EarningAnalysisItem struct {
	ID      int64    `json:"id"`
	Title   string   `json:"title"`
	Content []string `json:"content"`
}
type EarningAnalysis struct {
	ReportID int64                 `json:"report_id"`
	Overall  EarningAnalysisItem   `json:"overall"`
	Pros     []EarningAnalysisItem `json:"pros"`
	Cons     []EarningAnalysisItem `json:"cons"`
}
type EarningWatchPoint struct {
	Title   string   `json:"title"`
	Bullets []string `json:"bullets"`
}
type EarningSource struct {
	Type       string  `json:"type"`
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Provider   string  `json:"provider"`
	FaviconURL string  `json:"favicon_url,omitempty"`
	Subtitle   *string `json:"subtitle"`
	CreatedAt  string  `json:"created_at"`
}
type EarningWatchPoints struct {
	Points    []EarningWatchPoint `json:"points"`
	Sources   []EarningSource     `json:"sources"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
}

// Analysis and WatchPoints are independently nullable, including before a call.
type EarningReport struct {
	EventID     int64               `json:"event_id"`
	Analysis    *EarningAnalysis    `json:"analysis"`
	WatchPoints *EarningWatchPoints `json:"watch_points"`
	FetchedAt   time.Time           `json:"fetched_at"`
}
type WatchlistNewsStock struct {
	Name        string  `json:"name"`
	Fluctuation float64 `json:"fluctuation"`
}
type WatchlistNewsItem struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	Agency        string               `json:"agency"`
	Type          string               `json:"type"`
	CreatedAt     string               `json:"created_at"`
	ImageURL      string               `json:"image_url,omitempty"`
	RelatedStocks []WatchlistNewsStock `json:"related_stocks"`
}
type WatchlistNews struct {
	FolderID    int64               `json:"folder_id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	PageSize    int                 `json:"page_size"`
	News        []WatchlistNewsItem `json:"news"`
	FetchedAt   time.Time           `json:"fetched_at"`
}
