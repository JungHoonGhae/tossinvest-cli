package client

import (
	"context"
	"sync"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// The 계좌관리 screen is assembled from several small reads across two hosts.
// They are independent, so they are fetched concurrently; only the identity
// read is required, because a margin endpoint being unavailable is no reason to
// withhold the account number. Failures of the optional parts are surfaced as
// warnings rather than swallowed.

type accountDetailRaw struct {
	No            string `json:"no"`
	Status        string `json:"status"`
	OpenDate      string `json:"openDate"`
	LastTradeDate string `json:"lastTradeDate"`
	AccountName   string `json:"accountName"`
}

type withdrawableRaw struct {
	WithdrawableAmount struct {
		Day0 float64 `json:"day0"`
		Day1 float64 `json:"day1"`
		Day2 float64 `json:"day2"`
	} `json:"withdrawableAmount"`
	WithdrawableAmountLimit struct {
		PerTransaction        float64 `json:"perTransaction"`
		PerDay                float64 `json:"perDay"`
		SumOfTodayWithdrawals float64 `json:"sumOfTodayWithdrawals"`
	} `json:"withdrawableAmountLimit"`
	PossibleDateOfFullWithdrawal string `json:"possibleDateOfFullWithdrawal"`
}

type marginOverviewRaw struct {
	KR struct {
		Receivable bool    `json:"receivable"`
		Message    *string `json:"message"`
	} `json:"kr"`
	US struct {
		Receivable bool    `json:"receivable"`
		Message    *string `json:"message"`
	} `json:"us"`
}

// GetAccountDetail assembles the read-only half of the 계좌관리 screen.
// WTS-only.
func (c *Client) GetAccountDetail(ctx context.Context) (domain.AccountDetail, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccountDetail{}, err
	}

	// Required: identity. Everything else is optional.
	var idEnv quoteEnvelope[accountDetailRaw]
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/account/detail", &idEnv); err != nil {
		return domain.AccountDetail{}, err
	}
	out := domain.AccountDetail{
		Number:       idEnv.Result.No,
		Name:         idEnv.Result.AccountName,
		Status:       idEnv.Result.Status,
		OpenedAt:     idEnv.Result.OpenDate,
		LastTradedAt: idEnv.Result.LastTradeDate,
		FetchedAt:    time.Now(),
	}

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		warn = func(what string, err error) {
			mu.Lock()
			defer mu.Unlock()
			out.Warnings = append(out.Warnings, what+": "+err.Error())
		}
	)

	wg.Add(4)

	go func() {
		defer wg.Done()
		var env quoteEnvelope[withdrawableRaw]
		if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/transfer/withdrawable-status", &env); err != nil {
			warn("출금 정보", err)
			return
		}
		r := env.Result
		mu.Lock()
		defer mu.Unlock()
		out.Withdrawable = &domain.WithdrawableByDay{
			Day0: r.WithdrawableAmount.Day0,
			Day1: r.WithdrawableAmount.Day1,
			Day2: r.WithdrawableAmount.Day2,
		}
		out.WithdrawalLimits = &domain.WithdrawalLimits{
			PerTransaction: r.WithdrawableAmountLimit.PerTransaction,
			PerDay:         r.WithdrawableAmountLimit.PerDay,
			UsedToday:      r.WithdrawableAmountLimit.SumOfTodayWithdrawals,
		}
		out.FullWithdrawalOn = r.PossibleDateOfFullWithdrawal
	}()

	go func() {
		defer wg.Done()
		var env quoteEnvelope[marginOverviewRaw]
		if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/dashboard/wts/overview/margin", &env); err != nil {
			warn("미수거래 상태", err)
			return
		}
		str := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		mu.Lock()
		defer mu.Unlock()
		out.MarginKR = &domain.MarginStatus{Receivable: env.Result.KR.Receivable, Message: str(env.Result.KR.Message)}
		out.MarginUS = &domain.MarginStatus{Receivable: env.Result.US.Receivable, Message: str(env.Result.US.Message)}
	}()

	go func() {
		defer wg.Done()
		var env quoteEnvelope[bool]
		if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/margin/cert/differential-margin/enabled", &env); err != nil {
			warn("차등증거금", err)
			return
		}
		v := env.Result
		mu.Lock()
		defer mu.Unlock()
		out.DifferentialMargin = &v
	}()

	go func() {
		defer wg.Done()
		var env quoteEnvelope[bool]
		if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/trade-purpose-verification/transfer-limit-restricted", &env); err != nil {
			warn("송금 한도 제한", err)
			return
		}
		v := env.Result
		mu.Lock()
		defer mu.Unlock()
		out.TransferRestricted = &v
	}()

	wg.Wait()
	return out, nil
}

// MaskAccountNumber hides the middle of an account number, keeping enough to
// recognise it. Output lands in terminals that get pasted into issues, so the
// full number is opt-in (see CLAUDE.md: no real account data in public output).
func MaskAccountNumber(no string) string {
	r := []rune(no)
	if len(r) <= 7 {
		return no
	}
	keepHead, keepTail := 6, 3
	masked := make([]rune, 0, len(r))
	masked = append(masked, r[:keepHead]...)
	for _, ch := range r[keepHead : len(r)-keepTail] {
		if ch == '-' {
			masked = append(masked, '-')
			continue
		}
		masked = append(masked, '*')
	}
	masked = append(masked, r[len(r)-keepTail:]...)
	return string(masked)
}
