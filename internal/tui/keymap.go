// SPDX-License-Identifier: GPL-3.0-only

package tui

import "charm.land/bubbles/v2/key"

// keyMap defines the key bindings for the application.
type keyMap struct {
	Send    key.Binding
	Cancel  key.Binding
	History key.Binding
	Quit    key.Binding
}

// ShortHelp implements the help.KeyMap interface. Returns a slice of bindings to be
// displayed in the short version of the help. The help bubble will render help in the
// order in which the help items are returned here.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.Cancel, k.History, k.Quit}
}

// FullHelp implements the help.KeyMap interface. Returns an extended group of help
// items, grouped by columns. The help bubble will render the help in the order in which
// the help items are returned here.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// defaultKeyMap returns the default key bindings for the application.
func defaultKeyMap() keyMap {
	return keyMap{
		Send:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		History: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "history")),
		Quit:    key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "quit")),
	}
}
