// Package briefing composes existing read endpoints into a holdings briefing.
package briefing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type Reader interface {
	ListPositions(context.Context) ([]domain.Position, error)
	ListPendingOrders(context.Context) ([]domain.Order, error)
	GetEarningCalls(context.Context) (domain.EarningCalls, error)
	GetMarketNews(context.Context, string, int) (domain.MarketNews, error)
}

type Section struct {
	Status    string `json:"status"`
	FetchedAt string `json:"fetched_at"`
	Warning   string `json:"warning,omitempty"`
}

type Result struct {
	Source        string               `json:"source"`
	CollectedAt   string               `json:"collected_at"`
	Partial       bool                 `json:"partial"`
	Positions     []domain.Position    `json:"positions"`
	Earnings      []domain.EarningCall `json:"earnings"`
	News          []domain.NewsItem    `json:"news"`
	PendingOrders []domain.Order       `json:"pending_orders"`
	Sections      map[string]Section   `json:"sections"`
	Coverage      string               `json:"coverage"`
}

func Collect(ctx context.Context, reader Reader, newsLimit int) (Result, error) {
	out := Result{Source: "wts", CollectedAt: time.Now().UTC().Format(time.RFC3339Nano), Positions: []domain.Position{}, Earnings: []domain.EarningCall{}, News: []domain.NewsItem{}, PendingOrders: []domain.Order{}, Sections: map[string]Section{}, Coverage: fmt.Sprintf("Current WTS session holdings; upcoming earnings and pending orders matched by product code or symbol. News uses the holdings feed, newest %d articles (server maximum 50); it is not exhaustive. Sections are fetched independently, not as an atomic account snapshot.", newsLimit)}
	if newsLimit < 1 || newsLimit > tossclient.MaxNewsLimit {
		return out, fmt.Errorf("news-limit must be 1..%d", tossclient.MaxNewsLimit)
	}
	if reader == nil {
		return out, fmt.Errorf("portfolio briefing needs a Toss web session")
	}
	positions, err := reader.ListPositions(ctx)
	if err != nil {
		return out, err
	}
	out.Positions = positions
	out.Sections["positions"] = Section{Status: "ok", FetchedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	keys := map[string]bool{}
	for _, p := range positions {
		if p.ProductCode != "" {
			keys[stockKey(p.ProductCode)] = true
		}
		if p.Symbol != "" {
			keys[stockKey(p.Symbol)] = true
		}
	}
	var orders []domain.Order
	var earnings domain.EarningCalls
	var news domain.MarketNews
	var orderErr, earningErr, newsErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); orders, orderErr = reader.ListPendingOrders(ctx) }()
	go func() { defer wg.Done(); earnings, earningErr = reader.GetEarningCalls(ctx) }()
	go func() {
		defer wg.Done()
		scope, _ := tossclient.NewsScope("holdings")
		news, newsErr = reader.GetMarketNews(ctx, scope, newsLimit)
	}()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	for name, err := range map[string]error{"pending_orders": orderErr, "earnings": earningErr, "news": newsErr} {
		section := Section{Status: "ok", FetchedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if err != nil {
			out.Partial = true
			section.Status = "error"
			section.Warning = "Upstream read failed; this section is unavailable, not empty. Retry the corresponding standalone command for diagnostics."
		}
		out.Sections[name] = section
	}
	if orderErr == nil {
		for _, o := range orders {
			if keys[stockKey(o.Symbol)] {
				o.Raw = nil
				out.PendingOrders = append(out.PendingOrders, o)
			}
		}
	}
	if earningErr == nil {
		for _, e := range earnings.Events {
			if keys[stockKey(e.CompanyCode)] {
				out.Earnings = append(out.Earnings, e)
			}
		}
	}
	if newsErr == nil {
		out.News = append(out.News, news.Items...)
	}
	return out, nil
}

func stockKey(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) == 7 && s[0] == 'A' {
		digits := true
		for _, r := range s[1:] {
			if r < '0' || r > '9' {
				digits = false
				break
			}
		}
		if digits {
			return s[1:]
		}
	}
	return s
}
