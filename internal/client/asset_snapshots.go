package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	assetSnapshotRange    = "ONE_MONTH"
	assetSnapshotStepUnit = "DAY"
)

// GetAssetPerformance returns the verified one-month daily portfolio history.
// An empty account key selects the all-account aggregate.
func (c *Client) GetAssetPerformance(ctx context.Context, accountKey string) (domain.AssetPerformance, error) {
	if err := c.requireSession(); err != nil {
		return domain.AssetPerformance{}, err
	}
	var envelope quoteEnvelope[struct {
		HasKRStock          bool                         `json:"hasKrStock"`
		HasKRStockInRange   bool                         `json:"hasKrStockInRange"`
		HasProduct          bool                         `json:"hasProduct"`
		HasProductInRange   bool                         `json:"hasProductInRange"`
		EvaluatedAmountDiff domain.AssetAmount           `json:"evaluatedAmountDiff"`
		MaxEvaluated        assetSnapshotExtremeResponse `json:"maxEvaluated"`
		MinEvaluated        assetSnapshotExtremeResponse `json:"minEvaluated"`
		Points              []assetSnapshotPointResponse `json:"points"`
	}]
	allAccountsPath := "/api/v1/asset-snapshot/all-accounts/chart/" + assetSnapshotRange + "/" + assetSnapshotStepUnit
	accountPath := "/api/v1/asset-snapshot/chart/" + assetSnapshotRange + "/" + assetSnapshotStepUnit
	scope, accountScope, err := c.getAssetSnapshotJSON(ctx, accountKey, allAccountsPath, accountPath, &envelope)
	if err != nil {
		return domain.AssetPerformance{}, err
	}
	result := domain.AssetPerformance{
		Scope:               scope,
		Range:               assetSnapshotRange,
		StepUnit:            assetSnapshotStepUnit,
		HasKRStock:          envelope.Result.HasKRStock,
		HasKRStockInRange:   envelope.Result.HasKRStockInRange,
		HasProduct:          envelope.Result.HasProduct,
		HasProductInRange:   envelope.Result.HasProductInRange,
		EvaluatedAmountDiff: envelope.Result.EvaluatedAmountDiff,
		MaxEvaluated:        envelope.Result.MaxEvaluated.domainValue(),
		MinEvaluated:        envelope.Result.MinEvaluated.domainValue(),
		Points:              make([]domain.AssetSnapshotPoint, 0, len(envelope.Result.Points)),
		FetchedAt:           time.Now().UTC(),
	}
	result.AccountScope = accountScope
	for _, point := range envelope.Result.Points {
		result.Points = append(result.Points, point.domainValue())
	}
	return result, nil
}

// ListAssetSnapshots returns one cursor page of historical valuations. An
// empty account key selects the all-account aggregate.
func (c *Client) ListAssetSnapshots(ctx context.Context, accountKey, cursor string, pageSize int) (domain.AssetSnapshotPage, error) {
	if err := c.requireSession(); err != nil {
		return domain.AssetSnapshotPage{}, err
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize < 0 {
		return domain.AssetSnapshotPage{}, fmt.Errorf("page size must be positive")
	}
	query := url.Values{"pageSize": {strconv.Itoa(pageSize)}}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		query.Set("cursorKey", cursor)
	}
	var envelope quoteEnvelope[struct {
		Body          []assetSnapshotPointResponse `json:"body"`
		NextCursorKey string                       `json:"nextCursorKey"`
	}]
	allAccountsPath := "/api/v1/asset-snapshot/all-accounts/page?" + query.Encode()
	accountPath := "/api/v1/asset-snapshot/page?" + query.Encode()
	scope, accountScope, err := c.getAssetSnapshotJSON(ctx, accountKey, allAccountsPath, accountPath, &envelope)
	if err != nil {
		return domain.AssetSnapshotPage{}, err
	}
	result := domain.AssetSnapshotPage{
		Scope:      scope,
		PageSize:   pageSize,
		Snapshots:  make([]domain.AssetSnapshotPoint, 0, len(envelope.Result.Body)),
		NextCursor: envelope.Result.NextCursorKey,
		HasNext:    strings.TrimSpace(envelope.Result.NextCursorKey) != "",
		FetchedAt:  time.Now().UTC(),
	}
	result.AccountScope = accountScope
	for _, point := range envelope.Result.Body {
		result.Snapshots = append(result.Snapshots, point.domainValue())
	}
	return result, nil
}

