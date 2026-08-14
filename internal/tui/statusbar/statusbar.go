// SPDX-License-Identifier: GPL-3.0-only

// Package statusbar provides a status bar component.
package statusbar

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var style = lipgloss.NewStyle().Faint(true)

// Model represents the state of the status bar.
type Model struct {
	directory       string
	model           string
	messageCount    int
	usage           Usage
	contextWindow   int64
	contextProgress progress.Model
}

// Usage contains the token statistics for the latest request.
type Usage struct {
	PromptTokens      int
	CompletionTokens  int
	CachedTokens      int
	CachedTokensRatio float64
}

// New creates a status bar that shows the current working directory and model name.
func New(modelName string, contextWindow int64) Model {
	directory, err := os.Getwd()
	if err != nil {
		directory = ""
	}

	return Model{
		directory:     shortenHome(directory, homeDirectory()),
		model:         modelName,
		contextWindow: contextWindow,
		contextProgress: progress.New(
			progress.WithColors(lipgloss.BrightBlack, lipgloss.BrightBlack),
			progress.WithoutPercentage(),
			progress.WithWidth(20),
		),
	}
}

// Init implements the tea.Model interface. It is the first function that will be
// called. It returns an optional initial command. To not perform an initial command
// return nil.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements the tea.Model interface. It is called when a message is received.
// Use it to inspect messages and, in response, update the model and/or send a command.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// SetStats updates the chat and request statistics shown by the statusbar.
func (m *Model) SetStats(messageCount int, usage Usage) {
	m.messageCount = messageCount
	m.usage = usage
}

// View implements the tea.Model interface. It renders the program's UI, which can be
// a string or a Layer. The view is rendered after every Update.
func (m Model) View() string {
	parts := []string{m.directory}
	if m.model != "" {
		parts = append(parts, m.model)
	}

	return style.Render(strings.Join(parts, " · ")) + "\n" + m.usageSummary()
}

// usageSummary formats the current conversation and request statistics.
func (m Model) usageSummary() string {
	contextUsage := "context unknown"
	if m.contextWindow > 0 {
		percent := float64(m.usage.PromptTokens) / float64(m.contextWindow)
		contextUsage = m.contextProgress.ViewAs(percent) + style.Render(fmt.Sprintf(" %s/%s", formatTokens(int64(m.usage.PromptTokens)), formatTokens(m.contextWindow)))
	}
	text := fmt.Sprintf(
		" · %d messages · %.1f%% cache hit",
		m.messageCount,
		m.usage.CachedTokensRatio*100,
	)
	return contextUsage + style.Render(text)
}

func formatTokens(tokens int64) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.0fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// shortenHome replaces the home directory prefix with ~.
func shortenHome(directory, home string) string {
	if directory == "" {
		return ""
	}

	directory = filepath.Clean(directory)
	home = filepath.Clean(home)
	if home == "." {
		return directory
	}

	relative, err := filepath.Rel(home, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return directory
	}
	if relative == "." {
		return "~"
	}

	return "~" + string(filepath.Separator) + relative
}

// homeDirectory returns the current user's home directory. It returns an empty string
// when the home directory cannot be determined.
func homeDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}
