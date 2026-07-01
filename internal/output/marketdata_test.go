package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func TestSessionTime(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("ko")
	defer i18n.SetLang(prev)

	cases := []struct{ in, want string }{
		{"", "휴장"},
		{"09:00:00.000", "09:00"},
		{"22:30:00.000", "22:30"},
		{"9:0", "9:0"}, // too short, passthrough
	}
	for _, c := range cases {
		if got := sessionTime(c.in); got != c.want {
			t.Errorf("sessionTime(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWriteStockWarningsEmpty(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("ko")
	defer i18n.SetLang(prev)

	var buf bytes.Buffer
	sw := domain.StockWarnings{ProductCode: "A005930", Name: "삼성전자"}
	if err := WriteStockWarnings(&buf, FormatTable, sw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "매수 유의사항 없음") {
		t.Errorf("expected empty-warning notice, got %q", buf.String())
	}
}

func TestWriteScreenerResultTable(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("ko")
	defer i18n.SetLang(prev)

	var buf bytes.Buffer
	sr := domain.ScreenerResult{
		PresetName: "꾸준한 배당주", Nation: "kr", TotalCount: 26,
		Stocks: []domain.ScreenedStock{{ProductCode: "A095570", Name: "AJ네트웍스", Close: 4380, ChangeRate: 0.0023}},
	}
	if err := WriteScreenerResult(&buf, FormatTable, sr); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "꾸준한 배당주") || !strings.Contains(out, "AJ네트웍스") {
		t.Errorf("expected screener row: %q", out)
	}
	if !strings.Contains(out, "26종목") {
		t.Errorf("expected total count: %q", out)
	}
}

func TestWriteScreenerPresetsTable(t *testing.T) {
	var buf bytes.Buffer
	sp := domain.ScreenerPresets{Presets: []domain.ScreenerPreset{
		{ID: "8", Name: "꾸준한 배당주", Description: "배당을 꾸준히 주는 주식"},
	}}
	if err := WriteScreenerPresets(&buf, FormatTable, sp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "꾸준한 배당주") {
		t.Errorf("expected preset row: %q", buf.String())
	}
}

func TestWriteAISignalsTable(t *testing.T) {
	var buf bytes.Buffer
	sg := domain.AISignals{Label: "AI 시그널", Signals: []domain.AISignal{
		{AssetName: "아마존", Title: "AI 인프라 투자 부담", Keyword: "AI 투자 부담", Fluctuation: "1.7% 하락"},
	}}
	if err := WriteAISignals(&buf, FormatTable, sg); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "아마존") || !strings.Contains(out, "AI 인프라 투자 부담") {
		t.Errorf("expected signal row: %q", out)
	}
}

func TestWriteTradingFlowsTableSigned(t *testing.T) {
	var buf bytes.Buffer
	tf := domain.TradingFlows{
		ProductCode: "A005930", Name: "삼성전자",
		Flows: []domain.TradingFlow{
			{Date: "2026-06-02", NetIndividuals: 10917935, NetForeigner: -13711920, NetInstitution: 2602241},
		},
	}
	if err := WriteTradingFlows(&buf, FormatTable, tf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "+10,917,935") {
		t.Errorf("expected signed+comma individuals: %q", out)
	}
	if !strings.Contains(out, "-13,711,920") {
		t.Errorf("expected negative foreigner: %q", out)
	}
}

func TestWriteMarketIndicesCSV(t *testing.T) {
	var buf bytes.Buffer
	mi := domain.MarketIndices{Indices: []domain.MarketIndex{
		{Code: "KGG01P", Name: "코스피", Nation: "kr", Latest: 8801.49, Base: 8788.38, Change: 13.11, ChangeRate: 0.0015},
	}}
	if err := WriteMarketIndices(&buf, FormatCSV, mi); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "code,name,nation,latest,base,change,change_rate") {
		t.Errorf("unexpected CSV header: %q", out)
	}
	if !strings.Contains(out, "코스피") {
		t.Errorf("expected 코스피 in CSV: %q", out)
	}
}

