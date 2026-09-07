package hybrid

import (
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

// readPolicy is the private policy seam for a hybrid read. Keeping the
// decision here means the operation adapters only describe their typed calls;
// they do not each reimplement credential and preference rules.
//
// The zero value is deliberately useful: it represents the default auto
// policy with no fallback. Client values made as struct literals in tests (or
// by older internal callers) therefore retain their existing behaviour.
type readPolicy struct {
	prefer   routing.Preference
	fallback bool
	stderr   io.Writer
}

func (p readPolicy) officialEnabled(off *official.Client) bool {
	return off != nil && p.prefer != routing.WTS
}

func (p readPolicy) fallbackEnabled(err error) bool {
	return p.fallback && official.ShouldFallback(err)
}

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
