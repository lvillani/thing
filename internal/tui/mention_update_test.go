// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestUpdateOpensMentionOnAtKey ensures that typing '@' through Update opens the
// inline mention popup over the repo's top-level entries.
func TestUpdateOpensMentionOnAtKey(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("read ")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	next := m2.(Model)
	m = &next
	if !m.mention.open {
		t.Fatalf("typing @ did not open mention popup")
	}
	if len(m.mention.all) == 0 {
		t.Fatalf("mention popup listed no entries")
	}
	// The popup lists the real directory contents under the package cwd.
	want := listEntries(".")
	for i := range want {
		if i >= len(m.mention.all) || m.mention.all[i] != want[i] {
			t.Errorf("mention popup mismatch at %d: got %v, want %v", i, m.mention.all, want)
			break
		}
	}
}

// TestUpdateCommitMentionViaEnter ensures pressing Enter commits the selected entry.
func TestUpdateCommitMentionViaEnter(t *testing.T) {
	m := newTestModel()
	m.mention.open = true
	m.mention.dir = "."
	m.mention.all = []string{"docs/", "go.mod", "main.go"}
	m.input.SetValue("@g") // query 'g' → matches go.mod
	m.input.SetCursor(2)

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := m2.(Model)
	m = &next
	if cmd != nil {
		t.Fatalf("expected nil cmd after commit, got %v", cmd)
	}
	want := "@go.mod "
	if m.input.Value() != want {
		t.Errorf("after Enter: %q, want %q", m.input.Value(), want)
	}
	if m.mention.open {
		t.Errorf("mention should be closed after commit")
	}
}

// TestUpdateMentionEscCloses ensures Esc closes the popup and restores normal editing.
func TestUpdateMentionEscCloses(t *testing.T) {
	m := newTestModel()
	m.mention.open = true
	m.mention.all = []string{"go.mod"}
	m.input.SetValue("@")
	m.input.SetCursor(1)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := m2.(Model)
	m = &next
	if m.mention.open {
		t.Errorf("mention should be closed after esc")
	}
	if m.input.Value() != "@" {
		t.Errorf("esc should leave input untouched, got %q", m.input.Value())
	}
}
