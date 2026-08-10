// SPDX-License-Identifier: GPL-3.0-only

// Package tui provides a terminal user interface for the application.
package tui

import (
	"thing/internal/agent"

	tea "charm.land/bubbletea/v2"
)

// Tui is the terminal user interface for the application.
type Tui struct {
	p *tea.Program
}

// NewTui creates a new Tui instance.
func NewTui(agent *agent.Agent) *Tui {
	m := initialModel(agent)

	// Here's a bit of a smelly catch-22: model needs tea.Program as dependency but
	// tea.Program needs model to be created. I'm not sure I like this kind of delayed
	// initialization, is there an alternative?
	p := tea.NewProgram(&m)
	m.p = p

	return &Tui{p: p}
}

// Run starts the TUI and blocks until it exits.
func (t *Tui) Run() error {
	_, err := t.p.Run()
	return err
}
