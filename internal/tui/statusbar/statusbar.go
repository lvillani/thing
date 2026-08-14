// SPDX-License-Identifier: GPL-3.0-only

// Package statusbar provides a status bar component.
package statusbar

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var style = lipgloss.NewStyle().Faint(true)

// Model represents the state of the status bar.
type Model struct {
	directory string
	model     string
}

// New creates a status bar that shows the current working directory and model name.
func New(modelName string) Model {
	directory, err := os.Getwd()
	if err != nil {
		directory = ""
	}

	return Model{
		directory: shortenHome(directory, homeDirectory()),
		model:     modelName,
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

// View implements the tea.Model interface. It renders the program's UI, which can be
// a string or a Layer. The view is rendered after every Update.
func (m Model) View() string {
	parts := []string{m.directory}
	if m.model != "" {
		parts = append(parts, m.model)
	}

	return style.Render(strings.Join(parts, " · "))
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
