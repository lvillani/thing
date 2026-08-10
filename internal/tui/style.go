// SPDX-License-Identifier: GPL-3.0-only

package tui

import "charm.land/lipgloss/v2"

// maxWidth is the maximum width of the UI. It was taken from Python black's default
// line length. I usually use wide monitors and prose becomes too long too read with no
// wrapping.
const maxWidth = 88

// textareaPadding is the horizontal padding around the text area.
const textareaPadding = 1

var (
	primaryColor = lipgloss.Color("208")
	errorColor   = lipgloss.Color("196")
	userColor    = lipgloss.Color("39")
)

var (
	errorStyle = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
	toolStyle  = lipgloss.NewStyle().Faint(true)

	assistantMessageStyle = baseMessageStyle.BorderForeground(primaryColor)
	toolMessageStyle      = baseMessageStyle.BorderForeground(toolStyle.GetForeground())
	userMessageStyle      = baseMessageStyle.BorderForeground(userColor)

	baseMessageStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				MarginBottom(1)

	textareaStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(userColor).
			Padding(0, textareaPadding)
)
