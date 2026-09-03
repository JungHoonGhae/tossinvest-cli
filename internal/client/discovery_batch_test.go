package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetNewsBriefingUsesRicherPersonalizedV2Contract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/reasoning/personalized" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"createdAt":"2026-09-02T00:00:00Z","items":[{"section":"HOLDING","signalId":"sig-1","traceId":"trace-1","createdAt":"2026-09-02T01:00:00Z","category":{"type":"수급","keywords":["외국인"]},"reasoningSummary":{"assetInfo":{"code":"A005930","name":"삼성전자","logoImageUrl":"https://example.test/logo.png"},"assetType":"STOCK","investmentType":"HOLDING","profitLossRate":12.5,"reasoningTitle":"외국인 순매수","signalDirection":1},"news":[{"id":"n1","title":"헤드라인","agencyName":"통신사","source":"src","faviconUrl":"https://example.test/favicon.png","createdAt":"2026-09-02T00:30:00Z"}],"relatedStocks":[{"code":"A000660"}]}]}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetNewsBriefing(context.Background())
	if err != nil {
		t.Fatalf("GetNewsBriefing: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %#v", got.Items)
	}
	item := got.Items[0]
	if item.AssetCode != "A005930" || item.AssetName != "삼성전자" || item.AssetLogoImageURL == "" || item.ReasoningTitle != "외국인 순매수" || item.ProfitLossRate != 12.5 || item.Section != "HOLDING" {
		t.Fatalf("richer personalized fields lost: %#v", item)
	}
	if len(item.News) != 1 || item.News[0].Title != "헤드라인" {
		t.Fatalf("news = %#v", item.News)
	}
	if item.News[0].ID != "n1" || item.News[0].FaviconURL == "" || len(item.RelatedStocks) != 1 || item.RelatedStocks[0].ProductCode != "A000660" {
		t.Fatalf("v2 payload fields lost: %#v", item)
	}
}

func TestGetMarketNewsBriefingUsesNationalLatestContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/dashboard/wts/overview/ai-signals/latest" || r.URL.Query().Get("nationCode") != "KOR" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"createdAt":"2026-09-03T00:00:00Z","items":[{"signalId":"sig-kr","traceId":"trace-kr","createdAt":"2026-09-03T01:00:00Z","category":{"type":"시장","keywords":["반도체"]},"reasoningSummary":{"assetInfo":{"code":"A000001","name":"예시 종목","logoImageUrl":"https://example.test/logo.png"},"assetType":"STOCKS","investmentType":"MARKET","profitLossRate":3.5,"reasoningTitle":"시장 브리핑","signalDirection":1},"news":[{"id":"news-kr","title":"국내 시장 뉴스","agencyName":"예시 통신사","source":"src","createdAt":"2026-09-03T00:30:00Z"}],"relatedStocks":[{"code":"A000002"}]}]}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetMarketNewsBriefing(context.Background(), "kr")
	if err != nil {
		t.Fatalf("GetMarketNewsBriefing: %v", err)
	}
	if got.Scope != "kr" || len(got.Items) != 1 {
		t.Fatalf("briefing = %#v", got)
	}
	item := got.Items[0]
	if item.SignalID != "sig-kr" || item.AssetCode != "A000001" || item.ReasoningTitle != "시장 브리핑" || len(item.News) != 1 || item.News[0].ID != "news-kr" {
		t.Fatalf("item = %#v", item)
	}
}

func TestGetMarketNewsBriefingRejectsUnknownMarketBeforeRequest(t *testing.T) {
	t.Parallel()
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested = true
	}))
	t.Cleanup(server.Close)

	_, err := testClientFor(server).GetMarketNewsBriefing(context.Background(), "jp")
	if err == nil {
		t.Fatal("expected unsupported market error")
	}
	if requested {
		t.Fatal("invalid market must be rejected before making a request")
	}
}

