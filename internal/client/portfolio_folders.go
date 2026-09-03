package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const portfolioFolderOverviewType = "FOLDER_OVERVIEW_V2"

type portfolioFolderSummaryResponse struct {
	PrincipalAmount          domain.AssetAmount `json:"principalAmount"`
	EvaluatedAmount          domain.AssetAmount `json:"evaluatedAmount"`
	EvaluatedAmountAfterFees domain.AssetAmount `json:"evaluatedAmountAfterFees"`
	ProfitLossAmount         domain.AssetAmount `json:"profitLossAmount"`
	ProfitLossAfterFees      domain.AssetAmount `json:"profitLossAmountAfterFees"`
	DailyProfitLossAmount    domain.AssetAmount `json:"dailyProfitLossAmount"`
	ProfitLossRate           domain.AssetRate   `json:"profitLossRate"`
	ProfitLossRateAfterFees  domain.AssetRate   `json:"profitLossRateAfterFees"`
	DailyProfitLossRate      domain.AssetRate   `json:"dailyProfitLossRate"`
}

func (r portfolioFolderSummaryResponse) domainValue() domain.PortfolioFolderSummary {
	return domain.PortfolioFolderSummary{
		PrincipalAmount:          r.PrincipalAmount,
		EvaluatedAmount:          r.EvaluatedAmount,
		EvaluatedAmountAfterFees: r.EvaluatedAmountAfterFees,
		ProfitLossAmount:         r.ProfitLossAmount,
		ProfitLossAfterFees:      r.ProfitLossAfterFees,
		DailyProfitLossAmount:    r.DailyProfitLossAmount,
		ProfitLossRate:           r.ProfitLossRate,
		ProfitLossRateAfterFees:  r.ProfitLossRateAfterFees,
		DailyProfitLossRate:      r.DailyProfitLossRate,
	}
}

type portfolioFolderItemResponse struct {
	Key               string  `json:"key"`
	ProductCode       string  `json:"stockCode"`
	ISIN              string  `json:"stockIsin"`
	Symbol            string  `json:"stockSymbol"`
	Name              string  `json:"stockName"`
	Quantity          float64 `json:"quantity"`
	TradableQuantity  float64 `json:"tradableQuantity"`
	UnsettledQuantity float64 `json:"unsettledQuantity"`

	CurrentPrice             domain.AssetAmount `json:"currentPrice"`
	BasePrice                domain.AssetAmount `json:"basePrice"`
	PurchasePrice            domain.AssetAmount `json:"purchasePrice"`
	PurchaseAmount           domain.AssetAmount `json:"purchaseAmount"`
	EvaluatedAmount          domain.AssetAmount `json:"evaluatedAmount"`
	EvaluatedAmountAfterFees domain.AssetAmount `json:"evaluatedAmountAfterFees"`
	ProfitLossAmount         domain.AssetAmount `json:"profitLossAmount"`
	ProfitLossAfterFees      domain.AssetAmount `json:"profitLossAmountAfterFees"`
	DailyProfitLossAmount    domain.AssetAmount `json:"dailyProfitLossAmount"`
	ProfitLossRate           domain.AssetRate   `json:"profitLossRate"`
	ProfitLossRateAfterFees  domain.AssetRate   `json:"profitLossRateAfterFees"`
	DailyProfitLossRate      domain.AssetRate   `json:"dailyProfitLossRate"`
	Commission               domain.AssetAmount `json:"commission"`
	CommissionRate           float64            `json:"commissionRate"`
	BuyCommission            domain.AssetAmount `json:"buyCommission"`
	SellCommission           domain.AssetAmount `json:"sellCommission"`

	MarketCode        string `json:"marketCode"`
	MarketDivision    string `json:"marketDivision"`
	Type              string `json:"type"`
	ShareHoldingsType string `json:"shareHoldingsType"`
	Delisting         bool   `json:"delisting"`
	Unlisting         bool   `json:"unlisting"`
	Archiving         bool   `json:"archiving"`
	ErrorPricing      bool   `json:"errorPricing"`
	NXTSupported      bool   `json:"nxtSupported"`
}

