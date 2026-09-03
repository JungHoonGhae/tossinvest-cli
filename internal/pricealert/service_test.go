package pricealert

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type fakeClient struct {
	alerts             []domain.PriceAlert
	addCalls           int
	deleteCalls        int
	addAppliedError    error
	deleteAppliedError error
}

func (f *fakeClient) ResolveProductCode(_ context.Context, symbol string) (string, error) {
	if symbol == "삼성전자" {
		return "A005930", nil
	}
	return symbol, nil
}

func (f *fakeClient) ListPriceAlerts(_ context.Context, productCode string) (domain.PriceAlerts, error) {
	return domain.PriceAlerts{ProductCode: productCode, Alerts: append([]domain.PriceAlert(nil), f.alerts...)}, nil
}

func (f *fakeClient) AddPriceAlert(_ context.Context, _ string, targetPrice float64, currency string) error {
	f.addCalls++
	f.alerts = append(f.alerts, domain.PriceAlert{TargetPrice: targetPrice, Currency: currency})
	return f.addAppliedError
}

func (f *fakeClient) DeletePriceAlert(_ context.Context, _ string, targetPrice float64, currency string) error {
	f.deleteCalls++
	for i, alert := range f.alerts {
		if alert.TargetPrice == targetPrice && alert.Currency == currency {
			f.alerts = append(f.alerts[:i], f.alerts[i+1:]...)
			break
		}
	}
	return f.deleteAppliedError
}

func TestChangePreviewConfirmationAndVerification(t *testing.T) {
	t.Parallel()
	f := &fakeClient{}
	s := NewService(f)

	preview, err := s.Change(context.Background(), ActionAdd, "삼성전자", 70000, "krw", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ProductCode != "A005930" || preview.Currency != "KRW" || preview.ConfirmToken == "" || preview.Applied || f.addCalls != 0 {
		t.Fatalf("preview = %#v, addCalls=%d", preview, f.addCalls)
	}
	if _, err := s.Change(context.Background(), ActionAdd, "삼성전자", 70000, "KRW", ExecuteOptions{Execute: true, Confirm: "wrong"}); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("wrong confirmation error = %v", err)
	}
	if f.addCalls != 0 {
		t.Fatalf("wrong confirmation mutated %d times", f.addCalls)
	}
	result, err := s.Change(context.Background(), ActionAdd, "삼성전자", 70000, "KRW", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Reconciled || f.addCalls != 1 || len(f.alerts) != 1 {
		t.Fatalf("result=%#v fake=%#v", result, f)
	}
}

func TestChangeRejectsStalePreviewAndHandlesNoop(t *testing.T) {
	t.Parallel()
	f := &fakeClient{}
	s := NewService(f)
	preview, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f.alerts = append(f.alerts, domain.PriceAlert{TargetPrice: 71000, Currency: "KRW"})
	if _, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken}); err == nil {
		t.Fatal("stale preview was accepted")
	}

	f.alerts = []domain.PriceAlert{{TargetPrice: 70000, Currency: "KRW"}}
	noop, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{})
	if err != nil || !noop.Noop {
		t.Fatalf("noop preview=%#v err=%v", noop, err)
	}
	result, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{Execute: true, Confirm: noop.ConfirmToken})
	if err != nil || !result.Applied || f.addCalls != 0 {
		t.Fatalf("noop result=%#v err=%v calls=%d", result, err, f.addCalls)
	}
}

func TestChangeReconcilesAppliedTransportError(t *testing.T) {
	t.Parallel()
	f := &fakeClient{addAppliedError: errors.New("response lost")}
	s := NewService(f)
	preview, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Change(context.Background(), ActionAdd, "A005930", 70000, "KRW", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !result.Applied || !result.Reconciled || f.addCalls != 1 {
		t.Fatalf("result=%#v calls=%d", result, f.addCalls)
	}
}

func TestChangeValidatesInputsAndLimit(t *testing.T) {
	t.Parallel()
	s := NewService(&fakeClient{})
	for _, tc := range []struct {
		price    float64
		currency string
	}{
		{0, "KRW"},
		{-1, "KRW"},
		{1, "EUR"},
	} {
		if _, err := s.Change(context.Background(), ActionAdd, "A005930", tc.price, tc.currency, ExecuteOptions{}); err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
	f := &fakeClient{}
	for i := 0; i < 10; i++ {
		f.alerts = append(f.alerts, domain.PriceAlert{TargetPrice: float64(i + 1), Currency: "KRW"})
	}
	if _, err := NewService(f).Change(context.Background(), ActionAdd, "A005930", 99, "KRW", ExecuteOptions{}); err == nil || !strings.Contains(err.Error(), "10") {
		t.Fatalf("limit error = %v", err)
	}
}
