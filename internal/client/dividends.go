package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	dividendsHistoryPath              = "/api/v1/dividends/accounts/annual/history"
	dividendsHistoryByPaymentDatePath = "/api/v1/dividends/accounts/annual/history/by-payment-date"
)

// getJSONWithAccountKey is getJSON with the account-scoping header the web app
// sends on per-account endpoints (dividends 등).
func (c *Client) getJSONWithAccountKey(ctx context.Context, endpoint, accountKey string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.applySession(req)
	if accountKey != "" {
		req.Header.Set("accountKey", accountKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, data)
	}
	return json.Unmarshal(data, target)
}

// primaryAccountKey returns the primary account's key for account-scoped calls.
func (c *Client) primaryAccountKey(ctx context.Context) (string, error) {
	accounts, primary, err := c.ListAccounts(ctx)
	if err != nil {
		return "", err
	}
	if primary != "" {
		return primary, nil
	}
	if len(accounts) > 0 {
		return accounts[0].ID, nil
	}
	return "", fmt.Errorf("no account found")
}

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
