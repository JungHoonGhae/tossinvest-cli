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

func TestWriteIndexQuoteIncludesVerifiedSessionMetadata(t *testing.T) {
	t.Parallel()
	quote := domain.IndexQuote{
		Code: "COMP.NAI", Name: "Nasdaq", Open: 100, High: 105, Low: 99, Close: 104, Base: 101,
		PriceFeed:      domain.IndexPriceFeed{Code: "REAL_TIME", Description: "realtime"},
		TradingStartAt: "2026-09-03T22:30:00+09:00",
		TradingEndAt:   "2026-09-04T05:00:00+09:00",
		MarketOpen:     true,
	}

	var table bytes.Buffer
	if err := WriteIndexQuote(&table, FormatTable, quote); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"REAL_TIME", "realtime", quote.TradingStartAt, quote.TradingEndAt, "true"} {
		if !strings.Contains(table.String(), want) {
			t.Errorf("table missing %q: %s", want, table.String())
		}
	}

	var csvOut bytes.Buffer
	if err := WriteIndexQuote(&csvOut, FormatCSV, quote); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(csvOut.String(), "code,name,open,high,low,close,change,change_rate,high_52w,low_52w,nation,base,volume,price_feed_code,price_feed_description,trading_start_at,trading_end_at,market_open\n") ||
		!strings.Contains(csvOut.String(), "REAL_TIME,realtime,"+quote.TradingStartAt+","+quote.TradingEndAt+",true") {
		t.Errorf("CSV reordered its released prefix or omitted session metadata: %s", csvOut.String())
	}

	var jsonOut bytes.Buffer
	if err := WriteIndexQuote(&jsonOut, FormatJSON, quote); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"price_feed"`, `"trading_start_at"`, `"trading_end_at"`, `"market_open": true`} {
		if !strings.Contains(jsonOut.String(), want) {
			t.Errorf("JSON missing %q: %s", want, jsonOut.String())
		}
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

func TestWriteRankingTable(t *testing.T) {
	r := domain.Ranking{
		Type: "MARKET_TRADING_AMOUNT", MarketCountry: "KR", Duration: "1d",
		Items: []domain.RankingItem{
			{Rank: 1, Symbol: "005930", LastPrice: 71900, ChangeRate: 0.0127, TradingAmount: 888000000000},
		},
	}
	var buf bytes.Buffer
	if err := WriteRanking(&buf, FormatTable, r); err != nil {
		t.Fatalf("WriteRanking: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "005930") {
		t.Errorf("table missing symbol: %q", out)
	}
}

func TestWriteRankingJSON(t *testing.T) {
	r := domain.Ranking{Type: "TOP_GAINERS", Items: []domain.RankingItem{{Rank: 1, Symbol: "X"}}}
	var buf bytes.Buffer
	if err := WriteRanking(&buf, FormatJSON, r); err != nil {
		t.Fatalf("WriteRanking JSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"symbol": "X"`) {
		t.Errorf("json missing field: %q", buf.String())
	}
}

func TestWriteMarketIndicatorPricesTable(t *testing.T) {
	p := domain.MarketIndicatorPrices{Indicators: []domain.MarketIndicatorPrice{{Symbol: "KOSPI", LastPrice: 2812.45, Timestamp: "2026-06-11T15:30:00+09:00"}}}
	var buf bytes.Buffer
	if err := WriteMarketIndicatorPrices(&buf, FormatTable, p); err != nil {
		t.Fatalf("WriteMarketIndicatorPrices: %v", err)
	}
	if !strings.Contains(buf.String(), "KOSPI") {
		t.Errorf("missing symbol: %q", buf.String())
	}
}

func TestWriteMarketIndicatorCandlesTable(t *testing.T) {
	c := domain.MarketIndicatorCandles{Symbol: "KOSPI", Interval: "1d", Candles: []domain.MarketIndicatorCandle{{Timestamp: "2026-06-11T09:00:00+09:00", Open: 2798.32, High: 2820.15, Low: 2790.1, Close: 2812.45, Volume: 123456}}}
	var buf bytes.Buffer
	if err := WriteMarketIndicatorCandles(&buf, FormatTable, c); err != nil {
		t.Fatalf("WriteMarketIndicatorCandles: %v", err)
	}
	if !strings.Contains(buf.String(), "2812.45") {
		t.Errorf("missing close: %q", buf.String())
	}
}

func TestWriteInvestorTradingTable(t *testing.T) {
	it := domain.InvestorTrading{
		Symbol: "KOSPI", Interval: "1d",
		Records: []domain.InvestorTradingRecord{
			{
				Date:             "2026-06-11",
				Individual:       domain.InvestorTradingParty{NetAmount: -150000000000},
				Foreigner:        domain.InvestorTradingParty{NetAmount: 200000000000},
				Institution:      domain.InvestorTradingParty{NetAmount: -80000000000},
				OtherCorporation: domain.InvestorTradingParty{NetAmount: 10000000000},
			},
		},
	}
	var buf bytes.Buffer
	if err := WriteInvestorTrading(&buf, FormatTable, it); err != nil {
		t.Fatalf("WriteInvestorTrading: %v", err)
	}
	if !strings.Contains(buf.String(), "2026-06-11") {
		t.Errorf("missing date: %q", buf.String())
	}
}

func TestWriteInvestorTradingJSON(t *testing.T) {
	it := domain.InvestorTrading{Symbol: "KOSPI", Records: []domain.InvestorTradingRecord{{Date: "D"}}}
	var buf bytes.Buffer
	if err := WriteInvestorTrading(&buf, FormatJSON, it); err != nil {
		t.Fatalf("WriteInvestorTrading JSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"symbol": "KOSPI"`) {
		t.Errorf("json missing: %q", buf.String())
	}
}

// 수수료율 스케일 회귀. 공식 API 는 소수 비율로 준다("0.00015" = 0.015%) — 표에서
// 퍼센트로 보이려면 ×100 이 정확히 한 번만 일어나야 한다. 두 번 곱하거나 아예 안
// 곱해도 에러가 안 나고 숫자만 100배 어긋난다.
//
// spec 1.2.14(2026-08-12)에서 이 필드의 표현이 실제로 바뀐 적이 있다.
func TestWriteCommissionPercentScale(t *testing.T) {
	c := domain.Commission{Symbol: "000000", CommissionRate: 0.00015, TaxRate: 0.0018}

	var table bytes.Buffer
	if err := WriteCommission(&table, FormatTable, c); err != nil {
		t.Fatalf("WriteCommission table: %v", err)
	}
	if !strings.Contains(table.String(), "0.015%") {
		t.Errorf("commission rate not rendered as 0.015%%:\n%s", table.String())
	}
	if !strings.Contains(table.String(), "0.18%") {
		t.Errorf("tax rate not rendered as 0.18%%:\n%s", table.String())
	}

	// CSV·JSON 은 기계용이라 원값을 그대로 둔다. 표만 퍼센트로 바꾼다.
	var csvBuf bytes.Buffer
	if err := WriteCommission(&csvBuf, FormatCSV, c); err != nil {
		t.Fatalf("WriteCommission csv: %v", err)
	}
	if !strings.Contains(csvBuf.String(), "commission_rate_ratio") {
		t.Errorf("CSV header should state the unit:\n%s", csvBuf.String())
	}
	if !strings.Contains(csvBuf.String(), "0.00015") {
		t.Errorf("CSV should carry the raw ratio:\n%s", csvBuf.String())
	}
}
