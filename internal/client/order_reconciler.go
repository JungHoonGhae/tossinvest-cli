package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// orderReconciler owns the post-mutation state transition.  The transport
// methods only submit a request; this module is the single place that polls
// pending/completed history and turns an invisible result into an explicit
// unknown state.  Keeping the polling policy here gives place, cancel, and
// amend the same retry and timing semantics without widening their interface.
type orderReconciler struct {
	client          *Client
	attempts        int
	interval        time.Duration
	completedWindow time.Duration
}

func newOrderReconciler(client *Client) orderReconciler {
	return orderReconciler{
		client:          client,
		attempts:        mutationReconcileAttempts,
		interval:        mutationReconcileInterval,
		completedWindow: mutationCompletedLookback,
	}
}

// run executes one bounded reconciliation loop. probe returns (result, true,
// nil) when the expected state is visible, (zero, false, nil) while it is
// still settling, and an error when the read itself failed.
func (r orderReconciler) run(
	ctx context.Context,
	startedAt time.Time,
	probe func(earliestCompleted time.Time) (trading.MutationResult, bool, error),
	unknown func() trading.MutationResult,
) (trading.MutationResult, error) {
	earliest := startedAt.Add(-r.completedWindow)
	for attempt := 0; attempt < r.attempts; attempt++ {
		result, found, err := probe(earliest)
		if err != nil {
			return trading.MutationResult{}, err
		}
		if found {
			return result, nil
		}
		if attempt == r.attempts-1 {
			return unknown(), nil
		}
		timer := time.NewTimer(r.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return trading.MutationResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return unknown(), nil
}
