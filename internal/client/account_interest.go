package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type interestPaymentRaw struct {
	Date          string  `json:"date"`
	TotalAmount   float64 `json:"totalAmount"`
	Tax           float64 `json:"tax"`
	PaymentAmount float64 `json:"paymentAmount"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	Estimated     bool    `json:"estimated"`
}

type interestMonthRaw struct {
	Month         int                  `json:"month"`
	TotalInterest float64              `json:"totalInterest"`
	Details       []interestPaymentRaw `json:"details"`
}

type accountInterestRaw struct {
	Year            int                `json:"year"`
	TotalInterest   float64            `json:"totalInterest"`
	MonthlySchedule []interestMonthRaw `json:"monthlySchedule"`
}

const interestBasePath = "/api/v1/interest/accounts/annual/history"

// GetInterestYears returns the years that have a deposit-interest record,
// oldest first — the server's own list, so callers don't have to guess how far
// back an account goes.
func (c *Client) GetInterestYears(ctx context.Context) ([]int, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return nil, err
	}

	var envelope quoteEnvelope[[]int]
	endpoint := c.certBaseURL + interestBasePath + "/years"
	if err := c.getJSONWithAccountKey(ctx, endpoint, key, &envelope); err != nil {
		return nil, err
	}
	return envelope.Result, nil
}

// GetAccountInterest returns one year's deposit-interest (예탁금 이용료)
// payments. 공식 Open API 에 없는 web 전용 기능.
//
// The months the server returns are payment months, but each payment carries
// the accrual period it covers (StartDate/EndDate) — those routinely differ,
// so the two must not be conflated.
func (c *Client) GetAccountInterest(ctx context.Context, year int) (domain.AccountInterest, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccountInterest{}, err
	}
	if year <= 0 {
		year = time.Now().Year()
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.AccountInterest{}, err
	}

	var envelope quoteEnvelope[accountInterestRaw]
	endpoint := fmt.Sprintf("%s%s/by-payment-date?year=%d", c.certBaseURL, interestBasePath, year)
	if err := c.getJSONWithAccountKey(ctx, endpoint, key, &envelope); err != nil {
		return domain.AccountInterest{}, err
	}
	r := envelope.Result

	out := domain.AccountInterest{
		Year:      year,
		Total:     r.TotalInterest,
		FetchedAt: time.Now().UTC(),
	}
	if r.Year != 0 {
		out.Year = r.Year
	}
	for _, m := range r.MonthlySchedule {
		month := domain.InterestMonth{Month: m.Month, Total: m.TotalInterest}
		for _, d := range m.Details {
			month.Payments = append(month.Payments, domain.InterestPayment{
				Date:          d.Date,
				Amount:        d.TotalAmount,
				Tax:           d.Tax,
				PaymentAmount: d.PaymentAmount,
				StartDate:     d.StartDate,
				EndDate:       d.EndDate,
				Estimated:     d.Estimated,
			})
		}
		out.Monthly = append(out.Monthly, month)
	}
	return out, nil
}
