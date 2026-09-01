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

const (
	interestYearsPath         = "/api/v1/interest/accounts/annual/history/years"
	interestByPaymentDatePath = "/api/v1/interest/accounts/annual/history/by-payment-date"
)

const (
	interestYearsCacheTTL          = 15 * time.Minute
	interestYearsEnrichmentTimeout = 750 * time.Millisecond
)

type interestYearsCacheEntry struct {
	years     []int
	expiresAt time.Time
}

// interestYearsCall is one shared lookup. Each caller keeps its own context;
// the HTTP request is cancelled only after the last waiter leaves.
type interestYearsCall struct {
	done     chan struct{}
	cancel   context.CancelFunc
	waiters  int
	complete bool
	years    []int
	err      error
}

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
	return c.getInterestYearsCached(ctx, key)
}

func (c *Client) getInterestYears(ctx context.Context, accountKey string) ([]int, error) {
	var envelope quoteEnvelope[[]int]
	endpoint := c.certBaseURL + interestYearsPath
	if err := c.getJSONWithAccountKey(ctx, endpoint, accountKey, &envelope); err != nil {
		return nil, err
	}
	return envelope.Result, nil
}

func (c *Client) getInterestYearsCached(ctx context.Context, accountKey string) ([]int, error) {
	if years, ok := c.cachedInterestYears(accountKey, time.Now()); ok {
		return years, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.interestYearsMu.Lock()
	if entry, ok := c.interestYearsCache[accountKey]; ok && time.Now().Before(entry.expiresAt) {
		years := append([]int(nil), entry.years...)
		c.interestYearsMu.Unlock()
		return years, nil
	}
	call := c.interestYearsCalls[accountKey]
	if call == nil {
		fetchCtx, cancel := context.WithCancel(context.Background())
		call = &interestYearsCall{done: make(chan struct{}), cancel: cancel}
		c.interestYearsCalls[accountKey] = call
		go c.runInterestYearsCall(fetchCtx, accountKey, call)
	}
	call.waiters++
	c.interestYearsMu.Unlock()

	select {
	case <-ctx.Done():
		c.releaseInterestYearsCall(accountKey, call)
		return nil, ctx.Err()
	case <-call.done:
		c.interestYearsMu.Lock()
		years, err := append([]int(nil), call.years...), call.err
		c.interestYearsMu.Unlock()
		c.releaseInterestYearsCall(accountKey, call)
		if err != nil {
			return nil, err
		}
		return years, nil
	}
}

func (c *Client) runInterestYearsCall(ctx context.Context, accountKey string, call *interestYearsCall) {
	years, err := c.getInterestYears(ctx, accountKey)
	call.cancel()
	stored := append([]int(nil), years...)

	c.interestYearsMu.Lock()
	call.years, call.err, call.complete = stored, err, true
	if err == nil {
		c.interestYearsCache[accountKey] = interestYearsCacheEntry{
			years: append([]int(nil), stored...), expiresAt: time.Now().Add(interestYearsCacheTTL),
		}
	}
	if c.interestYearsCalls[accountKey] == call {
		delete(c.interestYearsCalls, accountKey)
	}
	close(call.done)
	c.interestYearsMu.Unlock()
}

func (c *Client) releaseInterestYearsCall(accountKey string, call *interestYearsCall) {
	c.interestYearsMu.Lock()
	defer c.interestYearsMu.Unlock()
	call.waiters--
	if call.waiters == 0 && !call.complete {
		if c.interestYearsCalls[accountKey] == call {
			delete(c.interestYearsCalls, accountKey)
		}
		call.cancel()
	}
}

func (c *Client) cachedInterestYears(accountKey string, now time.Time) ([]int, bool) {
	c.interestYearsMu.Lock()
	defer c.interestYearsMu.Unlock()
	entry, ok := c.interestYearsCache[accountKey]
	if !ok || !now.Before(entry.expiresAt) {
		if ok {
			delete(c.interestYearsCache, accountKey)
		}
		return nil, false
	}
	return append([]int(nil), entry.years...), true
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
	endpoint := fmt.Sprintf("%s%s?year=%d", c.certBaseURL, interestByPaymentDatePath, year)
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
	// An empty year is common before account opening or before interest starts.
	// Enrich it with the server's authoritative year list so CLI and ops expose
	// the same meaning. The main report remains useful if this best-effort second
	// read fails, and the already-resolved account key avoids another account-list
	// request.
	if !accountInterestHasPayments(out) {
		yearsCtx, cancel := context.WithTimeout(ctx, interestYearsEnrichmentTimeout)
		defer cancel()
		if years, yearsErr := c.getInterestYearsCached(yearsCtx, key); yearsErr == nil {
			out.AvailableYears = years
		}
	}
	return out, nil
}

func accountInterestHasPayments(interest domain.AccountInterest) bool {
	for _, month := range interest.Monthly {
		if len(month.Payments) > 0 {
			return true
		}
	}
	return false
}
