package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// investModeFor returns the ticks viewType/investMode pair for a product code.
// KR codes use the KRX combined view; everything else falls back to the US view.
func investModeFor(productCode string) (viewType, investMode string) {
	if deriveSecurityType(productCode) == "kr-s" {
		return "krx_all", "krx"
	}
	return "unified", "unified"
}

type tradeRaw struct {
	Time             string  `json:"time"`
	Price            float64 `json:"price"`
	Base             float64 `json:"base"`
	Volume           float64 `json:"volume"`
	TradeType        string  `json:"tradeType"`
	CumulativeVolume float64 `json:"cumulativeVolume"`
}

// GetTrades returns the most recent executed ticks (체결) for a symbol.
func (c *Client) GetTrades(ctx context.Context, symbol string, count int) (domain.TradeList, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.TradeList{}, err
	}
	info, _ := c.getStockInfo(ctx, productCode)

	if count <= 0 {
		count = 30
	}
	viewType, investMode := investModeFor(productCode)

	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v2/stock-prices/%s/ticks", c.infoBaseURL, productCode))
	if err != nil {
		return domain.TradeList{}, err
	}
	q := endpoint.Query()
	q.Set("viewType", viewType)
	q.Set("investMode", investMode)
	q.Set("count", strconv.Itoa(count))
	endpoint.RawQuery = q.Encode()

	var envelope quoteEnvelope[[]tradeRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.TradeList{}, err
	}

	out := domain.TradeList{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		FetchedAt:   time.Now().UTC(),
	}
	out.Trades = make([]domain.Trade, 0, len(envelope.Result))
	for _, r := range envelope.Result {
		out.Trades = append(out.Trades, domain.Trade{
			Time:             r.Time,
			Price:            r.Price,
			Base:             r.Base,
			Volume:           r.Volume,
			TradeType:        r.TradeType,
			CumulativeVolume: r.CumulativeVolume,
		})
	}
	return out, nil
}

// upper-lower returns a bare object (no BFF envelope).
type upperLowerRaw struct {
	Date       string  `json:"date"`
	UpperLimit float64 `json:"upperLimit"`
	LowerLimit float64 `json:"lowerLimit"`
}

// GetPriceLimits returns the daily upper/lower price band (상/하한가). This is a
// KRX-specific concept; US markets use intraday circuit breakers (LULD) instead
// of a fixed daily band, so non-KR symbols are rejected with a clear message.
func (c *Client) GetPriceLimits(ctx context.Context, symbol string) (domain.PriceLimits, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.PriceLimits{}, err
	}
	if deriveSecurityType(productCode) != "kr-s" {
		return domain.PriceLimits{}, fmt.Errorf("상/하한가는 국내(KRX) 종목만 제공됩니다 (미국장은 일일 가격제한 제도가 없음): %s", symbol)
	}
	info, _ := c.getStockInfo(ctx, productCode)

	var envelope quoteEnvelope[upperLowerRaw]
	endpoint := fmt.Sprintf("%s/api/v2/stock-prices/%s/upper-lower", c.infoBaseURL, productCode)
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.PriceLimits{}, err
	}

	return domain.PriceLimits{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		Date:        envelope.Result.Date,
		UpperLimit:  envelope.Result.UpperLimit,
		LowerLimit:  envelope.Result.LowerLimit,
	}, nil
}

// GetStockWarnings returns buy-caution badges (매수 유의사항). Badge shape is
// dynamic; recognized fields are mapped and the full object is kept as Raw.
func (c *Client) GetStockWarnings(ctx context.Context, symbol string) (domain.StockWarnings, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.StockWarnings{}, err
	}
	info, _ := c.getStockInfo(ctx, productCode)

	var envelope quoteEnvelope[[]json.RawMessage]
	endpoint := fmt.Sprintf("%s/api/v1/stock-infos/%s/wts-badges", c.infoBaseURL, productCode)
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.StockWarnings{}, err
	}

	out := domain.StockWarnings{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		FetchedAt:   time.Now().UTC(),
	}
	out.Warnings = make([]domain.StockWarning, 0, len(envelope.Result))
	for _, raw := range envelope.Result {
		var fields struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			Name  string `json:"name"`
			Text  string `json:"text"`
			Desc  string `json:"description"`
			Level string `json:"level"`
		}
		_ = json.Unmarshal(raw, &fields)
		title := fields.Title
		if title == "" {
			title = fields.Name
		}
		text := fields.Text
		if text == "" {
			text = fields.Desc
		}
		out.Warnings = append(out.Warnings, domain.StockWarning{
			Type:  fields.Type,
			Title: title,
			Text:  text,
			Level: fields.Level,
			Raw:   raw,
		})
	}
	return out, nil
}

// trading-hours/integrated session block.
type tradingSessionRaw struct {
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type tradingMarketRaw struct {
	Today      tradingSessionRaw `json:"today"`
	NextBizDay tradingSessionRaw `json:"nextBizDay"`
}

type tradingHoursRaw struct {
	KR tradingMarketRaw `json:"kr"`
	US tradingMarketRaw `json:"us"`
}

type exchangeRatesRaw struct {
	ExchangeRates []struct {
		Code  string  `json:"code"`
		Name  string  `json:"name"`
		Base  float64 `json:"base"`
		Close float64 `json:"close"`
	} `json:"exchangeRates"`
}

// GetExchangeRates returns FX/index quotes (USD/KRW, DXY 등).
func (c *Client) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	var envelope quoteEnvelope[exchangeRatesRaw]
	endpoint := c.infoBaseURL + "/api/v1/dashboard/wts/overview/exchange-rates"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.ExchangeRates{}, err
	}
	out := domain.ExchangeRates{FetchedAt: time.Now().UTC()}
	out.Rates = make([]domain.ExchangeRate, 0, len(envelope.Result.ExchangeRates))
	for _, r := range envelope.Result.ExchangeRates {
		out.Rates = append(out.Rates, domain.ExchangeRate{Code: r.Code, Name: r.Name, Base: r.Base, Close: r.Close})
	}
	return out, nil
}