func TestWriteStockRankingTable(t *testing.T) {
	var buf bytes.Buffer
	sr := domain.StockRanking{Stocks: []domain.RankedStock{
		{Rank: 1, Symbol: "TSLA", Name: "테슬라", Market: "NASDAQ"},
	}}
	if err := WriteStockRanking(&buf, FormatTable, sr); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "테슬라") || !strings.Contains(out, "TSLA") {
		t.Errorf("expected ranked stock in table: %q", out)
	}
}

func TestWriteTradesCSVHeader(t *testing.T) {
	var buf bytes.Buffer
	list := domain.TradeList{Trades: []domain.Trade{{Time: "09:00:01", Price: 360500, Volume: 10, TradeType: "BUY"}}}
	if err := WriteTrades(&buf, FormatCSV, list); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "time,price,volume,trade_type,cumulative_volume") {
		t.Errorf("unexpected CSV header: %q", out)
	}
	if !strings.Contains(out, "360500") {
		t.Errorf("expected price in CSV, got %q", out)
	}
}

func TestWriteThemeRankingsTable(t *testing.T) {
	var buf bytes.Buffer
	r := domain.ThemeRankings{
		Name: "tics_fluctuation_v2", DateTime: "2026-06-29T12:30:35",
		Items: []domain.ThemeRanking{
			{Ranking: 1, TicsID: "685", Title: "배터리제조", ChangeRate: 11.18, RiseCompanyCount: 13, TotalCount: 21, Summary: "21개 중 13개 종목 상승"},
			{Ranking: 96, TicsID: "100", Title: "조선", ChangeRate: -3.2, RiseCompanyCount: 1, TotalCount: 10},
		},
	}
	if err := WriteThemeRankings(&buf, FormatTable, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "배터리제조") || !strings.Contains(out, "+11.18%") {
		t.Errorf("expected gainer row: %q", out)
	}
	if !strings.Contains(out, "-3.20%") {
		t.Errorf("expected negative theme rate: %q", out)
	}
	if !strings.Contains(out, "13/21") {
		t.Errorf("expected rise/total counts: %q", out)
	}
	if !strings.Contains(out, "─") {
		t.Errorf("expected aligned table with separator rule: %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("table to non-TTY must be plain (no ANSI): %q", out)
	}
}

func TestWriteThemeRankingsCSV(t *testing.T) {
	var buf bytes.Buffer
	r := domain.ThemeRankings{Items: []domain.ThemeRanking{
		{Ranking: 1, TicsID: "685", Title: "배터리제조", ChangeRate: 11.18, RiseCompanyCount: 13, TotalCount: 21},
	}}
	if err := WriteThemeRankings(&buf, FormatCSV, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "ranking,tics_id,title,change_rate,rise_company_count,total_count") {
		t.Errorf("unexpected CSV header: %q", out)
	}
	if !strings.Contains(out, "배터리제조") {
		t.Errorf("expected theme in CSV: %q", out)
	}
}

func TestWriteThemeRankingsJSON(t *testing.T) {
	var buf bytes.Buffer
	r := domain.ThemeRankings{Name: "tics_fluctuation_v2", Items: []domain.ThemeRanking{
		{Ranking: 1, Title: "배터리제조", ChangeRate: 11.18},
	}}
	if err := WriteThemeRankings(&buf, FormatJSON, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"change_rate": 11.18`) || !strings.Contains(out, `"name": "tics_fluctuation_v2"`) {
		t.Errorf("unexpected JSON: %q", out)
	}
}

func TestWriteThemeRankingsEmpty(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("ko")
	defer i18n.SetLang(prev)

	var buf bytes.Buffer
	if err := WriteThemeRankings(&buf, FormatTable, domain.ThemeRankings{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "테마 데이터가 없습니다") {
		t.Errorf("expected empty notice: %q", buf.String())
	}
}
