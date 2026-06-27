// Package tui provides a thin, TTY-isolated wrapper around charmbracelet/huh.
// Only this package may import huh; all other packages must remain huh-free.
package tui

import (
	"errors"
	"os"

	"github.com/charmbracelet/huh"
)

// ErrNotInteractive is returned by all wrappers when the terminal is not a
// character device (e.g. piped or redirected I/O).
var ErrNotInteractive = errors.New("not an interactive terminal")

// IsInteractive reports true only when both in and out are character devices.
// Any stat error is treated as non-interactive.
func IsInteractive(in, out *os.File) bool {
	fi, err := in.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	fo, err := out.Stat()
	if err != nil {
		return false
	}
	return fo.Mode()&os.ModeCharDevice != 0
}

// Select presents a single-choice list via huh.
// Returns ErrNotInteractive when os.Stdin/os.Stdout are not a TTY.
func Select(title string, opts []string) (string, error) {
	return selectWith(os.Stdin, os.Stdout, title, opts)
}

// Password presents a masked text input via huh.
// Returns ErrNotInteractive when os.Stdin/os.Stdout are not a TTY.
func Password(title string) (string, error) {
	return passwordWith(os.Stdin, os.Stdout, title)
}

// Confirm presents a yes/no prompt via huh.
// Returns ErrNotInteractive when os.Stdin/os.Stdout are not a TTY.
func Confirm(title string) (bool, error) {
	return confirmWith(os.Stdin, os.Stdout, title)
}

// --- in/out-injected internals (exported for test package) ---

func selectWith(in, out *os.File, title string, opts []string) (string, error) {
	if !IsInteractive(in, out) {
		return "", ErrNotInteractive
	}
	options := make([]huh.Option[string], len(opts))
	for i, o := range opts {
		options[i] = huh.NewOption(o, o)
	}
	var result string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(options...).
				Value(&result),
		),
	).Run()
	return result, err
}

func passwordWith(in, out *os.File, title string) (string, error) {
	if !IsInteractive(in, out) {
		return "", ErrNotInteractive
	}
	var result string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				EchoMode(huh.EchoModePassword).
				Value(&result),
		),
	).Run()
	return result, err
}

func confirmWith(in, out *os.File, title string) (bool, error) {
	if !IsInteractive(in, out) {
		return false, ErrNotInteractive
	}
	var result bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Value(&result),
		),
	).Run()
	return result, err
}