type screenerPresetRaw struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Filters     json.RawMessage `json:"filters"`
}

// GetScreenerPresets returns predefined stock screens (조건검색 프리셋).
func (c *Client) GetScreenerPresets(ctx context.Context) (domain.ScreenerPresets, error) {
	var envelope quoteEnvelope[[]screenerPresetRaw]
	endpoint := c.certBaseURL + "/api/v2/screener/presets/common?useCustom=true"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.ScreenerPresets{}, err
	}
	out := domain.ScreenerPresets{FetchedAt: time.Now().UTC()}
	for _, p := range envelope.Result {
		out.Presets = append(out.Presets, domain.ScreenerPreset{ID: p.ID, Name: p.Name, Description: p.Description, Filters: p.Filters})
	}
	return out, nil
}

type screenerResultRaw struct {
	TotalCount int `json:"totalCount"`
	Stocks     []struct {
		StockCode string `json:"stockCode"`
		Name      string `json:"name"`
		Base      struct {
			KRW float64 `json:"krw"`
			USD float64 `json:"usd"`
		} `json:"base"`
		Close struct {
			KRW float64 `json:"krw"`
			USD float64 `json:"usd"`
		} `json:"close"`
	} `json:"stocks"`
}

// RunScreener executes a preset screen by id and returns matching stocks.
// nation: "kr" | "us". 공식 API 에 없는 조건검색 표면.
func (c *Client) RunScreener(ctx context.Context, presetID, nation string, size int) (domain.ScreenerResult, error) {
	// 프리셋 정의(filters)를 받아온다.
	var presetEnv quoteEnvelope[[]screenerPresetRaw]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v2/screener/presets/common?useCustom=true", &presetEnv); err != nil {
		return domain.ScreenerResult{}, err
	}
	var preset *screenerPresetRaw
	for i := range presetEnv.Result {
		if presetEnv.Result[i].ID == presetID {
			preset = &presetEnv.Result[i]
			break
		}
	}
	if preset == nil {
		return domain.ScreenerResult{}, fmt.Errorf("screener preset %q not found (run `market screener` to list)", presetID)
	}
	res, err := c.runScreenerFilters(ctx, preset.Filters, nation, size)
	if err != nil {
		return domain.ScreenerResult{}, err
	}
	res.PresetID = presetID
	res.PresetName = preset.Name
	return res, nil
}

// RunScreenerRaw executes a screen from a caller-supplied raw filter JSON array
// (조건검색 custom). 필터 카탈로그는 토스 web 의 JS 번들에 한글 ID 로 내장돼 있어
// 우리가 typed 로 복제하지 않고 raw passthrough 한다 — agent/파워유저가 직접 구성.
func (c *Client) RunScreenerRaw(ctx context.Context, filtersJSON, nation string, size int) (domain.ScreenerResult, error) {
	raw := json.RawMessage(strings.TrimSpace(filtersJSON))
	if !json.Valid(raw) {
		return domain.ScreenerResult{}, fmt.Errorf("--filter 는 유효한 JSON 배열이어야 합니다")
	}
	return c.runScreenerFilters(ctx, raw, nation, size)
}

// runScreenerFilters POSTs a filters array and maps the result.
func (c *Client) runScreenerFilters(ctx context.Context, filters json.RawMessage, nation string, size int) (domain.ScreenerResult, error) {
	if nation == "" {
		nation = "kr"
	}
	if size <= 0 {
		size = 30
	}
	usd := nation == "us"
	body, err := json.Marshal(map[string]any{
		"pagingParam": map[string]any{"key": nil, "number": 1, "size": size},
		"filters":     filters,
		"nation":      nation,
	})
	if err != nil {
		return domain.ScreenerResult{}, err
	}

	var env quoteEnvelope[screenerResultRaw]
	if err := c.postJSON(ctx, c.certBaseURL+"/api/v2/screener/screen", body, &env); err != nil {
		return domain.ScreenerResult{}, err
	}

	out := domain.ScreenerResult{
		Nation:     nation,
		TotalCount: env.Result.TotalCount,
		FetchedAt:  time.Now().UTC(),
	}
	for _, s := range env.Result.Stocks {
		base, close := s.Base.KRW, s.Close.KRW
		if usd {
			base, close = s.Base.USD, s.Close.USD
		}
		st := domain.ScreenedStock{ProductCode: s.StockCode, Name: s.Name, Close: close}
		st.Change = close - base
		if base != 0 {
			st.ChangeRate = st.Change / base
		}
		out.Stocks = append(out.Stocks, st)
	}
	return out, nil
}