func (r portfolioFolderItemResponse) domainValue() domain.PortfolioFolderItem {
	return domain.PortfolioFolderItem{
		Key: r.Key, ProductCode: r.ProductCode, ISIN: r.ISIN, Symbol: r.Symbol, Name: r.Name,
		Quantity: r.Quantity, TradableQuantity: r.TradableQuantity, UnsettledQuantity: r.UnsettledQuantity,
		CurrentPrice: r.CurrentPrice, BasePrice: r.BasePrice, PurchasePrice: r.PurchasePrice, PurchaseAmount: r.PurchaseAmount,
		EvaluatedAmount: r.EvaluatedAmount, EvaluatedAmountAfterFees: r.EvaluatedAmountAfterFees,
		ProfitLossAmount: r.ProfitLossAmount, ProfitLossAfterFees: r.ProfitLossAfterFees, DailyProfitLossAmount: r.DailyProfitLossAmount,
		ProfitLossRate: r.ProfitLossRate, ProfitLossRateAfterFees: r.ProfitLossRateAfterFees, DailyProfitLossRate: r.DailyProfitLossRate,
		Commission: r.Commission, CommissionRate: r.CommissionRate, BuyCommission: r.BuyCommission, SellCommission: r.SellCommission,
		MarketCode: r.MarketCode, MarketDivision: r.MarketDivision, Type: r.Type, ShareHoldingsType: r.ShareHoldingsType,
		Delisting: r.Delisting, Unlisting: r.Unlisting, Archiving: r.Archiving, ErrorPricing: r.ErrorPricing, NXTSupported: r.NXTSupported,
	}
}

type portfolioFolderResponse struct {
	portfolioFolderSummaryResponse
	Key        string                         `json:"folderKey"`
	Name       string                         `json:"folderName"`
	Type       string                         `json:"folderType"`
	DetailType *string                        `json:"detailType"`
	Default    bool                           `json:"isDefault"`
	Items      *[]portfolioFolderItemResponse `json:"items"`
}

type portfolioFoldersDataResponse struct {
	portfolioFolderSummaryResponse
	Folders []portfolioFolderResponse `json:"folders"`
	Hidden  struct {
		Count  int     `json:"count"`
		All    bool    `json:"all"`
		Amount float64 `json:"amount"`
	} `json:"hiddenStock"`
}

