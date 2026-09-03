package orderintent

import "testing"

func TestNormalizeOptionPlaceProducesPortableLiveIntent(t *testing.T) {
	t.Parallel()
	paper, err := NormalizeOptionPlace(OptionPlaceInput{
		Symbol: " opt_spy260904c01000000_20260731 ", Exchange: "amx",
		Side: "BUY", OrderType: "LIMIT", Price: 0.01, Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if paper.Symbol != "OPT_SPY260904C01000000_20260731" || paper.Exchange != "AMX" || paper.CurrencyMode != "USD" {
		t.Fatalf("paper = %#v", paper)
	}
	live, err := paper.LiveIntent()
	if err != nil {
		t.Fatal(err)
	}
	if live.Symbol != paper.Symbol || live.Market != "us" || live.Side != "buy" || live.OrderType != "limit" || live.Quantity != 1 || live.Price != 0.01 || live.CurrencyMode != "USD" {
		t.Fatalf("live = %#v", live)
	}
	if paper.PortableCanonical() != CanonicalPlace(live) {
		t.Fatalf("paper canonical %q != live canonical %q", paper.PortableCanonical(), CanonicalPlace(live))
	}
}

func TestNormalizeOptionPlaceRejectsNonOptionAndFractionalQuantity(t *testing.T) {
	t.Parallel()
	for _, input := range []OptionPlaceInput{
		{Symbol: "AAPL", Side: "buy", OrderType: "limit", Price: 1, Quantity: 1},
		{Symbol: "OPT_AAPL", Side: "buy", OrderType: "limit", Price: 1, Quantity: 0},
	} {
		if _, err := NormalizeOptionPlace(input); err == nil {
			t.Fatalf("expected error for %#v", input)
		}
	}
}