type aiSignalRaw struct {
	Label string `json:"label"`
	Data  []struct {
		AssetType         string `json:"assetType"`
		AssetName         string `json:"assetName"`
		Title             string `json:"title"`
		Keyword           string `json:"keyword"`
		FluctuationPhrase string `json:"fluctuationPhrase"`
		Stocks            []struct {
			StockCode string `json:"stockCode"`
		} `json:"stocks"`
	} `json:"data"`
}

// GetAISignals returns Toss's AI market signal feed (토스증권 AI 시그널).
// 공식 API 에 없는 web 전용 표면.
func (c *Client) GetAISignals(ctx context.Context) (domain.AISignals, error) {
	var envelope quoteEnvelope[aiSignalRaw]
	endpoint := c.infoBaseURL + "/api/v2/reasoning-contents/interest"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.AISignals{}, err
	}
	out := domain.AISignals{Label: envelope.Result.Label, FetchedAt: time.Now().UTC()}
	out.Signals = make([]domain.AISignal, 0, len(envelope.Result.Data))
	for _, d := range envelope.Result.Data {
		sig := domain.AISignal{
			AssetName:   d.AssetName,
			AssetType:   d.AssetType,
			Title:       d.Title,
			Keyword:     d.Keyword,
			Fluctuation: d.FluctuationPhrase,
		}
		if len(d.Stocks) > 0 {
			sig.StockCode = d.Stocks[0].StockCode
		}
		out.Signals = append(out.Signals, sig)
	}
	return out, nil
}

type tradingTrendRaw struct {
	Body []struct {
		BaseDate                string  `json:"baseDate"`
		NetIndividualsBuyVolume float64 `json:"netIndividualsBuyVolume"`
		NetForeignerBuyVolume   float64 `json:"netForeignerBuyVolume"`
		NetInstitutionBuyVolume float64 `json:"netInstitutionBuyVolume"`
	} `json:"body"`
}

// GetTradingFlows returns investor-type net flows (수급 — 개인·외국인·기관 순매수).
// KRX 전용 (해외 종목은 데이터 없음). 공식 API 에 없는 web 전용 표면.
func (c *Client) GetTradingFlows(ctx context.Context, symbol string, size int) (domain.TradingFlows, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.TradingFlows{}, err
	}
	if deriveSecurityType(productCode) != "kr-s" {
		return domain.TradingFlows{}, fmt.Errorf("수급(투자자별 순매수)은 국내(KRX) 종목만 제공됩니다: %s", symbol)
	}
	info, _ := c.getStockInfo(ctx, productCode)
	if size <= 0 {
		size = 20
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/stock-infos/trade/trend/trading-trend")
	if err != nil {
		return domain.TradingFlows{}, err
	}
	q := endpoint.Query()
	q.Set("productCode", productCode)
	q.Set("size", strconv.Itoa(size))
	endpoint.RawQuery = q.Encode()

	var envelope quoteEnvelope[tradingTrendRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.TradingFlows{}, err
	}
	out := domain.TradingFlows{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		FetchedAt:   time.Now().UTC(),
	}
	out.Flows = make([]domain.TradingFlow, 0, len(envelope.Result.Body))
	for _, r := range envelope.Result.Body {
		out.Flows = append(out.Flows, domain.TradingFlow{
			Date:           r.BaseDate,
			NetIndividuals: r.NetIndividualsBuyVolume,
			NetForeigner:   r.NetForeignerBuyVolume,
			NetInstitution: r.NetInstitutionBuyVolume,
		})
	}
	return out, nil
}

type marketIndexRaw struct {
	MajorIndicatorInfos []struct {
		Code        string `json:"code"`
		DisplayName string `json:"displayName"`
		Nation      string `json:"nation"`
		Price       struct {
			LatestPrice float64 `json:"latestPrice"`
			BasePrice   float64 `json:"basePrice"`
		} `json:"price"`
	} `json:"majorIndicatorInfos"`
}

// GetMarketIndices returns major market index quotes (코스피·나스닥·VIX 등).
// 공식 API 에 없는 web 전용 기능.
func (c *Client) GetMarketIndices(ctx context.Context) (domain.MarketIndices, error) {
	var envelope quoteEnvelope[marketIndexRaw]
	endpoint := c.certBaseURL + "/api/v1/dashboard/wts/overview/indicator/index"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.MarketIndices{}, err
	}
	out := domain.MarketIndices{FetchedAt: time.Now().UTC()}
	out.Indices = make([]domain.MarketIndex, 0, len(envelope.Result.MajorIndicatorInfos))
	for _, r := range envelope.Result.MajorIndicatorInfos {
		idx := domain.MarketIndex{
			Code:   r.Code,
			Name:   r.DisplayName,
			Nation: r.Nation,
			Latest: r.Price.LatestPrice,
			Base:   r.Price.BasePrice,
		}
		idx.Change = idx.Latest - idx.Base
		if idx.Base != 0 {
			idx.ChangeRate = idx.Change / idx.Base
		}
		out.Indices = append(out.Indices, idx)
	}
	return out, nil
}

// indexAliases maps friendly English/Korean names to index display names so a
// caller can pass "nasdaq" / "코스피" instead of the raw code.
var indexAliases = map[string]string{
	"nasdaq": "나스닥", "kospi": "코스피", "kosdaq": "코스닥",
	"sp500": "S&P 500", "s&p500": "S&P 500", "snp": "S&P 500",
	"sox": "필라델피아 반도체", "vix": "VIX",
}

