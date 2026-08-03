package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type commissionTierRaw struct {
	CommissionRate         float64 `json:"commissionRate"`
	CommissionPerContract  float64 `json:"commissionPerContract"`
	HasCommissionReduction bool    `json:"hasCommissionReduction"`
	ReductionEndDateTime   string  `json:"reductionEndDateTime"`
}

type commissionInfoRaw struct {
	Kr    commissionTierRaw  `json:"commissionInfoKr"`
	Us    commissionTierRaw  `json:"commissionInfoUs"`
	UsOpt *commissionTierRaw `json:"commissionInfoUsOpt"`
}

func (r commissionTierRaw) toDomain() domain.CommissionTier {
	tier := domain.CommissionTier{
		RatePercent:  r.CommissionRate,
		PerContract:  r.CommissionPerContract,
		HasReduction: r.HasCommissionReduction,
	}
	// The end date is only meaningful while a reduction is in effect. With
	// hasCommissionReduction=false the API still sends a date — sometimes a
	// stale past one, sometimes the sentinel 9999-12-31 — and passing that
	// through reads like an active promotion that isn't there.
	if r.HasCommissionReduction {
		tier.ReductionEndAt = r.ReductionEndDateTime
	}
	return tier
}

// GetCommissionSchedule returns the account's commission rates per trading
// surface (KR equities, US equities, US options).
//
// v2, not v1: both return 200 and the same KR/US tiers, but v1 always reports
// commissionInfoUsOpt as null while v2 fills it in. The web app calls v2 from
// the options screen for exactly that reason. Confirmed live 2026-08-04.
func (c *Client) GetCommissionSchedule(ctx context.Context) (domain.CommissionSchedule, error) {
	if err := c.requireSession(); err != nil {
		return domain.CommissionSchedule{}, err
	}

	var envelope quoteEnvelope[commissionInfoRaw]
	url := c.apiBaseURL + "/api/v2/trading/commission-info"
	if err := c.getJSON(ctx, url, &envelope); err != nil {
		return domain.CommissionSchedule{}, err
	}

	schedule := domain.CommissionSchedule{
		Korea:     envelope.Result.Kr.toDomain(),
		US:        envelope.Result.Us.toDomain(),
		FetchedAt: time.Now().UTC(),
	}
	// null on accounts without the US options agreement — omitted rather than
	// rendered as a zero-fee tier.
	if envelope.Result.UsOpt != nil {
		tier := envelope.Result.UsOpt.toDomain()
		schedule.USOptions = &tier
	}
	return schedule, nil
}
