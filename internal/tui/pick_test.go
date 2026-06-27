package tui

import (
	"errors"
	"os"
	"testing"
)

func TestPickFromListNonInteractive(t *testing.T) {
	r, _, _ := os.Pipe()
	_, err := pickFromListWith(r, os.Stdout, "주문 선택", []Item{{ID: "1", Label: "a"}})
	if !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("want ErrNotInteractive, got %v", err)
	}
}

func TestPickFromListEmpty(t *testing.T) {
	// interactive 여부와 무관하게 빈 목록은 에러
	_, err := PickFromList("x", nil)
	if err == nil {
		t.Fatal("empty list must error")
	}
}