// GetAssetSnapshot returns the full all-market valuation for one date. An
// empty account key selects the all-account aggregate.
func (c *Client) GetAssetSnapshot(ctx context.Context, accountKey, baseDate string) (domain.AssetSnapshotDetail, error) {
	if err := c.requireSession(); err != nil {
		return domain.AssetSnapshotDetail{}, err
	}
	baseDate = strings.TrimSpace(baseDate)
	if _, err := time.Parse("2006-01-02", baseDate); err != nil {
		return domain.AssetSnapshotDetail{}, fmt.Errorf("base date must be YYYY-MM-DD: %w", err)
	}
	query := url.Values{"baseDate": {baseDate}}
	var envelope quoteEnvelope[assetSnapshotDetailResponse]
	allAccountsPath := "/api/v1/asset-snapshot/all-accounts/detail-by-date?" + query.Encode()
	accountPath := "/api/v1/asset-snapshot/detail-by-date?" + query.Encode()
	scope, accountScope, err := c.getAssetSnapshotJSON(ctx, accountKey, allAccountsPath, accountPath, &envelope)
	if err != nil {
		return domain.AssetSnapshotDetail{}, err
	}
	return envelope.Result.domainValue(scope, accountScope, time.Now().UTC()), nil
}

func (c *Client) getAssetSnapshotJSON(
	ctx context.Context,
	accountKey, allAccountsPath, accountPath string,
	target any,
) (domain.AssetSnapshotScope, string, error) {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		if err := c.getJSON(ctx, c.certBaseURL+allAccountsPath, target); err != nil {
			return "", "", err
		}
		return domain.AssetSnapshotScopeAllAccounts, "", nil
	}
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+accountPath, accountKey, target); err != nil {
		return "", "", err
	}
	return domain.AssetSnapshotScopeAccount, c.accountScope(accountKey), nil
}

type assetSnapshotExtremeResponse struct {
	BaseDate string             `json:"baseDate"`
	Amount   domain.AssetAmount `json:"amount"`
}

func (v assetSnapshotExtremeResponse) domainValue() domain.AssetSnapshotExtreme {
	return domain.AssetSnapshotExtreme{BaseDate: v.BaseDate, Amount: v.Amount}
}

type assetSnapshotPointResponse struct {
	BaseDate           string             `json:"baseDate"`
	PrincipalAmount    domain.AssetAmount `json:"principalAmount"`
	EvaluatedAmount    domain.AssetAmount `json:"evaluatedAmount"`
	ProfitLossAmount   domain.AssetAmount `json:"profitLossAmount"`
	ProfitLossRate     domain.AssetRate   `json:"profitLossRate"`
	Realtime           bool               `json:"realtime"`
	EvaluationComplete bool               `json:"evaluationComplete"`
}

func (v assetSnapshotPointResponse) domainValue() domain.AssetSnapshotPoint {
	return domain.AssetSnapshotPoint{
		BaseDate:           v.BaseDate,
		PrincipalAmount:    v.PrincipalAmount,
		EvaluatedAmount:    v.EvaluatedAmount,
		ProfitLossAmount:   v.ProfitLossAmount,
		ProfitLossRate:     v.ProfitLossRate,
		Realtime:           v.Realtime,
		EvaluationComplete: v.EvaluationComplete,
	}
}

