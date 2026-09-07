package hybrid

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

// mutationPath is intentionally smaller than the public Policy. A mutation
// must choose one starting adapter; unlike reads it never has a fallback
// transition after the request has been sent.
type mutationPath uint8

const (
	mutationWTS mutationPath = iota
	mutationOfficial
)

func (p Policy) mutationPath(hasOfficial bool, place *orderintent.PlaceIntent) mutationPath {
	if !hasOfficial || p.Prefer == routing.WTS {
		return mutationWTS
	}
	if place != nil && !officialEligiblePlace(*place) {
		return mutationWTS
	}
	return mutationOfficial
}
