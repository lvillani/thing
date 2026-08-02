// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// mentionMaxRows caps how tall the inline suggestion list can grow.
	mentionMaxRows = 10
)

var (
	mentionCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	mentionDirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
)

// mention holds the transient state for an active "@..." file-reference popup. It
// renders as a compact list just above the (still-visible, editable) input rather
// than a full-screen modal, so the user never loses sight of what they're typing.
type mention struct {
	open     bool
	dir      string
	all      []string // flattened relative paths under dir (files in subdirs included)
	selected int      // index into the currently-filtered matches
}

func newMention() mention { return mention{selected: 0} }

func (m *mention) close() {
	m.open = false
	m.all = nil
	m.selected = 0
}

// mentionQuery extracts the active @mention query from the input at the cursor:
// the text between the nearest "@" at-or-before the cursor and the cursor itself.
// Returns ok=false when no mention is active (no "@", or whitespace after it, which
// means the reference has been completed).
func mentionQuery(value string, pos int) (query string, ok bool) {
	idx := strings.LastIndexByte(value[:pos], '@')
	if idx < 0 {
		return "", false
	}
	q := value[idx+1 : pos]
	if strings.ContainsAny(q, " \t\n") {
		return "", false
	}
	return q, true
}

// listEntries returns a flattened, non-hidden listing under dir: every file and
// directory reached by a recursive walk, as a path relative to dir. This is what
// makes files in subdirectories appear in the popup, like other harnesses' flattened
// file list. Directories keep a trailing "/" and (because "/" < any letter) sort
// just before their own contents, so the tree reads naturally.
func listEntries(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == dir {
			return nil
		}
		// Skip hidden entries entirely; don't descend into hidden dirs.
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		if d.IsDir() {
			out = append(out, rel+"/")
		} else {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// refreshMention reconciles the popup with the current input. It opens (and lazily
// lists the directory) when a mention is active, and closes when it isn't.
func (m Model) refreshMention() Model {
	if _, ok := mentionQuery(m.input.Value(), m.input.Position()); !ok {
		if m.mention.open {
			m.mention.close()
		}
		return m
	}
	if !m.mention.open {
		m.mention.open = true
		m.mention.dir = pickerRoot()
		m.mention.all = listEntries(m.mention.dir)
	}
	m.mention.selected = 0
	return m
}

// mentionMatches returns the entries matching the current query, clamped by the
// empty-state rule (empty query matches everything). Matching is fuzzy: every query
// character must appear in the name in order, not necessarily contiguously (e.g.
// "agtcore" matches "internal/agt/core.go"). Results are ranked best-first.
func (m Model) mentionMatches() []string {
	if !m.mention.open {
		return nil
	}
	q, _ := mentionQuery(m.input.Value(), m.input.Position())
	q = strings.ToLower(q)
	if q == "" {
		return append([]string(nil), m.mention.all...)
	}

	type scored struct {
		name  string
		score int
	}
	var hits []scored
	for _, e := range m.mention.all {
		if score, ok := fuzzyScore(strings.ToLower(e), q); ok {
			hits = append(hits, scored{e, score})
		}
	}
	// Rank: better score first; ties keep the flattened-list (alphabetical) order.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })

	matches := make([]string, len(hits))
	for i, h := range hits {
		matches[i] = h.name
	}
	return matches
}

// fuzzyScore reports whether every query char appears in name in order (a
// subsequence match, e.g. "agtcore" matches "internal/agt/core.go") and returns a
// score where lower is better. An exact prefix match gets -1000 so it always tops
// the list; otherwise the score is the number of characters skipped in name between
// the first and last matched query char — tight matches rank above far-flung ones.
func fuzzyScore(name, query string) (int, bool) {
	qi := 0
	for ni := 0; ni < len(name) && qi < len(query); ni++ {
		if name[ni] == query[qi] {
			qi++
		}
	}
	if qi != len(query) {
		return 0, false
	}
	if strings.HasPrefix(name, query) {
		return -1000, true
	}
	first := strings.Index(name, query[:1])
	last := strings.LastIndex(name, query[len(query)-1:])
	span := last - first - (len(query) - 1)
	if span < 0 {
		span = 0
	}
	return span, true
}

// renderMention renders the inline suggestion list. An active selection is marked
// with ">"; the list scrolls to keep the selection visible and is capped in height.
func (m Model) renderMention() string {
	matches := m.mentionMatches()
	if len(matches) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Render("  no matches")
	}
	sel := m.mention.selected
	if sel >= len(matches) {
		sel = len(matches) - 1
	}

	start, end := 0, len(matches)
	if end > mentionMaxRows {
		start = sel - mentionMaxRows + 1
		end = sel + 1
		if start < 0 {
			start, end = 0, mentionMaxRows
		}
	}

	var rows []string
	for i := start; i < end; i++ {
		name := matches[i]
		if i == sel {
			rows = append(rows, mentionCursorStyle.Render("> "+name))
			continue
		}
		if strings.HasSuffix(name, "/") {
			rows = append(rows, mentionDirStyle.Render("  "+name))
		} else {
			rows = append(rows, "  "+name)
		}
	}
	return strings.Join(rows, "\n")
}

// commitMention replaces the current query (after the "@") with the selected entry
// and closes the popup, leaving the cursor right after the inserted name.
func (m Model) commitMention() Model {
	if !m.mention.open {
		return m
	}
	matches := m.mentionMatches()
	if len(matches) == 0 {
		return m.mentionClosed()
	}
	sel := m.mention.selected
	if sel >= len(matches) {
		sel = len(matches) - 1
	}
	name := matches[sel]

	v := m.input.Value()
	pos := m.input.Position()
	q, ok := mentionQuery(v, pos)
	if !ok {
		return m.mentionClosed()
	}
	at := pos - len(q) - 1 // index of the "@"
	next := v[:at+1] + name + " " + v[pos:]
	m.input.SetValue(next)
	m.input.SetCursor(at + 1 + len(name) + 1)
	return m.mentionClosed()
}

func (m Model) mentionClosed() Model {
	m.mention.close()
	return m
}

func (m Model) mentionCursorUp() Model {
	if m.mention.selected > 0 {
		m.mention.selected--
	}
	return m
}

func (m Model) mentionCursorDown() Model {
	if m.mention.selected < len(m.mentionMatches())-1 {
		m.mention.selected++
	}
	return m
}

// pickerRoot is the directory a mention references from; the agent's cwd.
func pickerRoot() string {
	if wd, err := filepath.Abs("."); err == nil {
		return wd
	}
	return "."
}
