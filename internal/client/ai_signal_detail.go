package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type aiSignalDescriptionRaw struct {
	Data []string `json:"data"`
}

type aiSignalNewsRaw struct {
	Data []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		AgencyName string `json:"agencyName"`
		Source     string `json:"source"`
		FaviconURL string `json:"faviconUrl"`
		CreatedAt  string `json:"createdAt"`
	} `json:"data"`
}

type aiSignalDetailRaw struct {
	SignalID            string `json:"signalId"`
	TraceID             string `json:"traceId"`
	CreatedAt           string `json:"createdAt"`
	SignalDirection     int    `json:"signalDirection"`
	HasRelatedReasoning bool   `json:"hasRelatedReasoning"`
	Reasoning           struct {
		Description string `json:"description"`
		Issue       struct {
			AssetCode      string                 `json:"assetCode"`
			AssetName      string                 `json:"assetName"`
			AssetType      string                 `json:"assetType"`
			Description    aiSignalDescriptionRaw `json:"description"`
			InvestmentType string                 `json:"investmentType"`
			LogoImageURL   string                 `json:"logoImageUrl"`
			OriginCodes    []string               `json:"originCodes"`
			ProfitLossRate float64                `json:"profitLossRate"`
		} `json:"issue"`
		Keywords []string        `json:"keywords"`
		News     aiSignalNewsRaw `json:"news"`
	} `json:"reasoning"`
	RelatedReasoning struct {
		Callout string `json:"callout"`
		Details []struct {
			SignalID     string                 `json:"signalId"`
			AssetCode    string                 `json:"assetCode"`
			AssetName    string                 `json:"assetName"`
			Description  aiSignalDescriptionRaw `json:"description"`
			Relationship struct {
				SubjectName string `json:"subjectName"`
				Relation    string `json:"relation"`
				ObjectName  string `json:"objectName"`
			} `json:"relationship"`
			RelatedStocks []relatedStockRaw `json:"relatedStocks"`
		} `json:"details"`
	} `json:"relatedReasoning"`
	Terms *struct {
		ServiceAgreed             bool `json:"serviceAgreed"`
		PersonalizedServiceAgreed bool `json:"personalizedServiceAgreed"`
	} `json:"terms"`
}

// AISignalProductType normalizes the supported stock/ETF and index routes.
func AISignalProductType(value string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", "_"))
	switch normalized {
	case "STOCK", "STOCKS":
		return "STOCKS", nil
	case "ETF", "EQUITY_ETF":
		return "EQUITY_ETF", nil
	case "INDEX":
		return "INDEX", nil
	default:
		return "", fmt.Errorf("unsupported AI signal product type %q: use stocks, equity_etf, or index", value)
	}
}

// GetAISignalDetail returns current AI reasoning for a stock, equity ETF, or
// index. An explicitly null current signal is represented by Found=false.
func (c *Client) GetAISignalDetail(ctx context.Context, symbol, productType string) (domain.AISignalDetail, error) {
	typ, err := AISignalProductType(productType)
	if err != nil {
		return domain.AISignalDetail{}, err
	}
	if err := c.requireSession(); err != nil {
		return domain.AISignalDetail{}, err
	}
	code := strings.TrimSpace(symbol)
	var endpoint string
	if typ == "INDEX" {
		if code == "" || strings.ContainsAny(code, "/?#%\\") {
			return domain.AISignalDetail{}, fmt.Errorf("provide an index code from market index")
		}
		endpoint = c.certBaseURL + "/api/v1/reasoning/indices/" + url.PathEscape(code) + "/detail"
	} else {
		code, err = c.resolveProductCode(ctx, symbol)
		if err != nil {
			return domain.AISignalDetail{}, err
		}
		query := url.Values{"productCode": {code}, "productType": {typ}}
		endpoint = c.infoBaseURL + "/api/v1/dashboard/wts/overview/ai-signals/detail?" + query.Encode()
	}
	data, err := c.resultJSON(ctx, endpoint)
	if err != nil {
		return domain.AISignalDetail{}, err
	}
	var raw *aiSignalDetailRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.AISignalDetail{}, err
	}
	out := domain.AISignalDetail{
		ProductCode: code,
		ProductType: typ,
		Found:       raw != nil,
		FetchedAt:   time.Now().UTC(),
		Keywords:    []string{},
		News:        []domain.BriefingNews{},
		Related:     []domain.AISignalRelatedReasoning{},
	}
	if raw == nil {
		return out, nil
	}
	if raw.SignalID == "" || raw.Reasoning.Issue.AssetCode == "" {
		return domain.AISignalDetail{}, fmt.Errorf("AI signal is missing identity fields")
	}
	out.SignalID = raw.SignalID
	out.TraceID = raw.TraceID
	out.CreatedAt = raw.CreatedAt
	out.SignalDirection = raw.SignalDirection
	out.HasRelatedReasoning = raw.HasRelatedReasoning || len(raw.RelatedReasoning.Details) > 0
	out.Description = raw.Reasoning.Description
	out.Issue = domain.AISignalIssue{
		AssetCode: raw.Reasoning.Issue.AssetCode, AssetName: raw.Reasoning.Issue.AssetName,
		AssetType: raw.Reasoning.Issue.AssetType, Description: raw.Reasoning.Issue.Description.Data,
		InvestmentType: raw.Reasoning.Issue.InvestmentType, LogoImageURL: raw.Reasoning.Issue.LogoImageURL,
		OriginCodes: raw.Reasoning.Issue.OriginCodes, ProfitLossRate: raw.Reasoning.Issue.ProfitLossRate,
	}
	out.Keywords = append(out.Keywords, raw.Reasoning.Keywords...)
	for _, item := range raw.Reasoning.News.Data {
		out.News = append(out.News, domain.BriefingNews{
			ID: item.ID, Title: item.Title, Agency: item.AgencyName, Source: item.Source,
			FaviconURL: item.FaviconURL, CreatedAt: item.CreatedAt,
		})
	}
	out.RelatedCallout = raw.RelatedReasoning.Callout
	for _, item := range raw.RelatedReasoning.Details {
		out.Related = append(out.Related, domain.AISignalRelatedReasoning{
			SignalID: item.SignalID, AssetCode: item.AssetCode, AssetName: item.AssetName,
			Description: item.Description.Data,
			Relationship: domain.AISignalRelationship{
				SubjectName: item.Relationship.SubjectName,
				Relation:    item.Relationship.Relation,
				ObjectName:  item.Relationship.ObjectName,
			},
			Stocks: mapRelatedStocks(item.RelatedStocks),
		})
	}
	if raw.Terms != nil {
		out.Terms = &domain.AISignalTerms{
			ServiceAgreed:             raw.Terms.ServiceAgreed,
			PersonalizedServiceAgreed: raw.Terms.PersonalizedServiceAgreed,
		}
	}
	return out, nil
}
