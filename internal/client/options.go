package client

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type optionExpiryRaw struct {
	MaturityDate        string  `json:"maturityDate"`
	MaturityDateTime    string  `json:"maturityDateTime"`
	LiquidationDateTime string  `json:"liquidationDateTime"`
	DisplayLiquidation  string  `json:"displayLiquidationDateTime"`
	CorporateActionName *string `json:"displayCorporateActionName"`
}

type optionExpiriesRaw struct {
	Items []optionExpiryRaw `json:"items"`
}

// GetOptionExpiries lists the expirations quoted for a US underlying.
// 공식 Open API 에 없는 web 전용 표면.
//
// The API keys options off `underlyingGuid`, which is the same string as the
// product code (verified live: AAPL resolves to US19801212001 for both). It is
// still passed under its own name so a future divergence shows up here rather
// than as a silent wrong-symbol lookup.
func (c *Client) GetOptionExpiries(ctx context.Context, symbol string) (domain.OptionExpiries, error) {
	if err := c.requireSession(); err != nil {
		return domain.OptionExpiries{}, err
	}
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.OptionExpiries{}, err
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/option-maturity-date/get-all")
	if err != nil {
		return domain.OptionExpiries{}, err
	}
	query := endpoint.Query()
	query.Set("underlyingGuid", productCode)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[optionExpiriesRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.OptionExpiries{}, err
	}

	out := domain.OptionExpiries{Symbol: symbol, ProductCode: productCode, FetchedAt: time.Now().UTC()}
	for _, e := range envelope.Result.Items {
		exp := domain.OptionExpiry{
			MaturityDate:        e.MaturityDate,
			MaturityDateTime:    e.MaturityDateTime,
			LiquidationDateTime: e.LiquidationDateTime,
			DisplayLiquidation:  e.DisplayLiquidation,
		}
		if e.CorporateActionName != nil {
			exp.CorporateActionName = *e.CorporateActionName
		}
		out.Expiries = append(out.Expiries, exp)
	}
	return out, nil
}

type optionChainRowRaw struct {
	StrikePrice      float64 `json:"strikePrice"`
	CallGUID         string  `json:"callGuid"`
	PutGUID          string  `json:"putGuid"`
	CallOpenInterest int     `json:"callOpenInterest"`
	PutOpenInterest  int     `json:"putOpenInterest"`
}

// GetOptionChain returns the call/put chain for one expiration.
// 공식 Open API 에 없는 web 전용 표면.
//
// Server order is preserved — it comes back ascending by strike, and that is
// how the app renders it.
func (c *Client) GetOptionChain(ctx context.Context, symbol, maturityDate string) (domain.OptionChain, error) {
	if err := c.requireSession(); err != nil {
		return domain.OptionChain{}, err
	}
	if maturityDate == "" {
		return domain.OptionChain{}, fmt.Errorf("maturity date is required")
	}
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.OptionChain{}, err
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/option-both-chain/get-all")
	if err != nil {
		return domain.OptionChain{}, err
	}
	query := endpoint.Query()
	query.Set("underlyingGuid", productCode)
	query.Set("maturityDate", maturityDate)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[[]optionChainRowRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.OptionChain{}, err
	}

	out := domain.OptionChain{
		Symbol: symbol, ProductCode: productCode,
		MaturityDate: maturityDate, FetchedAt: time.Now().UTC(),
	}
	for _, r := range envelope.Result {
		out.Rows = append(out.Rows, domain.OptionChainRow{
			StrikePrice:      r.StrikePrice,
			CallGUID:         r.CallGUID,
			PutGUID:          r.PutGUID,
			CallOpenInterest: r.CallOpenInterest,
			PutOpenInterest:  r.PutOpenInterest,
		})
	}
	return out, nil
}
