package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type primeInfoRaw struct {
	IsMember        bool    `json:"isMember"`
	UserID          *string `json:"userId"`
	PrimeType       *string `json:"primeType"`
	BenefitsStartAt *string `json:"benefitsStartAt"`
	BenefitsEndAt   *string `json:"benefitsEndAt"`
	CycleNumber     *int    `json:"cycleNumber"`
}

type primeInterestTierRaw struct {
	Status           string `json:"status"`
	NonPrimeInterest int    `json:"nonPrimeInterest"`
	PrimeInterest    int    `json:"primeInterest"`
	BenefitInterest  int    `json:"benefitInterest"`
}

type primeBenefitsRaw struct {
	Month           string  `json:"month"`
	BenefitsStartAt *string `json:"benefitsStartAt"`
	BenefitsEndAt   *string `json:"benefitsEndAt"`
	Exchange        struct {
		NonPrimeFee int `json:"nonPrimeFee"`
		PrimeFee    int `json:"primeFee"`
		BenefitFee  int `json:"benefitFee"`
	} `json:"exchange"`
	InterestKRW     primeInterestTierRaw `json:"interestKrw"`
	InterestUSD     primeInterestTierRaw `json:"interestUsd"`
	BaseRate        float64              `json:"baseRate"`
	MonthlyTotalKRW float64              `json:"monthlyTotalKrw"`
}

// GetPrimeStatus returns the account's Toss Prime membership state and this
// month's fee/interest benefit comparison. 공식 Open API 에 없는 tossctl 고유
// 기능 — non-member accounts still get a real benefit comparison back
// (verified live), so this never returns a "not enrolled" error the way
// GetDividends-adjacent endpoints can.
func (c *Client) GetPrimeStatus(ctx context.Context) (domain.PrimeStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.PrimeStatus{}, err
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.PrimeStatus{}, err
	}

	var infoEnvelope quoteEnvelope[primeInfoRaw]
	infoURL := c.certBaseURL + "/api/v1/prime/users/info"
	if err := c.getJSONWithAccountKey(ctx, infoURL, key, &infoEnvelope); err != nil {
		return domain.PrimeStatus{}, fmt.Errorf("prime info: %w", err)
	}

	var benefitsEnvelope quoteEnvelope[primeBenefitsRaw]
	benefitsURL := c.certBaseURL + "/api/v1/prime/users/benefits"
	if err := c.getJSONWithAccountKey(ctx, benefitsURL, key, &benefitsEnvelope); err != nil {
		return domain.PrimeStatus{}, fmt.Errorf("prime benefits: %w", err)
	}

	// 누적 혜택. 나머지 두 호출과 같은 기준으로 에러를 올린다 — 조용히 0 으로 두면
	// "혜택이 없다" 와 "못 가져왔다" 가 구별되지 않는다.
	var cumEnvelope quoteEnvelope[struct {
		Exchange    float64 `json:"exchangeBenefit"`
		InterestKRW float64 `json:"interestBenefitKrw"`
		InterestUSD float64 `json:"interestBenefitUsd"`
		TotalKRW    float64 `json:"totalKrw"`
	}]
	cumURL := c.certBaseURL + "/api/v1/prime/users/benefits/cumulative"
	if err := c.getJSONWithAccountKey(ctx, cumURL, key, &cumEnvelope); err != nil {
		return domain.PrimeStatus{}, fmt.Errorf("prime cumulative: %w", err)
	}

	info := infoEnvelope.Result
	benefits := benefitsEnvelope.Result
	cum := cumEnvelope.Result

	return domain.PrimeStatus{
		IsMember:        info.IsMember,
		UserID:          info.UserID,
		PrimeType:       info.PrimeType,
		BenefitsStartAt: info.BenefitsStartAt,
		BenefitsEndAt:   info.BenefitsEndAt,
		CycleNumber:     info.CycleNumber,
		Month:           benefits.Month,
		Exchange: domain.PrimeExchangeFee{
			NonPrimeFee: benefits.Exchange.NonPrimeFee,
			PrimeFee:    benefits.Exchange.PrimeFee,
			BenefitFee:  benefits.Exchange.BenefitFee,
		},
		InterestKRW: domain.PrimeInterestTier{
			Status:           benefits.InterestKRW.Status,
			NonPrimeInterest: benefits.InterestKRW.NonPrimeInterest,
			PrimeInterest:    benefits.InterestKRW.PrimeInterest,
			BenefitInterest:  benefits.InterestKRW.BenefitInterest,
		},
		InterestUSD: domain.PrimeInterestTier{
			Status:           benefits.InterestUSD.Status,
			NonPrimeInterest: benefits.InterestUSD.NonPrimeInterest,
			PrimeInterest:    benefits.InterestUSD.PrimeInterest,
			BenefitInterest:  benefits.InterestUSD.BenefitInterest,
		},
		BaseRate:        benefits.BaseRate,
		MonthlyTotalKRW: benefits.MonthlyTotalKRW,
		Cumulative: domain.PrimeCumulative{
			Exchange:    cum.Exchange,
			InterestKRW: cum.InterestKRW,
			InterestUSD: cum.InterestUSD,
			TotalKRW:    cum.TotalKRW,
		},
		FetchedAt: time.Now().UTC(),
	}, nil
}
