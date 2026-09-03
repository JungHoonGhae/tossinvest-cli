package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	dividendsHistoryPath              = "/api/v1/dividends/accounts/annual/history"
	dividendsHistoryByPaymentDatePath = "/api/v1/dividends/accounts/annual/history/by-payment-date"
)

type divAmt struct {
	KRW float64 `json:"krw"`
	USD float64 `json:"usd"`
}

type divBucket struct {
	TotalAmount     divAmt  `json:"totalAmount"`
	PaidAmount      divAmt  `json:"paidAmount"`
	EstimatedAmount divAmt  `json:"estimatedAmount"`
	TotalTax        *divAmt `json:"totalTax"`
	TotalCommission *divAmt `json:"totalCommission"`
}

type divMonthRaw struct {
	Month   int       `json:"month"`
	Summary divBucket `json:"summary"`
	Details []struct {
		ProductCode string  `json:"productCode"`
		ProductName string  `json:"productName"`
		Quantity    float64 `json:"quantity"`
		Amount      divAmt  `json:"amount"`
	} `json:"details"`
}

type dividendsRaw struct {
	Summary       divBucket            `json:"summary"`
	RegionSummary map[string]divBucket `json:"regionSummary"`
	Calendar      struct {
		Year            int           `json:"year"`
		MonthlySchedule []divMonthRaw `json:"monthlySchedule"`
	} `json:"calendar"`
}

func divSummary(b divBucket, withTax bool) domain.DividendSummary {
	s := domain.DividendSummary{
		Total:     domain.DividendAmount(b.TotalAmount),
		Paid:      domain.DividendAmount(b.PaidAmount),
		Estimated: domain.DividendAmount(b.EstimatedAmount),
	}
	if withTax {
		if b.TotalTax != nil {
			t := domain.DividendAmount(*b.TotalTax)
			s.Tax = &t
		}
		if b.TotalCommission != nil {
			cm := domain.DividendAmount(*b.TotalCommission)
			s.Commission = &cm
		}
	}
	return s
}

// GetDividends returns an account's annual dividend report. byPaymentDate
// switches from the ex-date view to the payment-date view (which also carries
// tax/commission). 공식 API 에 없는 web 전용 기능.
func (c *Client) GetDividends(ctx context.Context, year int, byPaymentDate bool) (domain.Dividends, error) {
	if err := c.requireSession(); err != nil {
		return domain.Dividends{}, err
	}
	if year <= 0 {
		year = time.Now().Year()
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.Dividends{}, err
	}

	path := dividendsHistoryPath
	if byPaymentDate {
		path = dividendsHistoryByPaymentDatePath
	}
	endpoint := fmt.Sprintf("%s%s?year=%d", c.certBaseURL, path, year)

	var envelope quoteEnvelope[dividendsRaw]
	if err := c.getJSONWithAccountKey(ctx, endpoint, key, &envelope); err != nil {
		return domain.Dividends{}, err
	}
	r := envelope.Result

	out := domain.Dividends{
		Year:          year,
		ByPaymentDate: byPaymentDate,
		Summary:       divSummary(r.Summary, byPaymentDate),
		FetchedAt:     time.Now().UTC(),
	}
	if r.Calendar.Year != 0 {
		out.Year = r.Calendar.Year
	}
	// Stable region order: kr, us.
	for _, region := range []string{"kr", "us"} {
		b, ok := r.RegionSummary[region]
		if !ok {
			continue
		}
		out.Regions = append(out.Regions, domain.DividendRegion{
			Region:  region,
			Summary: divSummary(b, byPaymentDate),
		})
	}
	for _, m := range r.Calendar.MonthlySchedule {
		dm := domain.DividendMonth{Month: m.Month, Summary: divSummary(m.Summary, false)}
		for _, d := range m.Details {
			dm.Stocks = append(dm.Stocks, domain.DividendStock{
				ProductCode: d.ProductCode,
				Name:        d.ProductName,
				Quantity:    d.Quantity,
				Amount:      domain.DividendAmount(d.Amount),
			})
		}
		out.Monthly = append(out.Monthly, dm)
	}
	return out, nil
}