// GetIndexDetail returns a detailed quote for one index. query may be an index
// code (COMP.NAI), a display name (나스닥), or an alias (nasdaq). 공식 API 에
// 없는 web 전용 기능.
func (c *Client) GetIndexDetail(ctx context.Context, query string) (domain.IndexQuote, error) {
	indices, err := c.GetMarketIndices(ctx)
	if err != nil {
		return domain.IndexQuote{}, err
	}
	want := strings.TrimSpace(query)
	if alias, ok := indexAliases[strings.ToLower(want)]; ok {
		want = alias
	}
	var matched *domain.MarketIndex
	for i := range indices.Indices {
		idx := &indices.Indices[i]
		if strings.EqualFold(idx.Code, want) || strings.EqualFold(idx.Name, want) || strings.Contains(idx.Name, want) {
			matched = idx
			break
		}
	}
	if matched == nil {
		names := make([]string, 0, len(indices.Indices))
		for _, idx := range indices.Indices {
			names = append(names, idx.Name)
		}
		return domain.IndexQuote{}, fmt.Errorf("index %q not found (available: %s)", query, strings.Join(names, ", "))
	}

	var envelope quoteEnvelope[struct {
		Open    float64 `json:"open"`
		High    float64 `json:"high"`
		Low     float64 `json:"low"`
		Close   float64 `json:"close"`
		Volume  float64 `json:"volume"`
		Base    float64 `json:"base"`
		High52w float64 `json:"high52w"`
		Low52w  float64 `json:"low52w"`
	}]
	endpoint := fmt.Sprintf("%s/api/v1/index-prices/%s", c.infoBaseURL, url.PathEscape(matched.Code))
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.IndexQuote{}, err
	}
	r := envelope.Result
	out := domain.IndexQuote{
		Code: matched.Code, Name: matched.Name, Nation: matched.Nation,
		Open: r.Open, High: r.High, Low: r.Low, Close: r.Close, Base: r.Base,
		Volume: r.Volume, High52w: r.High52w, Low52w: r.Low52w,
		FetchedAt: time.Now().UTC(),
	}
	out.Change = out.Close - out.Base
	if out.Base != 0 {
		out.ChangeRate = out.Change / out.Base
	}
	return out, nil
}

type rankingRaw struct {
	Data []struct {
		Code   string `json:"code"`
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
		Market struct {
			DisplayName string `json:"displayName"`
		} `json:"market"`
	} `json:"data"`
}

// GetStockRanking returns the realtime popularity ranking (실시간 인기 순위).
// 공식 API 에 없는 discovery 표면.
func (c *Client) GetStockRanking(ctx context.Context, size int) (domain.StockRanking, error) {
	if size <= 0 {
		size = 20
	}
	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/rankings/realtime/stock")
	if err != nil {
		return domain.StockRanking{}, err
	}
	q := endpoint.Query()
	q.Set("size", strconv.Itoa(size))
	endpoint.RawQuery = q.Encode()

	var envelope quoteEnvelope[rankingRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.StockRanking{}, err
	}
	out := domain.StockRanking{FetchedAt: time.Now().UTC()}
	out.Stocks = make([]domain.RankedStock, 0, len(envelope.Result.Data))
	for i, r := range envelope.Result.Data {
		out.Stocks = append(out.Stocks, domain.RankedStock{
			Rank:        i + 1,
			ProductCode: r.Code,
			Symbol:      r.Symbol,
			Name:        r.Name,
			Market:      r.Market.DisplayName,
		})
	}
	return out, nil
}

// GetTradingHours returns today's KR and US session windows (장 운영 시간).
func (c *Client) GetTradingHours(ctx context.Context) (domain.TradingHours, error) {
	var envelope quoteEnvelope[tradingHoursRaw]
	endpoint := c.apiBaseURL + "/api/v2/system/trading-hours/integrated"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.TradingHours{}, err
	}
	raw := envelope.Result
	sess := func(s tradingSessionRaw) domain.MarketSession {
		return domain.MarketSession{Date: s.Date, StartTime: s.StartTime, EndTime: s.EndTime}
	}
	return domain.TradingHours{
		KR:        sess(raw.KR.Today),
		US:        sess(raw.US.Today),
		NextKR:    sess(raw.KR.NextBizDay),
		NextUS:    sess(raw.US.NextBizDay),
		FetchedAt: time.Now().UTC(),
	}, nil
}

// GetOrderBook returns the bid/ask depth ladder (호가) for a symbol via the
// web v3 quotes endpoint. Offer = ask (매도), Bid = buy (매수), best-first.
func (c *Client) GetOrderBook(ctx context.Context, symbol string) (domain.OrderBook, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.OrderBook{}, err
	}
	info, _ := c.getStockInfo(ctx, productCode)

	var envelope quoteEnvelope[struct {
		Close        float64   `json:"close"`
		OfferPrices  []float64 `json:"offerPrices"`
		OfferVolumes []float64 `json:"offerVolumes"`
		BidPrices    []float64 `json:"bidPrices"`
		BidVolumes   []float64 `json:"bidVolumes"`
		OfferVolume  float64   `json:"offerVolume"`
		BidVolume    float64   `json:"bidVolume"`
	}]
	endpoint := fmt.Sprintf("%s/api/v3/stock-prices/%s/quotes", c.infoBaseURL, productCode)
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.OrderBook{}, err
	}

	r := envelope.Result
	out := domain.OrderBook{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		Close:       r.Close,
		TotalOffer:  r.OfferVolume,
		TotalBid:    r.BidVolume,
		FetchedAt:   time.Now().UTC(),
	}
	out.Offers = zipOrderBookLevels(r.OfferPrices, r.OfferVolumes)
	out.Bids = zipOrderBookLevels(r.BidPrices, r.BidVolumes)
	return out, nil
}

