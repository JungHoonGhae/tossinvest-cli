package hybrid

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// --- Test fakes -------------------------------------------------------------
//
// fakeWTS implements trading.Broker; fakeOfficial implements officialOrderer.
// Both count calls so the double-order guard can be asserted directly: after an
// official failure the corresponding WTS counter MUST remain 0.

type fakeWTS struct {
	placeCalls  int
	cancelCalls int
	amendCalls  int
	actionCalls int

	placeRes  trading.MutationResult
	cancelRes trading.MutationResult
	amendRes  trading.MutationResult
	actions   map[string]any
}

func (f *fakeWTS) PlacePendingOrder(_ context.Context, _ orderintent.PlaceIntent) (trading.MutationResult, error) {
	f.placeCalls++
	return f.placeRes, nil
}

func (f *fakeWTS) CancelPendingOrder(_ context.Context, _ orderintent.CancelIntent) (trading.MutationResult, error) {
	f.cancelCalls++
	return f.cancelRes, nil
}

func (f *fakeWTS) AmendPendingOrder(_ context.Context, _ orderintent.AmendIntent) (trading.MutationResult, error) {
	f.amendCalls++
	return f.amendRes, nil
}

func (f *fakeWTS) GetOrderAvailableActions(_ context.Context, _ string) (map[string]any, error) {
	f.actionCalls++
	return f.actions, nil
}

type fakeOfficial struct {
	placeCalls  int
	cancelCalls int
	modifyCalls int

	placeRes  trading.MutationResult
	placeErr  error
	cancelRes trading.MutationResult
	cancelErr error
	modifyRes trading.MutationResult
	modifyErr error
}

func (f *fakeOfficial) PlaceOrder(_ context.Context, _ orderintent.PlaceIntent) (trading.MutationResult, error) {
	f.placeCalls++
	return f.placeRes, f.placeErr
}

func (f *fakeOfficial) CancelOrder(_ context.Context, _ string) (trading.MutationResult, error) {
	f.cancelCalls++
	return f.cancelRes, f.cancelErr
}

func (f *fakeOfficial) ModifyOrder(_ context.Context, _ orderintent.AmendIntent) (trading.MutationResult, error) {
	f.modifyCalls++
	return f.modifyRes, f.modifyErr
}

// regularIntent is an official-eligible limit order (US, USD, non-fractional).
func regularIntent() orderintent.PlaceIntent {
	return orderintent.PlaceIntent{
		Symbol:       "AAPL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        100,
		CurrencyMode: "USD",
	}
}

// krwFractionalIntent is a KRW-settlement fractional order — WTS-unique, so the
// broker must route it to WTS and never touch the official path.
func krwFractionalIntent() orderintent.PlaceIntent {
	return orderintent.PlaceIntent{
		Symbol:       "AAPL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "market",
		Amount:       100000,
		CurrencyMode: "KRW",
		Fractional:   true,
	}
}

// --- Place routing ----------------------------------------------------------

