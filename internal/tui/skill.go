// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"sort"
	"strings"

	"thing/internal/skills"
)

const (
	// skillMaxRows caps how tall the inline skill popup can grow.
	skillMaxRows = mentionMaxRows
)

// skillPopup holds the transient state for an active "/skill:" autocomplete popup.
// It is a separate, parallel state machine from the file-mention popup (Option B of
// the ADR): skills and files have different data sources and different commit
// behaviors, so they are not folded into one generalized popup.
type skillPopup struct {
	open     bool
	all      []skills.Skill // catalog from the core; the single source of truth
	selected int            // index into the currently-filtered matches
}

func newSkillPopup() skillPopup { return skillPopup{selected: 0} }

func (p *skillPopup) close() {
	p.open = false
	p.all = nil
	p.selected = 0
}

// skillQuery extracts the active "/skill:" query from the input at the cursor: the
// text between the "/skill:" prefix and the cursor itself. Returns ok=false when no
// skill popup should be active (no prefix, or whitespace after the colon, which
// means a skill name has been completed and a task typed).
func skillQuery(value string, pos int) (query string, ok bool) {
	idx := strings.LastIndex(value[:pos], skillCommandPrefix)
	if idx < 0 {
		return "", false
	}
	q := value[idx+len(skillCommandPrefix) : pos]
	if strings.ContainsAny(q, " \t\n") {
		return "", false
	}
	return q, true
}

// refreshSkill reconciles the skill popup with the current input. It opens (and
// loads the catalog from the agent) when a "/skill:" query is active, and closes
// when it isn't. It is mutually exclusive with the file-mention popup: the two data
// sources are different, and a "/" cannot follow an unclosed "@" query.
func (m Model) refreshSkill() Model {
	if m.agent == nil {
		if m.skill.open {
			m.skill.close()
		}
		return m
	}
	if _, ok := skillQuery(m.input.Value(), m.input.Position()); !ok || m.mention.open {
		if m.skill.open {
			m.skill.close()
		}
		return m
	}
	if !m.skill.open {
		m.skill.open = true
		m.skill.all = m.agent.Skills()
	}
	m.skill.selected = 0
	return m
}

// skillMatches returns the catalog entries matching the current query, clamped by
// the empty-state rule (empty query matches everything). Matching is a plain
// case-insensitive subsequence like the file mention, ranked best-first.
func (m Model) skillMatches() []skills.Skill {
	if !m.skill.open {
		return nil
	}
	q, _ := skillQuery(m.input.Value(), m.input.Position())
	q = strings.ToLower(q)
	if q == "" {
		return append([]skills.Skill(nil), m.skill.all...)
	}

	type scored struct {
		skill skills.Skill
		score int
	}
	var hits []scored
	for _, s := range m.skill.all {
		if score, ok := fuzzyScore(strings.ToLower(s.Name), q); ok {
			hits = append(hits, scored{s, score})
		} else if score, ok := fuzzyScore(strings.ToLower(s.Description), q); ok {
			// Match against name first; a description-only hit ranks after any
			// name hit by a wide margin.
			hits = append(hits, scored{s, score + 10000})
		}
	}
	// Rank: better score first; ties keep the catalog (alphabetical) order.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })

	matches := make([]skills.Skill, len(hits))
	for i, h := range hits {
		matches[i] = h.skill
	}
	return matches
}

// renderSkill renders the inline "/skill:" autocomplete list. An active selection is
// marked with ">"; the list scrolls to keep the selection visible and is capped in
// height. Each row shows the skill's name and a dimmed one-line hint of its
// description.
func (m Model) renderSkill() string {
	matches := m.skillMatches()
	if len(matches) == 0 {
		return skillMutedStyle.Render("  no matching skills")
	}
	sel := m.skill.selected
	if sel >= len(matches) {
		sel = len(matches) - 1
	}

	start, end := 0, len(matches)
	if end > skillMaxRows {
		start = sel - skillMaxRows + 1
		end = sel + 1
		if start < 0 {
			start, end = 0, skillMaxRows
		}
	}

	var rows []string
	for i := start; i < end; i++ {
		name := matches[i].Name
		row := "  " + name
		if matches[i].Description != "" {
			row += " — " + skillDescStyle.Render(matches[i].Description)
		}
		if i == sel {
			rows = append(rows, skillCursorStyle.Render("> "+name+" — "+matches[i].Description))
			continue
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

// commitSkill replaces the "/skill:" query with the selected skill name and closes
// the popup, leaving the cursor right after the inserted name so the user can type a
// task. Only the name is committed (the description is not part of the command);
// this is the skill popup's distinct commit behavior vs. the file-mention popup.
func (m Model) commitSkill() Model {
	if !m.skill.open {
		return m
	}
	matches := m.skillMatches()
	if len(matches) == 0 {
		return m.skillClosed()
	}
	sel := m.skill.selected
	if sel >= len(matches) {
		sel = len(matches) - 1
	}
	name := matches[sel].Name

	v := m.input.Value()
	pos := m.input.Position()
	q, ok := skillQuery(v, pos)
	if !ok {
		return m.skillClosed()
	}
	at := pos - len(q) // index just after "/skill:"
	next := v[:at] + name + " " + v[pos:]
	m.input.SetValue(next)
	m.input.SetCursor(at + len(name) + 1)
	return m.skillClosed()
}

func (m Model) skillClosed() Model {
	m.skill.close()
	return m
}

func (m Model) skillCursorUp() Model {
	if m.skill.selected > 0 {
		m.skill.selected--
	}
	return m
}

func (m Model) skillCursorDown() Model {
	if m.skill.selected < len(m.skillMatches())-1 {
		m.skill.selected++
	}
	return m
}
