// SPDX-License-Identifier: GPL-3.0-only

package tui

import "charm.land/lipgloss/v2"

// All style definitions in the TUI live here so presentation choices are easy to
// find and tweak in one place. Each style is a shared value-type; lipgloss.Style is
// immutable and safe to reuse across renders.

var (
	// Scrollback / chat-output styles.
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	youStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	agentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	toolStyle  = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	usageStyle = lipgloss.NewStyle().Faint(true)

	// File-mention popup styles.
	mentionCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	mentionDirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	mentionMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))

	// Skill popup styles.
	skillCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	skillDescStyle   = lipgloss.NewStyle().Faint(true)
	skillMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
)