func zipOrderBookLevels(prices, volumes []float64) []domain.OrderBookLevel {
	levels := make([]domain.OrderBookLevel, 0, len(prices))
	for i, p := range prices {
		var v float64
		if i < len(volumes) {
			v = volumes[i]
		}
		if p == 0 && v == 0 {
			continue
		}
		levels = append(levels, domain.OrderBookLevel{Price: p, Volume: v})
	}
	return levels
}

// GetSellableQuantity returns how many shares of a held symbol can be sold now.
func (c *Client) GetSellableQuantity(ctx context.Context, symbol string) (domain.SellableQuantity, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.SellableQuantity{}, err
	}
	info, _ := c.getStockInfo(ctx, productCode)

	var envelope quoteEnvelope[float64]
	endpoint := fmt.Sprintf("%s/api/v1/trading/orders/calculate/%s/orderable-quantity/sell?forceFetch=false", c.certBaseURL, productCode)
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.SellableQuantity{}, err
	}
	return domain.SellableQuantity{
		ProductCode: productCode,
		Symbol:      info.Symbol,
		Name:        info.Name,
		Quantity:    envelope.Result,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// GetCommission returns the commission and tax rate applied to a symbol's trades.
func (c *Client) GetCommission(ctx context.Context, symbol string) (domain.Commission, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.Commission{}, err
	}
	info, _ := c.getStockInfo(ctx, productCode)

	var envelope quoteEnvelope[struct {
		CommissionRate float64 `json:"commissionRate"`
		TaxRate        float64 `json:"taxRate"`
	}]
	endpoint := fmt.Sprintf("%s/api/v2/trading/orders/calculate/%s/cost-basis-elements", c.certBaseURL, productCode)
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.Commission{}, err
	}
	return domain.Commission{
		ProductCode:    productCode,
		Symbol:         info.Symbol,
		Name:           info.Name,
		CommissionRate: envelope.Result.CommissionRate,
		TaxRate:        envelope.Result.TaxRate,
		FetchedAt:      time.Now().UTC(),
	}, nil
}

// GetInvestorRankings returns top net-buy stocks per investor type
// (외국인·개인·기관) — market-wide flow discovery.
func (c *Client) GetInvestorRankings(ctx context.Context, size int) (domain.InvestorRankings, error) {
	if size <= 0 {
		size = 10
	}
	var envelope quoteEnvelope[struct {
		Rankings map[string]struct {
			Type    string `json:"type"`
			BasedAt string `json:"basedAt"`
			Buy     []struct {
				StockCode string  `json:"stockCode"`
				Name      string  `json:"name"`
				Amount    float64 `json:"amount"`
				Base      float64 `json:"base"`
				Close     float64 `json:"close"`
			} `json:"buyStocks"`
		} `json:"rankings"`
	}]
	endpoint := c.infoBaseURL + "/api/v1/dashboard/wts/overview/rankings/by-investors"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.InvestorRankings{}, err
	}

	out := domain.InvestorRankings{FetchedAt: time.Now().UTC()}
	// Stable order: foreigner, institution, individual.
	for _, key := range []string{"foreigner", "institution", "individual"} {
		r, ok := envelope.Result.Rankings[key]
		if !ok {
			continue
		}
		ir := domain.InvestorRanking{InvestorType: r.Type, BasedAt: r.BasedAt}
		for i, s := range r.Buy {
			if i >= size {
				break
			}
			ir.Stocks = append(ir.Stocks, domain.InvestorRankedStock{
				Rank:         i + 1,
				ProductCode:  s.StockCode,
				Name:         s.Name,
				NetBuyAmount: s.Amount,
				Base:         s.Base,
				Close:        s.Close,
			})
		}
		out.Rankings = append(out.Rankings, ir)
	}
	return out, nil
}

type ticsItemRaw struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	CompanyCount int    `json:"companyCount"`
	Fluctuations struct {
		OneDayRate     float64 `json:"oneDayRate"`
		OneMonthRate   float64 `json:"oneMonthRate"`
		ThreeMonthRate float64 `json:"threeMonthsRate"`
		OneYearRate    float64 `json:"oneYearRate"`
	} `json:"fluctuations"`
	SubItems []ticsItemRaw `json:"subItems"`
}

type ticsRankItemRaw struct {
	TicsID           string `json:"ticsId"`
	Title            string `json:"title"`
	PreciseValue     string `json:"preciseValue"`
	Ranking          int    `json:"ranking"`
	TotalCount       int    `json:"totalCount"`
	RiseCompanyCount int    `json:"riseCompanyCount"`
	PriceValue       string `json:"priceValue"`
}

type ticsRankingRaw struct {
	Name     string            `json:"name"`
	DateTime string            `json:"dateTime"`
	Data     []ticsRankItemRaw `json:"data"`
}

