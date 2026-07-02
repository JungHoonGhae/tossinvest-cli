package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// WithSpinner runs fn while showing an animated spinner next to label, when
// stdin/stdout are a TTY. In non-interactive contexts (piped I/O, CI, agent
// loops) it prints "label..." once, runs fn synchronously, and prints
// "done"/"failed" on completion — no ANSI or animation, safe for logs.
func WithSpinner(label string, fn func() error) error {
	return withSpinnerWith(os.Stdin, os.Stdout, label, fn)
}

func withSpinnerWith(in, out *os.File, label string, fn func() error) error {
	if !IsInteractive(in, out) {
		fmt.Fprintf(out, "%s...\n", label)
		err := fn()
		if err != nil {
			fmt.Fprintln(out, "failed")
			return err
		}
		fmt.Fprintln(out, "done")
		return nil
	}

	m := newSpinnerModel(label, fn)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	fm, ok := finalModel.(*spinnerModel)
	if !ok {
		return nil
	}
	return fm.err
}

type spinnerModel struct {
	label string
	spin  spinner.Model
	fn    func() error
	err   error
	done  bool
}

func newSpinnerModel(label string, fn func() error) *spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &spinnerModel{label: label, spin: s, fn: fn}
}

func (m *spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, runFn(m.fn))
}

func (m *spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fnDoneMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *spinnerModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s %s\n", m.spin.View(), m.label)
}

type fnDoneMsg struct{ err error }

// runFn wraps fn as a tea.Cmd. Bubble Tea runs every returned tea.Cmd in its
// own goroutine, so this is how the spinner keeps animating while fn (e.g. a
// network call) runs in the background.
func runFn(fn func() error) tea.Cmd {
	return func() tea.Msg {
		return fnDoneMsg{err: fn()}
	}
}
