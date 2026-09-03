package output

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteOpenBankingStatusMasksEveryFormatByDefault(t *testing.T) {
	t.Parallel()
	status := domain.OpenBankingStatus{
		HolderName:       "홍길동",
		ConnectedAccount: &domain.OpenBankingAccount{AccountNo: "123-456-789", BankCode: "088", OpenBankingID: 42},
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteOpenBankingStatus(&out, format, status, false); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if strings.Contains(out.String(), "홍길동") || strings.Contains(out.String(), "123-456-789") {
			t.Fatalf("identity leaked in %s: %s", format, out.String())
		}
	}
}

func TestWriteOpenBankingStatusFullRevealsIdentity(t *testing.T) {
	t.Parallel()
	status := domain.OpenBankingStatus{
		HolderName:       "홍길동",
		ConnectedAccount: &domain.OpenBankingAccount{AccountNo: "123-456-789"},
	}
	var out bytes.Buffer
	if err := WriteOpenBankingStatus(&out, FormatJSON, status, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "홍길동") || !strings.Contains(out.String(), "123-456-789") {
		t.Fatalf("full output did not reveal requested identity: %s", out.String())
	}
}

func TestWriteOpenBankingStatusReportsLateTableWriteFailure(t *testing.T) {
	t.Parallel()
	status := domain.OpenBankingStatus{
		HolderName:       "홍길동",
		ConnectedAccount: &domain.OpenBankingAccount{AccountNo: "123-456-789", BankCode: "088"},
	}
	if err := WriteOpenBankingStatus(&failingWriter{after: 1}, FormatTable, status, true); err == nil {
		t.Fatal("late table write failure must be returned")
	}
}

func TestWriteOpenBankingStatusShowsDisconnectedWithoutRevealHint(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteOpenBankingStatus(&out, FormatTable, domain.OpenBankingStatus{}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not connected") || strings.Contains(out.String(), "--full") || strings.Contains(out.String(), "Account:") {
		t.Fatalf("disconnected table = %q", out.String())
	}
}

func TestWriteOpenBankingStatusShowsConnectionCapabilitiesInEveryFormat(t *testing.T) {
	t.Parallel()
	status := domain.OpenBankingStatus{ConnectionCreatable: true, RegistrationRequired: false}
	tests := []struct {
		format Format
		want   []string
	}{
		{format: FormatTable, want: []string{"Connection creatable:  true", "Registration required: false"}},
		{format: FormatJSON, want: []string{`"connection_creatable": true`, `"registration_required": false`}},
		{format: FormatCSV, want: []string{"connection_creatable", "registration_required", "true,false"}},
	}
	for _, tc := range tests {
		var out bytes.Buffer
		if err := WriteOpenBankingStatus(&out, tc.format, status, false); err != nil {
			t.Fatalf("%s: %v", tc.format, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%s missing %q: %s", tc.format, want, out.String())
			}
		}
	}
}

func TestWriteMarketKeyEventsRendersBothSections(t *testing.T) {
	t.Parallel()
	eps := 1.2
	actual, forecast, historical := 2.1, 2.0, 1.9
	events := domain.MarketKeyEvents{
		Earnings:   []domain.MarketKeyEarning{{AnnounceAt: "2026-09-03", CompanyName: "Example", EPS: &eps}},
		Indicators: []domain.MarketKeyIndicator{{AnnounceAt: "2026-09-03", Title: "CPI", Actual: &actual, Forecast: &forecast, Historical: &historical, Unit: "%"}},
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteMarketKeyEvents(&out, format, events); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if !strings.Contains(out.String(), "Example") || !strings.Contains(out.String(), "CPI") {
			t.Fatalf("sections missing in %s: %s", format, out.String())
		}
	}
}

func TestWriteEarningCallsExposesEventIDForDetailLookup(t *testing.T) {
	t.Parallel()
	calls := domain.EarningCalls{Events: []domain.EarningCall{{EventID: 42, CompanyName: "Dummy Inc.", Title: "Q2 call"}}}
	for _, format := range []Format{FormatTable, FormatCSV} {
		var out bytes.Buffer
		if err := WriteEarningCalls(&out, format, calls); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if !strings.Contains(out.String(), "42") {
			t.Fatalf("event id missing in %s: %s", format, out.String())
		}
	}
}

func TestWriteEarningCallDetailPreservesVerifiedFields(t *testing.T) {
	t.Parallel()
	gap := 1.25
	audio := "https://example.invalid/audio"
	detail := domain.EarningCallDetail{
		EventID: 42, CompanyName: "Dummy Inc.", Title: "Q2 call", Status: "ENDED",
		RepresentativeStockCode: "USDUMMY", ReportID: "report-42", AudioURL: &audio,
		ConsensusGapRate: &gap,
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteEarningCallDetail(&out, format, detail); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, want := range []string{"42", "Dummy Inc.", "USDUMMY", "report-42"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%q missing in %s: %s", want, format, out.String())
			}
		}
	}
}

