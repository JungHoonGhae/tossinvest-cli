package hiddenholding

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type fakeClient struct {
	holdings         []domain.HiddenHolding
	hideCalls        int
	showCalls        int
	hideAppliedError error
	showAppliedError error
}

func (f *fakeClient) ResolveProductCode(_ context.Context, symbol string) (string, error) {
	if symbol == "삼성전자" {
		return "A005930", nil
	}
	return symbol, nil
}

func (f *fakeClient) ListHiddenHoldings(_ context.Context, accountKey string) (domain.HiddenHoldings, error) {
	if accountKey == "" {
		accountKey = "primary-test"
	}
	return domain.HiddenHoldings{AccountKey: accountKey, AccountScope: "scope-test", Holdings: append([]domain.HiddenHolding(nil), f.holdings...)}, nil
}

func (f *fakeClient) HideHolding(_ context.Context, _ string, productCode string) error {
	f.hideCalls++
	f.holdings = append(f.holdings, domain.HiddenHolding{ProductCode: productCode})
	return f.hideAppliedError
}

func (f *fakeClient) ShowHolding(_ context.Context, _ string, productCode string) error {
	f.showCalls++
	for i, holding := range f.holdings {
		if holding.ProductCode == productCode {
			f.holdings = append(f.holdings[:i], f.holdings[i+1:]...)
			break
		}
	}
	return f.showAppliedError
}

func TestChangePreviewConfirmationAndAccountScope(t *testing.T) {
	t.Parallel()
	f := &fakeClient{}
	s := NewService(f)
	preview, err := s.Change(context.Background(), ActionHide, "삼성전자", "", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ProductCode != "A005930" || preview.AccountScope == "" || strings.Contains(preview.Canonical, "primary-test") || preview.ConfirmToken == "" || f.hideCalls != 0 {
		t.Fatalf("preview=%#v calls=%d", preview, f.hideCalls)
	}
	if _, err := s.Change(context.Background(), ActionHide, "삼성전자", "", ExecuteOptions{Execute: true, Confirm: "wrong"}); err == nil {
		t.Fatal("wrong confirmation was accepted")
	}
	result, err := s.Change(context.Background(), ActionHide, "삼성전자", "", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Reconciled || f.hideCalls != 1 || len(f.holdings) != 1 {
		t.Fatalf("result=%#v fake=%#v", result, f)
	}
}

func TestListReplacesRawAccountKeyWithSafeScopeForSerialization(t *testing.T) {
	t.Parallel()
	result, err := NewService(&fakeClient{}).List(context.Background(), "primary-test")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccountKey != "primary-test" || result.AccountScope == "" || result.AccountScope == result.AccountKey {
		t.Fatalf("unsafe or missing account scope: %#v", result)
	}
}

func TestChangeRejectsStalePreviewAndNoops(t *testing.T) {
	t.Parallel()
	f := &fakeClient{}
	s := NewService(f)
	preview, err := s.Change(context.Background(), ActionHide, "A005930", "", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f.holdings = append(f.holdings, domain.HiddenHolding{ProductCode: "US.AAPL"})
	if _, err := s.Change(context.Background(), ActionHide, "A005930", "", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken}); err == nil {
		t.Fatal("stale preview was accepted")
	}
	f.holdings = []domain.HiddenHolding{{ProductCode: "A005930"}}
	noop, err := s.Change(context.Background(), ActionHide, "A005930", "", ExecuteOptions{})
	if err != nil || !noop.Noop {
		t.Fatalf("noop=%#v err=%v", noop, err)
	}
	result, err := s.Change(context.Background(), ActionHide, "A005930", "", ExecuteOptions{Execute: true, Confirm: noop.ConfirmToken})
	if err != nil || !result.Applied || f.hideCalls != 0 {
		t.Fatalf("result=%#v err=%v calls=%d", result, err, f.hideCalls)
	}
}

func TestChangeReconcilesAppliedTransportError(t *testing.T) {
	t.Parallel()
	f := &fakeClient{showAppliedError: errors.New("response lost"), holdings: []domain.HiddenHolding{{ProductCode: "A005930"}}}
	s := NewService(f)
	preview, err := s.Change(context.Background(), ActionShow, "A005930", "", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Change(context.Background(), ActionShow, "A005930", "", ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Reconciled || f.showCalls != 1 || len(f.holdings) != 0 {
		t.Fatalf("result=%#v fake=%#v", result, f)
	}
}
