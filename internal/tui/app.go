// Package tui implements the Bubble Tea interface.
package tui

import (
	"context"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yufeiminds/nameary/internal/check"
)

// Result is the aggregate returned after the program exits.
type Result struct {
	Counts   map[check.Status]int
	Total    int
	Duration time.Duration
}

// Run starts the TUI and blocks until the user quits.
func Run(ctx context.Context, opts Options) (Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m := newModel(ctx, opts)
	p := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	final, err := p.Run()
	m.closeRun()

	if fm, ok := final.(*model); ok {
		m = fm
	}
	dur := time.Since(m.started)
	if !m.finished.IsZero() {
		dur = m.finished.Sub(m.started)
	}
	return Result{Counts: m.counts, Total: len(m.rows), Duration: dur}, err
}