// GetThemeRankings returns TICS themes ranked by today's fluctuation
// (오늘의 테마 등락 순위). Maps to GET /api/v1/tics/rankings (wts-info-api).
// size <= 0 returns the full list; otherwise the top `size` by ranking.
func (c *Client) GetThemeRankings(ctx context.Context, size int) (domain.ThemeRankings, error) {
	var env quoteEnvelope[ticsRankingRaw]
	if err := c.getJSON(ctx, c.infoBaseURL+"/api/v1/tics/rankings", &env); err != nil {
		return domain.ThemeRankings{}, err
	}
	out := domain.ThemeRankings{
		Name:      env.Result.Name,
		DateTime:  env.Result.DateTime,
		FetchedAt: time.Now().UTC(),
	}
	for _, it := range env.Result.Data {
		out.Items = append(out.Items, domain.ThemeRanking{
			Ranking:          it.Ranking,
			TicsID:           it.TicsID,
			Title:            it.Title,
			ChangeRate:       parsePercent(it.PreciseValue),
			RiseCompanyCount: it.RiseCompanyCount,
			TotalCount:       it.TotalCount,
			Summary:          it.PriceValue,
		})
	}
	if size > 0 && len(out.Items) > size {
		out.Items = out.Items[:size]
	}
	return out, nil
}

// parsePercent parses a Toss percent string like "+11.18%", "-3.2%", "0.0%"
// into a float (11.18, -3.2, 0). Returns 0 for unparseable input.
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimPrefix(s, "+")
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func ticsToSector(t ticsItemRaw) domain.Sector {
	s := domain.Sector{
		ID:             t.ID,
		Title:          t.Title,
		CompanyCount:   t.CompanyCount,
		OneDayRate:     t.Fluctuations.OneDayRate,
		OneMonthRate:   t.Fluctuations.OneMonthRate,
		ThreeMonthRate: t.Fluctuations.ThreeMonthRate,
		OneYearRate:    t.Fluctuations.OneYearRate,
	}
	for _, sub := range t.SubItems {
		s.SubSectors = append(s.SubSectors, ticsToSector(sub))
	}
	return s
}

// GetSectors returns the industry (TICS) tree with fluctuation rates
// (업종별 등락 — 1일·1개월·3개월·1년). 공식 API 에 없는 web 전용 기능.
func (c *Client) GetSectors(ctx context.Context) (domain.Sectors, error) {
	var envelope quoteEnvelope[struct {
		TicsItems []ticsItemRaw `json:"ticsItems"`
	}]
	endpoint := c.infoBaseURL + "/api/v1/tics/all"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.Sectors{}, err
	}
	out := domain.Sectors{FetchedAt: time.Now().UTC()}
	for _, t := range envelope.Result.TicsItems {
		out.Items = append(out.Items, ticsToSector(t))
	}
	return out, nil
}

// GetEarningCallHome returns the curated major-company earnings-call lineup
// (어닝콜 홈 — 주요 기업). 공식 API 에 없는 web 전용 기능.
func (c *Client) GetEarningCallHome(ctx context.Context) (domain.EarningCalls, error) {
	var envelope quoteEnvelope[struct {
		MajorCompanies struct {
			CurrentOrFuture []struct {
				EventID     int64  `json:"eventId"`
				Title       string `json:"eventTitle"`
				Status      string `json:"status"`
				LiveAt      string `json:"liveAt"`
				CompanyCode string `json:"companyCode"`
				CompanyName string `json:"companyName"`
				Sub         string `json:"subContentText"`
			} `json:"currentOrFuture"`
		} `json:"majorCompanies"`
	}]
	endpoint := c.infoBaseURL + "/api/v1/earning-call/home"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.EarningCalls{}, err
	}
	out := domain.EarningCalls{FetchedAt: time.Now().UTC()}
	for _, e := range envelope.Result.MajorCompanies.CurrentOrFuture {
		out.Events = append(out.Events, domain.EarningCall{
			EventID:     e.EventID,
			Title:       e.Title,
			Status:      e.Status,
			LiveAt:      e.LiveAt,
			CompanyCode: e.CompanyCode,
			CompanyName: e.CompanyName,
			Category:    e.Sub,
		})
	}
	return out, nil
}