func TestPlaceOffNilRoutesToWTS(t *testing.T) {
	fw := &fakeWTS{}
	b := &hybridBroker{wts: fw, off: nil, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	if _, err := b.PlacePendingOrder(context.Background(), regularIntent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.placeCalls != 1 {
		t.Fatalf("want WTS Place called once; got %d", fw.placeCalls)
	}
}

func TestPlacePreferWTSRoutesToWTS(t *testing.T) {
	fw := &fakeWTS{}
	fo := &fakeOfficial{}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "wts", Fallback: true}, stderr: io.Discard}

	if _, err := b.PlacePendingOrder(context.Background(), regularIntent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.placeCalls != 1 {
		t.Fatalf("want WTS Place called once; got %d", fw.placeCalls)
	}
	if fo.placeCalls != 0 {
		t.Fatalf("official must NOT be called when prefer=wts; got %d", fo.placeCalls)
	}
}

func TestPlaceKRWFractionalRoutesToWTS(t *testing.T) {
	fw := &fakeWTS{}
	fo := &fakeOfficial{}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	if _, err := b.PlacePendingOrder(context.Background(), krwFractionalIntent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.placeCalls != 1 {
		t.Fatalf("want WTS Place called once for KRW-fractional; got %d", fw.placeCalls)
	}
	if fo.placeCalls != 0 {
		t.Fatalf("official must NOT be called for KRW-fractional (WTS-unique); got %d", fo.placeCalls)
	}
}

func TestPlaceEligibleRoutesToOfficial(t *testing.T) {
	fw := &fakeWTS{}
	fo := &fakeOfficial{placeRes: trading.MutationResult{Kind: "place", OrderID: "OFF-1"}}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	res, err := b.PlacePendingOrder(context.Background(), regularIntent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID != "OFF-1" {
		t.Fatalf("want official result OFF-1; got %q", res.OrderID)
	}
	if fo.placeCalls != 1 {
		t.Fatalf("want official Place called once; got %d", fo.placeCalls)
	}
	if fw.placeCalls != 0 {
		t.Fatalf("WTS must NOT be called when official handles the order; got %d", fw.placeCalls)
	}
}

// TestPlaceOfficialErrorNoFallback is the double-order guard: when the official
// path fails, the broker MUST NOT retry via WTS (the order may already be in
// flight at the brokerage). The WTS Place counter must stay 0 and the returned
// error must wrap the underlying official error.
func TestPlaceOfficialErrorNoFallback(t *testing.T) {
	sentinel := errors.New("boom")
	fw := &fakeWTS{}
	fo := &fakeOfficial{placeErr: sentinel}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	_, err := b.PlacePendingOrder(context.Background(), regularIntent())
	if err == nil {
		t.Fatal("want error on official Place failure; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("returned error must wrap the official error; got %v", err)
	}
	if fo.placeCalls != 1 {
		t.Fatalf("want official Place attempted once; got %d", fo.placeCalls)
	}
	if fw.placeCalls != 0 {
		t.Fatalf("DOUBLE-ORDER GUARD VIOLATED: WTS Place called after official failure (%d)", fw.placeCalls)
	}
}

// --- Cancel routing ---------------------------------------------------------

func TestCancelOffNilRoutesToWTS(t *testing.T) {
	fw := &fakeWTS{}
	b := &hybridBroker{wts: fw, off: nil, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	if _, err := b.CancelPendingOrder(context.Background(), orderintent.CancelIntent{OrderID: "X"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.cancelCalls != 1 {
		t.Fatalf("want WTS Cancel called once; got %d", fw.cancelCalls)
	}
}

func TestCancelAutoRoutesToOfficial(t *testing.T) {
	fw := &fakeWTS{}
	fo := &fakeOfficial{cancelRes: trading.MutationResult{Kind: "cancel", OrderID: "C-1"}}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	res, err := b.CancelPendingOrder(context.Background(), orderintent.CancelIntent{OrderID: "X"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID != "C-1" {
		t.Fatalf("want official cancel result C-1; got %q", res.OrderID)
	}
	if fo.cancelCalls != 1 || fw.cancelCalls != 0 {
		t.Fatalf("want official Cancel once, WTS zero; got off=%d wts=%d", fo.cancelCalls, fw.cancelCalls)
	}
}

func TestCancelOfficialErrorNoFallback(t *testing.T) {
	sentinel := errors.New("cancel boom")
	fw := &fakeWTS{}
	fo := &fakeOfficial{cancelErr: sentinel}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	_, err := b.CancelPendingOrder(context.Background(), orderintent.CancelIntent{OrderID: "X"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want wrapped official error; got %v", err)
	}
	if fw.cancelCalls != 0 {
		t.Fatalf("no WTS retry after official cancel failure; got %d", fw.cancelCalls)
	}
}

// --- Amend routing ----------------------------------------------------------

func TestAmendOffNilRoutesToWTS(t *testing.T) {
	fw := &fakeWTS{}
	b := &hybridBroker{wts: fw, off: nil, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	if _, err := b.AmendPendingOrder(context.Background(), orderintent.AmendIntent{OrderID: "X"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.amendCalls != 1 {
		t.Fatalf("want WTS Amend called once; got %d", fw.amendCalls)
	}
}

func TestAmendAutoRoutesToOfficial(t *testing.T) {
	fw := &fakeWTS{}
	fo := &fakeOfficial{modifyRes: trading.MutationResult{Kind: "amend", CurrentOrderID: "A-2"}}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	res, err := b.AmendPendingOrder(context.Background(), orderintent.AmendIntent{OrderID: "X"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CurrentOrderID != "A-2" {
		t.Fatalf("want official amend result A-2; got %q", res.CurrentOrderID)
	}
	if fo.modifyCalls != 1 || fw.amendCalls != 0 {
		t.Fatalf("want official Modify once, WTS zero; got off=%d wts=%d", fo.modifyCalls, fw.amendCalls)
	}
}

func TestAmendOfficialErrorNoFallback(t *testing.T) {
	sentinel := errors.New("amend boom")
	fw := &fakeWTS{}
	fo := &fakeOfficial{modifyErr: sentinel}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	_, err := b.AmendPendingOrder(context.Background(), orderintent.AmendIntent{OrderID: "X"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want wrapped official error; got %v", err)
	}
	if fw.amendCalls != 0 {
		t.Fatalf("no WTS retry after official amend failure; got %d", fw.amendCalls)
	}
}

// --- GetOrderAvailableActions: always WTS ----------------------------------

func TestGetOrderAvailableActionsAlwaysWTS(t *testing.T) {
	fw := &fakeWTS{actions: map[string]any{"cancelable": true}}
	fo := &fakeOfficial{}
	b := &hybridBroker{wts: fw, off: fo, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	got, err := b.GetOrderAvailableActions(context.Background(), "X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.actionCalls != 1 {
		t.Fatalf("want WTS GetOrderAvailableActions called once even with off set; got %d", fw.actionCalls)
	}
	if v, _ := got["cancelable"].(bool); !v {
		t.Fatalf("want actions from WTS path; got %v", got)
	}
}

// --- officialEligiblePlace ---------------------------------------------------

func TestOfficialEligiblePlace(t *testing.T) {
	cases := []struct {
		name   string
		intent orderintent.PlaceIntent
		want   bool
	}{
		{"regular US limit USD", regularIntent(), true},
		{"KR limit KRW", orderintent.PlaceIntent{Market: "kr", OrderType: "limit", CurrencyMode: "KRW"}, true},
		{"US fractional USD", orderintent.PlaceIntent{Market: "us", OrderType: "market", Fractional: true, CurrencyMode: "USD"}, true},
		{"KRW fractional (WTS-unique)", krwFractionalIntent(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := officialEligiblePlace(tc.intent); got != tc.want {
				t.Fatalf("officialEligiblePlace=%v want %v", got, tc.want)
			}
		})
	}
}

// --- Broker() wiring: typed-nil guard ---------------------------------------

func TestBrokerNilOfficialBecomesNilInterface(t *testing.T) {
	c := New(client.New(client.Config{}), nil, Policy{Prefer: "auto"}, io.Discard)
	hb, ok := c.Broker().(*hybridBroker)
	if !ok {
		t.Fatal("Broker() must return *hybridBroker")
	}
	if hb.off != nil {
		t.Fatal("a nil *official.Client must become a nil officialOrderer, else off==nil routing breaks")
	}
}

func TestBrokerWiresOfficial(t *testing.T) {
	c := New(client.New(client.Config{}), &official.Client{}, Policy{Prefer: "auto"}, io.Discard)
	hb := c.Broker().(*hybridBroker)
	if hb.off == nil {
		t.Fatal("official client must be wired into the broker")
	}
}