func TestWriteEarningCallDetailJSONPreservesNullPublicationState(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteEarningCallDetail(&out, FormatJSON, domain.EarningCallDetail{EventID: 42}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"audio_url": null`, `"transcript_url": null`, `"slide_file_url": null`, `"consensus_gap_rate": null`, `"stock_change_rate": null`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("nullable field %q missing: %s", want, out.String())
		}
	}
}

func TestWriteMarketKeyEventsShowsMissingFutureValues(t *testing.T) {
	t.Parallel()
	events := domain.MarketKeyEvents{Indicators: []domain.MarketKeyIndicator{{Title: "Upcoming", Unit: "%"}}}
	var out bytes.Buffer
	if err := WriteMarketKeyEvents(&out, FormatTable, events); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Upcoming") || strings.Contains(out.String(), "0.00%") {
		t.Fatalf("missing future values must not look like zero: %s", out.String())
	}
}

func TestWriteMarketKeyEventsCSVShowsMissingFutureValues(t *testing.T) {
	t.Parallel()
	events := domain.MarketKeyEvents{Indicators: []domain.MarketKeyIndicator{{Title: "Upcoming", Unit: "%"}}}
	var out bytes.Buffer
	if err := WriteMarketKeyEvents(&out, FormatCSV, events); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Upcoming,-,-,-,%") {
		t.Fatalf("null indicator CSV = %q", out.String())
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil || len(records) != 2 || len(records[0]) != len(records[1]) {
		t.Fatalf("CSV column mismatch: records=%v err=%v", records, err)
	}
}

func TestWriteMarketKeyEventsCSVPreservesCompleteEarningsFields(t *testing.T) {
	t.Parallel()
	value, display := 20.0, "20%"
	events := domain.MarketKeyEvents{Earnings: []domain.MarketKeyEarning{
		{CompanyName: "Example", EPSSurprise: &value, EPSSurpriseDisplay: &display, SalesSurprise: &value, OperatingSurprise: &value, ReportID: "r1"},
	}}
	var out bytes.Buffer
	if err := WriteMarketKeyEvents(&out, FormatCSV, events); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surprise_display", "sales_surprise", "operating_profit_surprise", "report_id", "20%", "r1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("%q missing from complete earnings CSV: %s", want, out.String())
		}
	}
}

func TestWriteNotificationSettingsShowsEnabledState(t *testing.T) {
	t.Parallel()
	settings := domain.NotificationSettings{Settings: []domain.NotificationSetting{{ID: 1, Type: "CALENDAR_AI_SUMMARY_WEEKLY", Enabled: true}}}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteNotificationSettings(&out, format, settings); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if !strings.Contains(out.String(), "CALENDAR_AI_SUMMARY_WEEKLY") {
			t.Fatalf("setting missing in %s: %s", format, out.String())
		}
	}
}

func TestWriteNotificationSettingsSortsWithoutMutatingAndLabelsUntyped(t *testing.T) {
	t.Parallel()
	settings := domain.NotificationSettings{Settings: []domain.NotificationSetting{
		{ID: 1, Type: "Z_LAST", Enabled: true},
		{ID: 2, Type: "", Enabled: false},
		{ID: 3, Type: "A_FIRST", Enabled: true},
	}}
	var out bytes.Buffer
	if err := WriteNotificationSettings(&out, FormatTable, settings); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "(untyped)") || strings.Index(text, "A_FIRST") > strings.Index(text, "Z_LAST") {
		t.Fatalf("sorted table = %q", text)
	}
	if settings.Settings[0].Type != "Z_LAST" || settings.Settings[2].Type != "A_FIRST" {
		t.Fatalf("input mutated: %#v", settings.Settings)
	}
}

func TestWriteNewsBriefingIncludesPersonalizedReasoning(t *testing.T) {
	t.Parallel()
	briefing := domain.NewsBriefing{Items: []domain.BriefingItem{{
		CategoryType: "수급", AssetName: "삼성전자", AssetCode: "A005930",
		ProfitLossRate: 12.5, ReasoningTitle: "외국인 순매수",
	}}}
	var out bytes.Buffer
	if err := WriteNewsBriefing(&out, FormatTable, briefing); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"삼성전자", "A005930", "12.50%", "외국인 순매수"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("%q missing: %s", want, out.String())
		}
	}
}

func TestWriteNewsBriefingCSVPreservesLegacyFourColumnContract(t *testing.T) {
	t.Parallel()
	briefing := domain.NewsBriefing{Items: []domain.BriefingItem{{
		CategoryType: "수급", Keywords: []string{"외국인"}, Section: "HOLDING",
		SignalID: "sig-1", TraceID: "trace-1", CreatedAt: "2026-09-02T01:00:00Z",
		AssetCode: "A005930", AssetName: "삼성전자", AssetLogoImageURL: "https://example.test/logo.png",
		AssetType: "STOCK", InvestmentType: "HOLDING", ProfitLossRate: 12.5,
		SignalDirection: 1, ReasoningTitle: "외국인 순매수",
		RelatedStocks: []domain.RelatedStock{{ProductCode: "A000660"}},
		News:          []domain.BriefingNews{{Title: "헤드라인", Agency: "통신사", CreatedAt: "2026-09-02T00:30:00Z"}},
	}}}
	var out bytes.Buffer
	if err := WriteNewsBriefing(&out, FormatCSV, briefing); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil || len(records) != 2 || len(records[0]) != 4 || len(records[1]) != 4 {
		t.Fatalf("legacy CSV shape changed: records=%v err=%v", records, err)
	}
	wantHeader := []string{"category", "title", "agency", "created_at"}
	for i := range wantHeader {
		if records[0][i] != wantHeader[i] {
			t.Fatalf("header = %v", records[0])
		}
	}
	if strings.Contains(out.String(), "sig-1") || strings.Contains(out.String(), "A000660") {
		t.Fatalf("legacy CSV unexpectedly changed: %s", out.String())
	}
}

func TestWriteNewsBriefingTableReportsWriteFailure(t *testing.T) {
	t.Parallel()
	briefing := domain.NewsBriefing{Items: []domain.BriefingItem{{CategoryType: "수급", ReasoningTitle: "사유"}}}
	if err := WriteNewsBriefing(&failingWriter{after: 0}, FormatTable, briefing); err == nil {
		t.Fatal("table write failure must be returned")
	}
}
