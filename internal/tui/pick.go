package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
)

// Item represents a single selectable entry: ID is the return value, Label is
// what the user sees in the huh Select menu.
type Item struct {
	ID    string
	Label string
}

// PickFromList presents an interactive single-choice list via huh and returns
// the ID of the selected Item. Returns an error for empty lists; returns
// ErrNotInteractive when Stdin/Stdout are not a TTY.
func PickFromList(title string, items []Item) (string, error) {
	return pickFromListWith(os.Stdin, os.Stdout, title, items)
}

func pickFromListWith(in, out *os.File, title string, items []Item) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("선택할 항목이 없습니다")
	}
	if !IsInteractive(in, out) {
		return "", ErrNotInteractive
	}
	opts := make([]huh.Option[string], 0, len(items))
	for _, it := range items {
		opts = append(opts, huh.NewOption(it.Label, it.ID))
	}
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Options(opts...).Value(&selected),
	))
	if err := form.Run(); err != nil {
		return "", err
	}
	return selected, nil
}