// ListPortfolioFolders returns the current account's user-visible holding
// groups and their fee-aware valuation summaries. Folder and item keys are not
// exposed; this is a read surface, not a folder-mutation transport.
func (c *Client) ListPortfolioFolders(ctx context.Context, accountKey string) (domain.PortfolioFolders, error) {
	if err := c.requireSession(); err != nil {
		return domain.PortfolioFolders{}, err
	}
	resolved, err := c.resolveAccountKey(ctx, accountKey)
	if err != nil {
		return domain.PortfolioFolders{}, err
	}
	var envelope quoteEnvelope[struct {
		Sections []struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"sections"`
	}]
	body := []byte(`{"types":["FOLDER_OVERVIEW_V2"]}`)
	if err := c.postJSONWithAccountKey(ctx, c.certBaseURL+"/api/v2/dashboard/asset/sections/all", body, resolved, &envelope); err != nil {
		return domain.PortfolioFolders{}, err
	}
	for _, section := range envelope.Result.Sections {
		if section.Type != portfolioFolderOverviewType {
			continue
		}
		data, err := decodePortfolioFoldersData(section.Data)
		if err != nil {
			return domain.PortfolioFolders{}, fmt.Errorf("decode %s data: %w", portfolioFolderOverviewType, err)
		}
		out := domain.PortfolioFolders{
			PortfolioFolderSummary: data.portfolioFolderSummaryResponse.domainValue(),
			SectionType:            section.Type,
			AccountScope:           c.accountScope(resolved),
			Folders:                make([]domain.PortfolioFolder, 0, len(data.Folders)),
			Hidden: domain.PortfolioHiddenSummary{
				Count: data.Hidden.Count, All: data.Hidden.All, Amount: data.Hidden.Amount,
			},
			FetchedAt: time.Now().UTC(),
		}
		for _, folder := range data.Folders {
			mapped := domain.PortfolioFolder{
				PortfolioFolderSummary: folder.portfolioFolderSummaryResponse.domainValue(),
				Key:                    folder.Key,
				Name:                   folder.Name,
				Type:                   folder.Type,
				DetailType:             optionalStringValue(folder.DetailType),
				Default:                folder.Default,
				Items:                  make([]domain.PortfolioFolderItem, 0, len(*folder.Items)),
			}
			for _, item := range *folder.Items {
				mapped.Items = append(mapped.Items, item.domainValue())
			}
			out.Folders = append(out.Folders, mapped)
		}
		return out, nil
	}
	return domain.PortfolioFolders{}, fmt.Errorf("%s section not found", portfolioFolderOverviewType)
}

func decodePortfolioFoldersData(raw json.RawMessage) (portfolioFoldersDataResponse, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] != '{' {
		return portfolioFoldersDataResponse{}, fmt.Errorf("data must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return portfolioFoldersDataResponse{}, err
	}
	for _, required := range []struct {
		name string
		kind byte
	}{
		{name: "folders", kind: '['},
		{name: "hiddenStock", kind: '{'},
		{name: "evaluatedAmountAfterFees", kind: '{'},
		{name: "profitLossAmountAfterFees", kind: '{'},
	} {
		value, ok := fields[required.name]
		value = bytes.TrimSpace(value)
		if !ok || len(value) == 0 {
			return portfolioFoldersDataResponse{}, fmt.Errorf("missing %s", required.name)
		}
		if value[0] != required.kind {
			return portfolioFoldersDataResponse{}, fmt.Errorf("%s has invalid type", required.name)
		}
	}
	var hidden map[string]json.RawMessage
	if err := json.Unmarshal(fields["hiddenStock"], &hidden); err != nil {
		return portfolioFoldersDataResponse{}, fmt.Errorf("decode hiddenStock: %w", err)
	}
	for _, required := range []struct {
		name string
		kind string
	}{
		{name: "count", kind: "number"},
		{name: "all", kind: "boolean"},
		{name: "amount", kind: "number"},
	} {
		if err := requirePortfolioJSONType(hidden, required.name, required.kind, "hiddenStock."+required.name); err != nil {
			return portfolioFoldersDataResponse{}, err
		}
	}
	for _, name := range []string{"evaluatedAmountAfterFees", "profitLossAmountAfterFees"} {
		if err := validatePortfolioAssetPair(fields[name], name); err != nil {
			return portfolioFoldersDataResponse{}, err
		}
	}

	var folders []map[string]json.RawMessage
	if err := json.Unmarshal(fields["folders"], &folders); err != nil {
		return portfolioFoldersDataResponse{}, fmt.Errorf("decode folders: %w", err)
	}
	for folderIndex, folder := range folders {
		folderPath := fmt.Sprintf("folders[%d]", folderIndex)
		for _, name := range []string{"folderName", "folderType"} {
			if err := requirePortfolioNonEmptyString(folder, name, folderPath+"."+name); err != nil {
				return portfolioFoldersDataResponse{}, err
			}
		}
		for _, name := range []string{"evaluatedAmountAfterFees", "profitLossAmountAfterFees"} {
			if err := validatePortfolioAssetPair(folder[name], folderPath+"."+name); err != nil {
				return portfolioFoldersDataResponse{}, err
			}
		}
		itemsRaw, ok := folder["items"]
		if !ok {
			return portfolioFoldersDataResponse{}, fmt.Errorf("%s.items must be an array", folderPath)
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(itemsRaw, &items); err != nil || items == nil {
			return portfolioFoldersDataResponse{}, fmt.Errorf("%s.items must be an array", folderPath)
		}
		for itemIndex, item := range items {
			itemPath := fmt.Sprintf("%s.items[%d]", folderPath, itemIndex)
			if err := requirePortfolioNonEmptyString(item, "stockCode", itemPath+".stockCode"); err != nil {
				return portfolioFoldersDataResponse{}, fmt.Errorf("%s missing stockCode", itemPath)
			}
			for _, name := range []string{"evaluatedAmountAfterFees", "profitLossAmountAfterFees"} {
				if err := validatePortfolioAssetPair(item[name], itemPath+"."+name); err != nil {
					return portfolioFoldersDataResponse{}, err
				}
			}
		}
	}
	var data portfolioFoldersDataResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return portfolioFoldersDataResponse{}, err
	}
	return data, nil
}

func validatePortfolioAssetPair(raw json.RawMessage, path string) error {
	var fields map[string]json.RawMessage
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	for _, currency := range []string{"krw", "usd"} {
		if err := requirePortfolioJSONType(fields, currency, "number", path+"."+currency); err != nil {
			return err
		}
	}
	return nil
}

func requirePortfolioNonEmptyString(fields map[string]json.RawMessage, name, path string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing %s", path)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must be a non-empty string", path)
	}
	return nil
}

func requirePortfolioJSONType(fields map[string]json.RawMessage, name, kind, path string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing %s", path)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s must be a %s", path, kind)
	}
	valid := false
	switch kind {
	case "number":
		_, valid = value.(json.Number)
	case "boolean":
		_, valid = value.(bool)
	}
	if !valid {
		return fmt.Errorf("%s must be a %s", path, kind)
	}
	return nil
}