// GetNewsBriefing returns the personalized AI news briefing grouped by theme
// and enriched with the holding/watchlist context behind each signal. v2 adds
// asset, return, direction, and reasoning title while preserving the v1 news
// fields consumed by existing callers. 공식 API 에 없는 web 전용 기능.
func (c *Client) GetNewsBriefing(ctx context.Context) (domain.NewsBriefing, error) {
	var envelope quoteEnvelope[struct {
		CreatedAt string `json:"createdAt"`
		Items     []struct {
			Section   string `json:"section"`
			SignalID  string `json:"signalId"`
			TraceID   string `json:"traceId"`
			CreatedAt string `json:"createdAt"`
			Category  struct {
				Keywords []string `json:"keywords"`
				Type     string   `json:"type"`
			} `json:"category"`
			ReasoningSummary struct {
				AssetInfo struct {
					Code         string `json:"code"`
					Name         string `json:"name"`
					LogoImageURL string `json:"logoImageUrl"`
				} `json:"assetInfo"`
				AssetType       string  `json:"assetType"`
				InvestmentType  string  `json:"investmentType"`
				ProfitLossRate  float64 `json:"profitLossRate"`
				ReasoningTitle  string  `json:"reasoningTitle"`
				SignalDirection int     `json:"signalDirection"`
			} `json:"reasoningSummary"`
			News []struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				AgencyName string `json:"agencyName"`
				Source     string `json:"source"`
				FaviconURL string `json:"faviconUrl"`
				CreatedAt  string `json:"createdAt"`
			} `json:"news"`
			RelatedStocks []relatedStockRaw `json:"relatedStocks"`
		} `json:"items"`
	}]
	endpoint := c.certBaseURL + "/api/v2/reasoning/personalized"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.NewsBriefing{}, err
	}
	out := domain.NewsBriefing{CreatedAt: envelope.Result.CreatedAt, FetchedAt: time.Now().UTC()}
	for _, it := range envelope.Result.Items {
		bi := domain.BriefingItem{
			CategoryType:      it.Category.Type,
			Keywords:          it.Category.Keywords,
			Section:           it.Section,
			SignalID:          it.SignalID,
			TraceID:           it.TraceID,
			CreatedAt:         it.CreatedAt,
			AssetCode:         it.ReasoningSummary.AssetInfo.Code,
			AssetName:         it.ReasoningSummary.AssetInfo.Name,
			AssetLogoImageURL: it.ReasoningSummary.AssetInfo.LogoImageURL,
			AssetType:         it.ReasoningSummary.AssetType,
			InvestmentType:    it.ReasoningSummary.InvestmentType,
			ProfitLossRate:    it.ReasoningSummary.ProfitLossRate,
			ReasoningTitle:    it.ReasoningSummary.ReasoningTitle,
			SignalDirection:   it.ReasoningSummary.SignalDirection,
			RelatedStocks:     mapRelatedStocks(it.RelatedStocks),
		}
		for _, n := range it.News {
			bi.News = append(bi.News, domain.BriefingNews{
				ID:         n.ID,
				Title:      n.Title,
				Agency:     n.AgencyName,
				Source:     n.Source,
				FaviconURL: n.FaviconURL,
				CreatedAt:  n.CreatedAt,
			})
		}
		out.Items = append(out.Items, bi)
	}
	return out, nil
}

// GetEarningCalls returns the upcoming earnings-call (어닝콜) calendar.
func (c *Client) GetEarningCalls(ctx context.Context) (domain.EarningCalls, error) {
	var envelope quoteEnvelope[[]struct {
		EventID     int64  `json:"eventId"`
		Title       string `json:"eventTitle"`
		Status      string `json:"status"`
		LiveAt      string `json:"liveAt"`
		CompanyCode string `json:"companyCode"`
		CompanyName string `json:"companyName"`
		Sub         string `json:"subContentText"`
	}]
	endpoint := c.infoBaseURL + "/api/v1/earning-call/upcoming"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.EarningCalls{}, err
	}
	out := domain.EarningCalls{FetchedAt: time.Now().UTC()}
	for _, e := range envelope.Result {
		out.Events = append(out.Events, domain.EarningCall{
			EventID:     e.EventID,
			Title:       e.Title,
			Status:      e.Status,
			LiveAt:      e.LiveAt,
			CompanyCode: e.CompanyCode,
			CompanyName: e.CompanyName,
			Category:    e.Sub,
		})
	}
	return out, nil
}

// haltMarketNames maps Toss's market code to a readable name. An unmapped code
// keeps its raw form rather than becoming blank.
var haltMarketNames = map[string]string{"KSP": "KOSPI", "KSQ": "KOSDAQ"}

// haltTypes maps the server enum to a stable alias. Unmapped types keep the raw
// server string so a halt type Toss adds later is still reported.
var haltTypes = map[string]string{
	"CIRCUIT_BREAKER": "circuit_breaker",
	"SIDECAR":         "sidecar",
}

// GetMarketHalt returns the current 서킷브레이커·사이드카 state for KR markets.
//
// Endpoint: GET /api/v4/dashboard/wts/overview/indicator (cert host). The
// response is the whole market-overview widget (~40KB); only `marketEvents` is
// decoded. There is a dedicated 33-byte endpoint
// (/api/v1/dashboard/common/market-event/circuit-breaker) but it lists only
// breakers that are *firing* — it cannot answer "is the market normal?", and it
// says nothing about sidecars. See docs/reverse-engineering/wts-endpoints.json.
//
// ponytail: the oversized payload is fetched and mostly discarded rather than
// combining two endpoints. Switch to the dedicated endpoint for detail (level,
// start/end time, index snapshot) once a live firing is observed — the field
// names are known from the web bundle but the item shape has never been seen
// populated.
func (c *Client) GetMarketHalt(ctx context.Context) (domain.MarketHalt, error) {
	var envelope quoteEnvelope[struct {
		MarketEvents []struct {
			Market    string `json:"market"`
			Type      string `json:"type"`
			Activated bool   `json:"activated"`
		} `json:"marketEvents"`
	}]
	endpoint := c.certBaseURL + "/api/v4/dashboard/wts/overview/indicator"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.MarketHalt{}, err
	}
	out := domain.MarketHalt{FetchedAt: time.Now().UTC()}
	for _, e := range envelope.Result.MarketEvents {
		name, ok := haltMarketNames[e.Market]
		if !ok {
			name = e.Market
		}
		typ, ok := haltTypes[e.Type]
		if !ok {
			typ = e.Type
		}
		out.Events = append(out.Events, domain.MarketHaltEvent{
			Market:     e.Market,
			MarketName: name,
			Type:       typ,
			Activated:  e.Activated,
		})
	}
	return out, nil
}

