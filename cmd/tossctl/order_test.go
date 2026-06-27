package main

import (
	"os"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestOrderItemsLabels(t *testing.T) {
	t.Parallel()

	items := orderItems([]domain.Order{
		{ID: "O1", Symbol: "005930", Name: "삼성전자", Side: "sell", Quantity: 1, Price: 1100, OrderDate: "2026-06-27"},
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %+v", len(items), items)
	}
	if items[0].ID != "O1" {
		t.Fatalf("expected ID O1, got %q", items[0].ID)
	}
	if !strings.Contains(items[0].Label, "삼성전자") {
		t.Fatalf("label missing Name: %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "1100") {
		t.Fatalf("label missing Price: %q", items[0].Label)
	}
}

func TestOrderItemsEmptyName(t *testing.T) {
	t.Parallel()

	// When Name is empty, Symbol should appear in the label.
	items := orderItems([]domain.Order{
		{ID: "O2", Symbol: "TSLA", Side: "buy", Quantity: 2, Price: 250.5, OrderDate: "2026-06-27"},
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !strings.Contains(items[0].Label, "TSLA") {
		t.Fatalf("label missing Symbol when Name empty: %q", items[0].Label)
	}
}

func TestOrderItemsLargePriceNotScientific(t *testing.T) {
	t.Parallel()

	// Large prices must render in plain decimal, never scientific notation.
	items := orderItems([]domain.Order{
		{ID: "O3", Symbol: "005930", Name: "삼성전자", Side: "buy", Quantity: 10, Price: 2700000, OrderDate: "2026-06-27"},
	})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if strings.Contains(items[0].Label, "e+") {
		t.Fatalf("label should not use scientific notation: %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "2700000") {
		t.Fatalf("label should contain plain price 2700000: %q", items[0].Label)
	}
}

func TestOrderItemsEmpty(t *testing.T) {
	t.Parallel()

	items := orderItems(nil)
	if len(items) != 0 {
		t.Fatalf("expected empty slice, got %+v", items)
	}
}

// TestOrderCancelNonTTYError checks that `order cancel` with no --order-id
// and a non-TTY stdin returns a clean error without blocking.
func TestOrderCancelNonTTYError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	cmd := newOrderCancelCmd(opts)
	gotErr := cmd.RunE(cmd, nil)
	if gotErr == nil {
		t.Fatal("expected error for missing --order-id in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "order-id") {
		t.Fatalf("error should mention order-id, got: %v", gotErr)
	}
}

// TestOrderAmendNonTTYError checks that `order amend` with no --order-id
// and a non-TTY stdin returns a clean error without blocking.
func TestOrderAmendNonTTYError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	cmd := newOrderAmendCmd(opts)
	gotErr := cmd.RunE(cmd, nil)
	if gotErr == nil {
		t.Fatal("expected error for missing --order-id in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "order-id") {
		t.Fatalf("error should mention order-id, got: %v", gotErr)
	}
}

// TestOrderShowNonTTYNoArgsError checks that `order show` with no argument
// and a non-TTY stdin returns a clean error without blocking.
func TestOrderShowNonTTYNoArgsError(t *testing.T) {
	// Not parallel: temporarily replaces os.Stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	opts := &rootOptions{}
	cmd := newOrderShowCmd(opts)
	gotErr := cmd.RunE(cmd, []string{}) // no args
	if gotErr == nil {
		t.Fatal("expected error for missing arg in non-TTY mode, got nil")
	}
	if !strings.Contains(gotErr.Error(), "order-id") {
		t.Fatalf("error should mention order-id, got: %v", gotErr)
	}
}
