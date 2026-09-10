package briefing

import (
	"context"
	"errors"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type reader struct{ failNews bool }

func (*reader) ListPositions(context.Context) ([]domain.Position, error) {
	return []domain.Position{{ProductCode: "A005930", Symbol: "005930", Quantity: 1}, {ProductCode: "US-AAPL", Symbol: "AAPL", Quantity: 0.5}}, nil
}
func (*reader) ListPendingOrders(context.Context) ([]domain.Order, error) {
	return []domain.Order{{Symbol: "A005930", ID: "one", Raw: []byte(`{"private":"value"}`)}, {Symbol: "AAPL", ID: "two"}, {Symbol: "MSFT", ID: "not-held"}}, nil
}
func (*reader) GetEarningCalls(context.Context) (domain.EarningCalls, error) {
	return domain.EarningCalls{Events: []domain.EarningCall{{CompanyCode: "005930"}, {CompanyCode: "US-AAPL"}, {CompanyCode: "MSFT"}}}, nil
}
func (r *reader) GetMarketNews(_ context.Context, scope string, limit int) (domain.MarketNews, error) {
	if r.failNews {
		return domain.MarketNews{}, errors.New("network error")
	}
	if scope != "PERSONALIZE_HOLD" || limit != 10 {
		return domain.MarketNews{}, errors.New("wrong scope or limit")
	}
	return domain.MarketNews{Items: []domain.NewsItem{{ID: "news", Title: "Holdings news"}}}, nil
}

func TestHoldingsBriefingMatchesCodesAndLabelsPartialSections(t *testing.T) {
	for _, fail := range []bool{false, true} {
		data, err := Collect(context.Background(), &reader{failNews: fail}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(data.Positions) != 2 || len(data.Earnings) != 2 || len(data.PendingOrders) != 2 || data.Partial != fail {
			t.Fatalf("briefing=%+v", data)
		}
		if data.PendingOrders[0].Raw != nil {
			t.Fatal("raw order payload leaked")
		}
		if fail && (data.Sections["news"].Status != "error" || data.Sections["news"].Warning == "" || len(data.News) != 0) {
			t.Fatalf("missing section reported as empty success: %+v", data)
		}
		if !fail && len(data.News) != 1 {
			t.Fatalf("news=%+v", data.News)
		}
	}
	if _, err := Collect(context.Background(), &reader{}, 51); err == nil {
		t.Fatal("invalid limit accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Collect(ctx, &reader{}, 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
