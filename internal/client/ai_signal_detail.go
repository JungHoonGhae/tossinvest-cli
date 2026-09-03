package client

import (
	"context"
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
	Terms struct {
		ServiceAgreed             bool `json:"serviceAgreed"`
		PersonalizedServiceAgreed bool `json:"personalizedServiceAgreed"`
	} `json:"terms"`
}

// AISignalProductType converts the two product types observed in the live WTS
// briefing contract to the exact values accepted by the detail endpoint.
func AISignalProductType(value string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", "_"))
	switch normalized {
	case "STOCK", "STOCKS":
		return "STOCKS", nil
	case "ETF", "EQUITY_ETF":
		return "EQUITY_ETF", nil
	default:
		return "", fmt.Errorf("unsupported AI signal product type %q: use stocks or equity_etf", value)
	}
}

// GetAISignalDetail returns the full current AI reasoning for one stock or
// equity ETF. The product-type vocabulary is taken from the live WTS briefing
// contract; an absent current signal is represented by Found=false.
func (c *Client) GetAISignalDetail(ctx context.Context, symbol, productType string) (domain.AISignalDetail, error) {
	typ, err := AISignalProductType(productType)
	if err != nil {
		return domain.AISignalDetail{}, err
	}
	if err := c.requireSession(); err != nil {
		return domain.AISignalDetail{}, err
	}
	code, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.AISignalDetail{}, err
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/dashboard/wts/overview/ai-signals/detail")
	if err != nil {
		return domain.AISignalDetail{}, err
	}
	query := endpoint.Query()
	query.Set("productCode", code)
	query.Set("productType", typ)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[*aiSignalDetailRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.AISignalDetail{}, err
	}
	out := domain.AISignalDetail{
		ProductCode: code,
		ProductType: typ,
		Found:       envelope.Result != nil,
		FetchedAt:   time.Now().UTC(),
		Keywords:    []string{},
		News:        []domain.BriefingNews{},
		Related:     []domain.AISignalRelatedReasoning{},
	}
	if envelope.Result == nil {
		return out, nil
	}
	raw := envelope.Result
	out.SignalID = raw.SignalID
	out.TraceID = raw.TraceID
	out.CreatedAt = raw.CreatedAt
	out.SignalDirection = raw.SignalDirection
	out.HasRelatedReasoning = raw.HasRelatedReasoning
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
	out.Terms = domain.AISignalTerms{
		ServiceAgreed:             raw.Terms.ServiceAgreed,
		PersonalizedServiceAgreed: raw.Terms.PersonalizedServiceAgreed,
	}
	return out, nil
}
