package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type riaQuarterRaw struct {
	Quarter                 string  `json:"quarter"`
	Weight                  float64 `json:"weight"`
	TotalProfitLoss         float64 `json:"totalProfitLoss"`
	WeightedTotalProfitLoss float64 `json:"weightedTotalProfitLoss"`
}

type riaDeductionRaw struct {
	DeductionRate                   float64         `json:"deductionRate"`
	NormalAccountOverseasBuyAmount  float64         `json:"normalAccountOverseasBuyAmount"`
	NormalAccountOverseasSellAmount float64         `json:"normalAccountOverseasSellAmount"`
	RIAAccountOverseasSellAmount    float64         `json:"riaAccountOverseasSellAmount"`
	PreAdjustmentDeduction          float64         `json:"preAdjustmentDeduction"`
	TotalAmount                     float64         `json:"totalAmount"`
	QuarterlyProfitLoss             []riaQuarterRaw `json:"quarterlyProfitLoss"`
}

type riaReportRaw struct {
	TransferIncomeTax struct {
		EstimatedTransferIncomeTax float64 `json:"estimatedTransferIncomeTax"`
		EstimatedTaxSaving         float64 `json:"estimatedTaxSaving"`
		FinalTransferIncomeTax     float64 `json:"finalTransferIncomeTax"`
	} `json:"transferIncomeTax"`
	Detail struct {
		TotalTransferIncomeAmount   float64         `json:"totalTransferIncomeAmount"`
		NormalAccountTransferIncome float64         `json:"normalAccountTransferIncome"`
		RIAAccountTransferIncome    float64         `json:"riaAccountTransferIncome"`
		BaseDeduction               float64         `json:"baseDeduction"`
		RIADeduction                riaDeductionRaw `json:"riaDeduction"`
		ProfitAfterDeduction        float64         `json:"profitAfterDeduction"`
		TransferIncomeTaxRate       float64         `json:"transferIncomeTaxRate"`
		TransferIncomeTax           float64         `json:"transferIncomeTax"`
		LocalTaxRate                float64         `json:"localTaxRate"`
		LocalTax                    float64         `json:"localTax"`
	} `json:"transferIncomeTaxDetail"`
}

type riaLimitRaw struct {
	TotalLimit              float64 `json:"totalLimit"`
	RemainingLimit          float64 `json:"remainingLimit"`
	OverseasStockSellAmount float64 `json:"overseasStockSellAmount"`
	SettlementDate          *string `json:"settlementDate"`
	Settled                 bool    `json:"settled"`
}

type riaOptimizedRaw struct {
	EstimatedMaxTaxSaving float64 `json:"estimatedMaxTaxSaving"`
	ZeroReasonCode        string  `json:"zeroReasonCode"`
}

// GetRIAReport returns the RIA (해외주식 양도세 절세 계좌) tax-saving
// projection. 공식 Open API 에 없고, 웹에도 화면이 없는 모바일 앱 전용 표면.
//
// The report is the required part. The sell limit and the optimized-saving
// figure are enrichments fetched separately — an account without an RIA
// agreement can fail those while the report itself still answers, so they are
// best-effort and simply stay nil on error rather than sinking the command.
func (c *Client) GetRIAReport(ctx context.Context) (domain.RIAReport, error) {
	if err := c.requireSession(); err != nil {
		return domain.RIAReport{}, err
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.RIAReport{}, err
	}

	var envelope quoteEnvelope[riaReportRaw]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+"/api/v1/ria-calculator/report", key, &envelope); err != nil {
		return domain.RIAReport{}, err
	}
	r := envelope.Result
	d := r.Detail.RIADeduction

	out := domain.RIAReport{
		EstimatedTransferIncomeTax:  r.TransferIncomeTax.EstimatedTransferIncomeTax,
		EstimatedTaxSaving:          r.TransferIncomeTax.EstimatedTaxSaving,
		FinalTransferIncomeTax:      r.TransferIncomeTax.FinalTransferIncomeTax,
		TotalTransferIncomeAmount:   r.Detail.TotalTransferIncomeAmount,
		NormalAccountTransferIncome: r.Detail.NormalAccountTransferIncome,
		RIAAccountTransferIncome:    r.Detail.RIAAccountTransferIncome,
		BaseDeduction:               r.Detail.BaseDeduction,
		ProfitAfterDeduction:        r.Detail.ProfitAfterDeduction,
		TransferIncomeTaxRate:       r.Detail.TransferIncomeTaxRate,
		TransferIncomeTax:           r.Detail.TransferIncomeTax,
		LocalTaxRate:                r.Detail.LocalTaxRate,
		LocalTax:                    r.Detail.LocalTax,
		Deduction: domain.RIADeduction{
			DeductionRate:                   d.DeductionRate,
			NormalAccountOverseasBuyAmount:  d.NormalAccountOverseasBuyAmount,
			NormalAccountOverseasSellAmount: d.NormalAccountOverseasSellAmount,
			RIAAccountOverseasSellAmount:    d.RIAAccountOverseasSellAmount,
			PreAdjustmentDeduction:          d.PreAdjustmentDeduction,
			TotalAmount:                     d.TotalAmount,
		},
		FetchedAt: time.Now().UTC(),
	}
	for _, q := range d.QuarterlyProfitLoss {
		out.Deduction.QuarterlyProfitLoss = append(out.Deduction.QuarterlyProfitLoss, domain.RIAQuarterProfitLoss{
			Quarter:            q.Quarter,
			Weight:             q.Weight,
			TotalProfitLoss:    q.TotalProfitLoss,
			WeightedProfitLoss: q.WeightedTotalProfitLoss,
		})
	}

	var limitEnvelope quoteEnvelope[riaLimitRaw]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+"/api/v1/ria-calculator/limit", key, &limitEnvelope); err == nil {
		l := limitEnvelope.Result
		limit := domain.RIALimit{
			TotalLimit:              l.TotalLimit,
			RemainingLimit:          l.RemainingLimit,
			OverseasStockSellAmount: l.OverseasStockSellAmount,
			Settled:                 l.Settled,
		}
		if l.SettlementDate != nil {
			limit.SettlementDate = *l.SettlementDate
		}
		out.Limit = &limit
	}

	var optEnvelope quoteEnvelope[riaOptimizedRaw]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+"/api/v1/ria-calculator/tax-savings/optimized", key, &optEnvelope); err == nil {
		saving := optEnvelope.Result.EstimatedMaxTaxSaving
		out.MaxTaxSaving = &saving
		out.ZeroReasonCode = optEnvelope.Result.ZeroReasonCode
	}

	return out, nil
}
