package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type sectorPriceRaw struct {
	Base      *float64 `json:"base"`
	BaseKRW   *float64 `json:"baseKrw"`
	Close     *float64 `json:"close"`
	CloseKRW  *float64 `json:"closeKrw"`
	PriceType string   `json:"priceType"`
}

type sectorStockRaw struct {
	Rank            int            `json:"rank"`
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	LogoImageURL    string         `json:"logoImageUrl"`
	AnalystOpinion  string         `json:"analystOpinion"`
	ChangeRate      float64        `json:"changeRate"`
	MarketCapKRW    float64        `json:"marketCapKrw"`
	MarketCapUSD    float64        `json:"marketCapUsd"`
	TradingValueKRW float64        `json:"tradingValueKrw"`
	TradingValueUSD float64        `json:"tradingValueUsd"`
	Volume          float64        `json:"volume"`
	Price           sectorPriceRaw `json:"price"`
}

type sectorETFRaw struct {
	Rank           int     `json:"rank"`
	Code           string  `json:"code"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	DetailName     string  `json:"detailName"`
	LogoImageURL   string  `json:"logoImageUrl"`
	ChangeRate     float64 `json:"changeRate"`
	ExpenseRatio   float64 `json:"expenseRatio"`
	LeverageFactor float64 `json:"leverageFactor"`
	TopHolding     *struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"topHolding"`
	TradingValueKRW float64        `json:"tradingValueKrw"`
	TradingValueUSD float64        `json:"tradingValueUsd"`
	Price           sectorPriceRaw `json:"price"`
}

type sectorNewsRaw struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	ImageURLs []string `json:"imageUrls"`
}

func mapSectorPrice(raw sectorPriceRaw) domain.SectorPrice {
	return domain.SectorPrice{Base: raw.Base, BaseKRW: raw.BaseKRW, Close: raw.Close, CloseKRW: raw.CloseKRW, PriceType: raw.PriceType}
}

// GetSectorDetail combines the four WTS TICS detail resources. The aggregate
// keeps callers independent of the dashboard's multiple-request UI layout.
func (c *Client) GetSectorDetail(ctx context.Context, id int) (domain.SectorDetail, error) {
	if id <= 0 {
		return domain.SectorDetail{}, fmt.Errorf("sector id must be greater than zero")
	}
	if err := c.requireSession(); err != nil {
		return domain.SectorDetail{}, err
	}
	var overview quoteEnvelope[struct {
		TicsID       int    `json:"ticsId"`
		Name         string `json:"name"`
		Summary      string `json:"summary"`
		Description  string `json:"description"`
		Depth        int    `json:"depth"`
		CompanyCount int    `json:"companyCount"`
		ETFCount     int    `json:"etfCount"`
	}]
	var stocks quoteEnvelope[struct {
		Stocks     []sectorStockRaw `json:"stocks"`
		TotalCount int              `json:"totalCount"`
	}]
	var etfs quoteEnvelope[struct {
		ETFs       []sectorETFRaw `json:"etfs"`
		TotalCount int            `json:"totalCount"`
	}]
	var news quoteEnvelope[struct {
		Body       []sectorNewsRaw `json:"body"`
		TotalCount int             `json:"totalCount"`
	}]
	endpoints := []string{
		fmt.Sprintf("%s/api/v2/dashboard/wts/overview/tics/%d/overview", c.infoBaseURL, id),
		fmt.Sprintf("%s/api/v2/dashboard/wts/overview/tics/%d/stocks", c.infoBaseURL, id),
		fmt.Sprintf("%s/api/v2/dashboard/wts/overview/tics/%d/etfs", c.infoBaseURL, id),
		fmt.Sprintf("%s/api/v2/dashboard/wts/overview/tics/%d/news", c.infoBaseURL, id),
	}
	labels := []string{"overview", "stocks", "ETFs", "news"}
	errs := make([]error, len(endpoints))
	var wg sync.WaitGroup
	wg.Add(len(endpoints))
	go func() { defer wg.Done(); errs[0] = c.getJSON(ctx, endpoints[0], &overview) }()
	go func() { defer wg.Done(); errs[1] = c.postJSON(ctx, endpoints[1], json.RawMessage(`{}`), &stocks) }()
	go func() { defer wg.Done(); errs[2] = c.postJSON(ctx, endpoints[2], json.RawMessage(`{}`), &etfs) }()
	go func() { defer wg.Done(); errs[3] = c.getJSON(ctx, endpoints[3], &news) }()
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return domain.SectorDetail{}, fmt.Errorf("get sector %s: %w", labels[i], err)
		}
	}

	out := domain.SectorDetail{
		ID: overview.Result.TicsID, Name: overview.Result.Name,
		Summary: overview.Result.Summary, Description: overview.Result.Description,
		Depth: overview.Result.Depth, CompanyCount: overview.Result.CompanyCount, ETFCount: overview.Result.ETFCount,
		StockTotalCount: stocks.Result.TotalCount, ETFTotalCount: etfs.Result.TotalCount, NewsTotalCount: news.Result.TotalCount,
		Stocks: make([]domain.SectorStock, 0, len(stocks.Result.Stocks)),
		ETFs:   make([]domain.SectorETF, 0, len(etfs.Result.ETFs)),
		News:   make([]domain.SectorNews, 0, len(news.Result.Body)), FetchedAt: time.Now().UTC(),
	}
	for _, item := range stocks.Result.Stocks {
		out.Stocks = append(out.Stocks, domain.SectorStock{
			Rank: item.Rank, ProductCode: item.Code, Name: item.Name, LogoImageURL: item.LogoImageURL,
			AnalystOpinion: item.AnalystOpinion, ChangeRate: item.ChangeRate,
			MarketCapKRW: item.MarketCapKRW, MarketCapUSD: item.MarketCapUSD,
			TradingValueKRW: item.TradingValueKRW, TradingValueUSD: item.TradingValueUSD,
			Volume: item.Volume, Price: mapSectorPrice(item.Price),
		})
	}
	for _, item := range etfs.Result.ETFs {
		mapped := domain.SectorETF{
			Rank: item.Rank, ProductCode: item.Code, Symbol: item.Symbol, Name: item.Name,
			DetailName: item.DetailName, LogoImageURL: item.LogoImageURL, ChangeRate: item.ChangeRate,
			ExpenseRatio: item.ExpenseRatio, LeverageFactor: item.LeverageFactor,
			TradingValueKRW: item.TradingValueKRW, TradingValueUSD: item.TradingValueUSD,
			Price: mapSectorPrice(item.Price),
		}
		if item.TopHolding != nil {
			mapped.TopHolding = &domain.SectorTopHolding{Name: item.TopHolding.Name, Weight: item.TopHolding.Weight}
		}
		out.ETFs = append(out.ETFs, mapped)
	}
	for _, item := range news.Result.Body {
		out.News = append(out.News, domain.SectorNews{
			ID: item.ID, Title: item.Title, Summary: item.Summary, Source: item.Source,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, ImageURLs: item.ImageURLs,
		})
	}
	return out, nil
}