type assetSnapshotHoldingResponse struct {
	ProductCode      string             `json:"productCode"`
	ISIN             string             `json:"isin"`
	Symbol           string             `json:"symbol"`
	ProductName      string             `json:"productName"`
	Quantity         float64            `json:"quantity"`
	PurchasePrice    domain.AssetAmount `json:"purchasePrice"`
	PurchaseAmount   domain.AssetAmount `json:"purchaseAmount"`
	EvaluatedAmount  domain.AssetAmount `json:"evaluatedAmount"`
	ProfitLossAmount domain.AssetAmount `json:"profitLossAmount"`
	ProfitLossRate   domain.AssetRate   `json:"profitLossRate"`
	MarketDivision   string             `json:"marketDivision"`
	LogoImageURL     string             `json:"logoImageUrl"`
	Type             string             `json:"type"`
}

func (v assetSnapshotHoldingResponse) domainValue() domain.AssetSnapshotHolding {
	return domain.AssetSnapshotHolding{
		ProductCode:      v.ProductCode,
		ISIN:             v.ISIN,
		Symbol:           v.Symbol,
		Name:             v.ProductName,
		Quantity:         v.Quantity,
		PurchasePrice:    v.PurchasePrice,
		PurchaseAmount:   v.PurchaseAmount,
		EvaluatedAmount:  v.EvaluatedAmount,
		ProfitLossAmount: v.ProfitLossAmount,
		ProfitLossRate:   v.ProfitLossRate,
		MarketDivision:   v.MarketDivision,
		LogoImageURL:     v.LogoImageURL,
		Type:             v.Type,
	}
}

type assetSnapshotMarketResponse struct {
	PrincipalAmount  domain.AssetAmount             `json:"principalAmount"`
	EvaluatedAmount  domain.AssetAmount             `json:"evaluatedAmount"`
	ProfitLossAmount domain.AssetAmount             `json:"profitLossAmount"`
	ProfitLossRate   domain.AssetRate               `json:"profitLossRate"`
	Items            []assetSnapshotHoldingResponse `json:"items"`
}

func (v assetSnapshotMarketResponse) domainValue(market string) domain.AssetSnapshotMarket {
	result := domain.AssetSnapshotMarket{
		Market:           market,
		PrincipalAmount:  v.PrincipalAmount,
		EvaluatedAmount:  v.EvaluatedAmount,
		ProfitLossAmount: v.ProfitLossAmount,
		ProfitLossRate:   v.ProfitLossRate,
		Holdings:         make([]domain.AssetSnapshotHolding, 0, len(v.Items)),
	}
	for _, item := range v.Items {
		result.Holdings = append(result.Holdings, item.domainValue())
	}
	return result
}

type assetSnapshotDetailResponse struct {
	BaseDate           string                      `json:"baseDate"`
	EvaluationComplete bool                        `json:"evaluationComplete"`
	PrincipalAmount    domain.AssetAmount          `json:"principalAmount"`
	EvaluatedAmount    domain.AssetAmount          `json:"evaluatedAmount"`
	ProfitLossAmount   domain.AssetAmount          `json:"profitLossAmount"`
	ProfitLossRate     domain.AssetRate            `json:"profitLossRate"`
	KR                 assetSnapshotMarketResponse `json:"kr"`
	Option             assetSnapshotMarketResponse `json:"option"`
	US                 assetSnapshotMarketResponse `json:"us"`
	Bond               assetSnapshotMarketResponse `json:"bond"`
}

func (v assetSnapshotDetailResponse) domainValue(scope domain.AssetSnapshotScope, accountScope string, fetchedAt time.Time) domain.AssetSnapshotDetail {
	return domain.AssetSnapshotDetail{
		Scope:              scope,
		AccountScope:       accountScope,
		BaseDate:           v.BaseDate,
		EvaluationComplete: v.EvaluationComplete,
		PrincipalAmount:    v.PrincipalAmount,
		EvaluatedAmount:    v.EvaluatedAmount,
		ProfitLossAmount:   v.ProfitLossAmount,
		ProfitLossRate:     v.ProfitLossRate,
		Markets: []domain.AssetSnapshotMarket{
			v.KR.domainValue("kr"),
			v.Option.domainValue("option"),
			v.US.domainValue("us"),
			v.Bond.domainValue("bond"),
		},
		FetchedAt: fetchedAt,
	}
}
