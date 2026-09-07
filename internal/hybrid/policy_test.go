package hybrid

import (
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

func TestMutationPathKeepsWritesOnOneBackend(t *testing.T) {
	t.Parallel()

	regular := orderintent.PlaceIntent{Symbol: "AAPL", CurrencyMode: "USD"}
	fractionalKRW := regular
	fractionalKRW.Fractional = true
	fractionalKRW.CurrencyMode = "KRW"

	tests := []struct {
		name        string
		policy      Policy
		hasOfficial bool
		intent      *orderintent.PlaceIntent
		want        mutationPath
	}{
		{name: "missing official credential", policy: Policy{Prefer: routing.Auto}, intent: &regular, want: mutationWTS},
		{name: "wts preference", policy: Policy{Prefer: routing.WTS}, hasOfficial: true, intent: &regular, want: mutationWTS},
		{name: "wts-only fractional settlement", policy: Policy{Prefer: routing.Auto}, hasOfficial: true, intent: &fractionalKRW, want: mutationWTS},
		{name: "eligible official place", policy: Policy{Prefer: routing.Auto}, hasOfficial: true, intent: &regular, want: mutationOfficial},
		{name: "cancel follows configured backend", policy: Policy{Prefer: routing.Auto}, hasOfficial: true, want: mutationOfficial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.mutationPath(tt.hasOfficial, tt.intent); got != tt.want {
				t.Fatalf("mutationPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