func TestGetMarketKeyEventsMapsEarningsAndEconomicIndicators(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/calendar/ai-summary/key-events" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"earnings":[{"announceDateTime":"2026-09-03T08:00:00Z","announceMarketStatus":"BEFORE_MARKET","announceMarketStatusText":"장전","companyCode":"US123","companyName":"Example","countryIcon":"us","logoImageUrl":"https://example.test/logo.png","eps":1.2,"epsDisplay":"$1.20","epsEst":1.0,"epsEstDisplay":"$1.00","epsSurprise":20,"epsSurpriseDisplay":"20%","sales":100,"salesDisplay":"$100","salesEst":90,"salesEstDisplay":"$90","salesSurprise":11.1,"salesSurpriseDisplay":"11.1%","operatingProfit":30,"operatingProfitDisplay":"$30","operatingProfitEst":25,"operatingProfitEstDisplay":"$25","operatingProfitSurprise":20,"operatingProfitSurpriseDisplay":"20%","legacyReportId":"legacy","reportId":"r1","reportItem":"Q2","landingUrl":"/stocks/US123"}],"eci":{"indicators":[{"actValNs":"2026-09-03T12:30:00Z","actualValue":2.1,"forecastValue":2.0,"historical":1.9,"title":"CPI","unit":"%","unitPrefix":"","displayUnit":"%","ric":"CPI"},{"actValNs":"2026-09-04T12:30:00Z","actualValue":null,"forecastValue":null,"historical":null,"title":"Upcoming","unit":"%","ric":"UP"}]}}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetMarketKeyEvents(context.Background())
	if err != nil {
		t.Fatalf("GetMarketKeyEvents: %v", err)
	}
	if len(got.Earnings) != 1 || got.Earnings[0].CompanyName != "Example" || got.Earnings[0].EPS == nil || *got.Earnings[0].EPS != 1.2 {
		t.Fatalf("earnings = %#v", got.Earnings)
	}
	if got.Earnings[0].SalesSurprise == nil || *got.Earnings[0].SalesSurprise != 11.1 || got.Earnings[0].OperatingSurprise == nil || got.Earnings[0].LegacyReportID == nil {
		t.Fatalf("complete earnings payload not preserved: %#v", got.Earnings[0])
	}
	if len(got.Indicators) != 2 || got.Indicators[0].Title != "CPI" || got.Indicators[0].Forecast == nil || *got.Indicators[0].Forecast != 2.0 {
		t.Fatalf("indicators = %#v", got.Indicators)
	}
	if got.Indicators[1].Actual != nil || got.Indicators[1].Forecast != nil || got.Indicators[1].Historical != nil {
		t.Fatalf("nullable future indicator values were collapsed: %#v", got.Indicators[1])
	}
}

func TestGetMarketKeyEventsKeepsEmptyCollectionsAsArrays(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"earnings":[],"eci":{"indicators":[]}}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetMarketKeyEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Earnings == nil || got.Indicators == nil {
		t.Fatalf("empty collections must be non-nil: %#v", got)
	}
}

func TestGetOpenBankingStatusMapsOnlyStableObservedFields(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/autotrade/open-banking/info/find" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"name":"홍길동","connectedOpenBankingAccount":{"accountNo":"123-456-789","bankCode":"088","openBankingId":42},"openBankingAccounts":[{}],"registrableAccounts":[{},{}],"savingCount":3}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetOpenBankingStatus(context.Background())
	if err != nil {
		t.Fatalf("GetOpenBankingStatus: %v", err)
	}
	if got.HolderName != "홍길동" || got.ConnectedAccount == nil || got.ConnectedAccount.AccountNo != "123-456-789" || got.ConnectedAccount.BankCode != "088" {
		t.Fatalf("status = %#v", got)
	}
	if got.LinkedAccountCount != 1 || got.RegistrableAccountCount != 2 || got.SavingCount != 3 {
		t.Fatalf("counts = %#v", got)
	}
}

func TestGetOpenBankingStatusAllowsDisconnectedAccount(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/autotrade/open-banking/info/find" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"name":"","connectedOpenBankingAccount":null,"openBankingAccounts":[],"registrableAccounts":[],"savingCount":0}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetOpenBankingStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.ConnectedAccount != nil || got.LinkedAccountCount != 0 || got.RegistrableAccountCount != 0 {
		t.Fatalf("disconnected status = %#v", got)
	}
}

func TestGetNotificationSettingsOmitsInternalUserID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user-alimies" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"id":1,"userId":999,"type":"CALENDAR_AI_SUMMARY_WEEKLY","enabled":true,"createdAt":"2026-01-01","updatedAt":"2026-09-01"},{"id":2,"userId":999,"type":null,"enabled":false}]}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("GetNotificationSettings: %v", err)
	}
	if len(got.Settings) != 2 || got.Settings[0].Type != "CALENDAR_AI_SUMMARY_WEEKLY" || !got.Settings[0].Enabled || got.Settings[1].Type != "" {
		t.Fatalf("settings = %#v", got.Settings)
	}
}
