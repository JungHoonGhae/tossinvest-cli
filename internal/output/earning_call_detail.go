package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func optionalRate(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// WriteEarningCallDetail keeps the complete response in JSON and exposes a
// stable, loss-aware flat shape to CSV and human-readable table output.
func WriteEarningCallDetail(w io.Writer, format Format, detail domain.EarningCallDetail) error {
	if format == FormatJSON {
		return writeJSON(w, detail)
	}
	rows := [][]string{
		{"event_id", strconv.FormatInt(detail.EventID, 10)},
		{"company", detail.CompanyName},
		{"company_code", detail.CompanyCode},
		{"title", detail.Title},
		{"status", detail.Status},
		{"live_at", detail.LiveAt},
		{"went_live_at", optionalText(detail.WentLiveAt)},
		{"market_country", detail.MarketCountry},
		{"category", detail.Category},
		{"default_summarization_category", detail.DefaultSummarizationCategory},
		{"representative_stock_symbol", detail.RepresentativeStockSymbol},
		{"representative_stock_code", detail.RepresentativeStockCode},
		{"representative_stock_guid", detail.RepresentativeStockGUID},
		{"report_id", detail.ReportID},
		{"report_item", detail.ReportItem},
		{"consensus_gap_rate", optionalRate(detail.ConsensusGapRate)},
		{"stock_change_rate", optionalRate(detail.StockChangeRate)},
		{"is_gap_rate_visible", strconv.FormatBool(detail.IsGapRateVisible)},
		{"audio_url", optionalText(detail.AudioURL)},
		{"transcript_url", optionalText(detail.TranscriptURL)},
		{"slide_file_url", optionalText(detail.SlideFileURL)},
		{"company_logo_image_url", detail.CompanyLogoImageURL},
		{"mts_landing_path", detail.MTSLandingPath},
	}
	switch format {
	case FormatCSV:
		return writeCSV(w, []string{"field", "value"}, rows)
	case FormatTable:
		return renderTable(w,
			[]string{i18n.T("output.earningDetail.header.field"), i18n.T("output.earningDetail.header.value")},
			rows, AlignLeft, AlignLeft,
		)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