// GetIndexAnomalies returns the indices Toss has badged as moving unusually,
// with the AI signal attached to each.
//
// Endpoint: GET /api/v4/dashboard/wts/overview/indicator (cert host) — the same
// widget GetMarketHalt reads. Only `badgedIndices` is decoded here. Kept as a
// separate call rather than one combined fetch because the two answer unrelated
// questions ("is trading stopped?" vs "which index is moving oddly?") and a
// caller wanting one should not pay for the other's rendering.
func (c *Client) GetIndexAnomalies(ctx context.Context) (domain.IndexAnomalies, error) {
	var envelope quoteEnvelope[struct {
		BadgedIndices []struct {
			IndexCode   string  `json:"indexCode"`
			DisplayName string  `json:"displayName"`
			Category    string  `json:"category"`
			Direction   string  `json:"direction"`
			IsAnomaly   bool    `json:"isAnomaly"`
			ChangeRate  float64 `json:"changeRate"`
			SignalTitle string  `json:"aiSignalTitle"`
			SignalID    string  `json:"aiSignalId"`
			Keyword     string  `json:"keyword"`
			ZScore      float64 `json:"zscore"`
		} `json:"badgedIndices"`
	}]
	endpoint := c.certBaseURL + "/api/v4/dashboard/wts/overview/indicator"
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.IndexAnomalies{}, err
	}
	out := domain.IndexAnomalies{FetchedAt: time.Now().UTC()}
	for _, b := range envelope.Result.BadgedIndices {
		out.Indices = append(out.Indices, domain.IndexAnomaly{
			IndexCode:   b.IndexCode,
			DisplayName: b.DisplayName,
			Category:    b.Category,
			Direction:   b.Direction,
			IsAnomaly:   b.IsAnomaly,
			ChangeRate:  b.ChangeRate,
			SignalTitle: b.SignalTitle,
			SignalID:    b.SignalID,
			Keyword:     b.Keyword,
			ZScore:      b.ZScore,
		})
	}
	return out, nil
}

// GetStockCharts returns one trading day of intraday candles for many symbols
// in a single request.
//
// Endpoint: POST /api/v1/dashboard/common/stocks/mini-chart (cert host) with
// {"codes": [...]}. Range and step are chosen by the server (observed: 1d / 10m)
// and echoed back — they are NOT request parameters, so this cannot replace
// GetChart, which takes an explicit interval.
//
// KR and US both work (verified live 2026-08-24 with 005930 and AAPL). The
// server silently omits any code it has no data for instead of erroring, so the
// response can be shorter than the request; those symbols come back through the
// returned `missing` slice rather than as empty charts, letting a caller tell
// "no data for this symbol" from "the whole call failed".
func (c *Client) GetStockCharts(ctx context.Context, symbols []string) (domain.ChartBatch, error) {
	if err := c.requireSession(); err != nil {
		return domain.ChartBatch{}, err
	}
	requests, err := c.resolveProductCodes(ctx, symbols)
	if err != nil {
		return domain.ChartBatch{}, err
	}
	codes := batchCodes(requests)

	var envelope quoteEnvelope[struct {
		BaseStep   string `json:"baseStep"`
		MiniCharts []struct {
			Code    string `json:"code"`
			Candles []struct {
				StartDate time.Time `json:"startDate"`
				Open      float64   `json:"open"`
				High      float64   `json:"high"`
				Low       float64   `json:"low"`
				Close     float64   `json:"close"`
				Base      float64   `json:"base"`
			} `json:"candles"`
		} `json:"miniCharts"`
	}]
	body, err := json.Marshal(map[string]any{"codes": codes})
	if err != nil {
		return domain.ChartBatch{}, err
	}
	endpoint := c.certBaseURL + "/api/v1/dashboard/common/stocks/mini-chart"
	if err := c.postJSON(ctx, endpoint, body, &envelope); err != nil {
		return domain.ChartBatch{}, err
	}

	now := time.Now().UTC()
	chartsByCode := make(map[string]domain.Chart, len(envelope.Result.MiniCharts))
	for _, mc := range envelope.Result.MiniCharts {
		chart := domain.Chart{
			ProductCode: mc.Code,
			Interval:    envelope.Result.BaseStep,
			FetchedAt:   now,
		}
		for _, cd := range mc.Candles {
			// base 는 캔들마다 같은 전일 종가다. 차트 단위 값이라 위로 올린다.
			chart.Base = cd.Base
			chart.Candles = append(chart.Candles, domain.Candle{
				Time:  cd.StartDate,
				Open:  cd.Open,
				High:  cd.High,
				Low:   cd.Low,
				Close: cd.Close,
			})
		}
		chartsByCode[mc.Code] = chart
	}
	charts, missing, sequence := reconcileBatch(requests, chartsByCode, func(chart domain.Chart, symbol string) domain.Chart {
		chart.Symbol = symbol
		return chart
	})
	out := domain.ChartBatch{Charts: charts, Missing: missing, Sequence: sequence, FetchedAt: now}
	return out, nil
}

// resolveProductCodes turns a caller's symbols into Toss product codes for a
// batch request while preserving the original spelling at every position.
//
// Batch WTS endpoints answer with product codes only and omit codes they have
// no data for, so every caller needs ordered code/symbol pairs. A reverse map
// is insufficient because two aliases can resolve to the same product code.
func (c *Client) resolveProductCodes(ctx context.Context, symbols []string) ([]batchRequest, error) {
	requests := make([]batchRequest, 0, len(symbols))
	for _, s := range symbols {
		code, err := c.resolveProductCode(ctx, s)
		if err != nil {
			return nil, err
		}
		requests = append(requests, batchRequest{code: code, symbol: s})
	}
	return requests, nil
}
