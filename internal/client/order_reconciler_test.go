package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

func TestOrderReconcilerRetriesUntilStateIsVisible(t *testing.T) {
	r := orderReconciler{attempts: 3, interval: time.Millisecond, completedWindow: time.Minute}
	attempts := 0
	got, err := r.run(context.Background(), time.Now(), func(time.Time) (trading.MutationResult, bool, error) {
		attempts++
		return trading.MutationResult{Status: "accepted_pending"}, attempts == 2, nil
	}, func() trading.MutationResult {
		return trading.MutationResult{Status: "unknown"}
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got.Status != "accepted_pending" || attempts != 2 {
		t.Fatalf("run() = %#v after %d attempts, want accepted_pending after 2", got, attempts)
	}
}

func TestOrderReconcilerReturnsExplicitUnknownAfterBoundedAttempts(t *testing.T) {
	r := orderReconciler{attempts: 2, interval: time.Millisecond, completedWindow: time.Minute}
	unknown := trading.MutationResult{Kind: "place", Status: "unknown"}
	got, err := r.run(context.Background(), time.Now(), func(time.Time) (trading.MutationResult, bool, error) {
		return trading.MutationResult{}, false, nil
	}, func() trading.MutationResult { return unknown })
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got.Kind != unknown.Kind || got.Status != unknown.Status {
		t.Fatalf("run() = %#v, want %#v", got, unknown)
	}
}

func TestOrderReconcilerPropagatesProbeError(t *testing.T) {
	r := orderReconciler{attempts: 2, interval: time.Millisecond, completedWindow: time.Minute}
	want := errors.New("history unavailable")
	_, err := r.run(context.Background(), time.Now(), func(time.Time) (trading.MutationResult, bool, error) {
		return trading.MutationResult{}, false, want
	}, func() trading.MutationResult { return trading.MutationResult{Status: "unknown"} })
	if !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want %v", err, want)
	}
}
