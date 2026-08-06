// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"regexp"
	"strings"
	"testing"

	"thing/internal/agent"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func newTestModel() *Model {
	return newModel(nil)
}

func TestNavigateHistory(t *testing.T) {
	m := newTestModel()
	m.storeHistory("first")
	m.storeHistory("second")
	if m.histIdx != -1 {
		t.Fatalf("histIdx after store = %d, want -1", m.histIdx)
	}

	m.navigateHistory("up")
	if m.histIdx != 1 || m.input.Value() != "second" {
		t.Errorf("first up: idx=%d value=%q, want idx=1 value=second", m.histIdx, m.input.Value())
	}
	m.navigateHistory("up")
	if m.histIdx != 0 || m.input.Value() != "first" {
		t.Errorf("second up: idx=%d value=%q, want idx=0 value=first", m.histIdx, m.input.Value())
	}
	// At the top, further up is a no-op.
	m.navigateHistory("up")
	if m.histIdx != 0 || m.input.Value() != "first" {
		t.Errorf("up past top: idx=%d value=%q, want idx=0 value=first", m.histIdx, m.input.Value())
	}

	m.navigateHistory("down")
	if m.histIdx != 1 || m.input.Value() != "second" {
		t.Errorf("down: idx=%d value=%q, want idx=1 value=second", m.histIdx, m.input.Value())
	}
	// Down past the newest entry returns to a fresh (empty) input.
	m.navigateHistory("down")
	if m.histIdx != -1 || m.input.Value() != "" {
		t.Errorf("down past newest: idx=%d value=%q, want idx=-1 empty", m.histIdx, m.input.Value())
	}

	// Down on a fresh input is a no-op.
	m.navigateHistory("down")
	if m.histIdx != -1 {
		t.Errorf("down on fresh: idx=%d, want -1", m.histIdx)
	}
}

func TestRenderEvent(t *testing.T) {
	cases := []struct {
		name string
		ev   agent.Event
		want []string
	}{
		{name: "tool call", ev: agent.Event{Kind: agent.KindToolCall, Tool: "bash"}, want: []string{"↳ bash"}},
		{name: "tool result", ev: agent.Event{Kind: agent.KindToolResult, Message: "ok"}, want: []string{"ok"}},
		{name: "error", ev: agent.Event{Kind: agent.KindError, Message: "boom"}, want: []string{"error: boom"}},
		{name: "final markdown", ev: agent.Event{Kind: agent.KindFinal, Message: "# Done\n\nsome **answer**"}, want: []string{"Agent", "Done", "some", "answer"}},
		{name: "unknown", ev: agent.Event{Kind: "bogus"}, want: nil},
		{name: "tool call with command", ev: agent.Event{Kind: agent.KindToolCall, Tool: "bash", ToolInput: `{"command":"ls -la"}`}, want: []string{"↳ bash {\"command\":\"ls -la\"}"}},
		{name: "tool result truncated", ev: agent.Event{Kind: agent.KindToolResult, Message: "l1\nl2\nl3\nl4\nl5"}, want: []string{"… (truncated)", "l3", "l4", "l5"}},
	}
	for _, c := range cases {
		got := mergeLines(stripANSI(strings.Join(renderEvent(c.ev), "\n")))
		want := mergeLines(strings.Join(c.want, "\n"))
		if got != want {
			t.Errorf("%s: renderEvent produced %q, want %q", c.name, got, want)
		}
	}
}

func TestRenderUser(t *testing.T) {
	got := stripANSI(strings.Join(renderUser("hello **world**"), "\n"))
	if !strings.Contains(got, "You") || !strings.Contains(got, "hello") {
		t.Errorf("renderUser missing label or body: %q", got)
	}
}

func TestViewIsFooterOnly(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.running = true
	frame := stripANSI(m.View().Content)
	if !strings.Contains(frame, "> ") || !strings.Contains(frame, "enter send") {
		t.Errorf("footer missing input or help: %q", frame)
	}
	if !strings.Contains(frame, "Working") {
		t.Errorf("footer missing spinner while running: %q", frame)
	}
}

// mergeLines collapses a multi-line string to a single line (whitespace joined) so
// glamour line-wrapping/padding doesn't affect substring equality.
func mergeLines(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestMentionQuery(t *testing.T) {
	cases := []struct {
		value string
		pos   int
		query string
		ok    bool
	}{
		{value: "", pos: 0, query: "", ok: false},
		{value: "hi ", pos: 3, query: "", ok: false},
		{value: "@", pos: 1, query: "", ok: true},
		{value: "read @no", pos: 8, query: "no", ok: true},
		{value: "read @doc", pos: 9, query: "doc", ok: true},
		{value: "read @doc done", pos: 9, query: "doc", ok: true}, // cursor before " done"
		{value: "read @do c", pos: 9, query: "", ok: false},       // whitespace after @
		{value: "read @a@b", pos: 9, query: "b", ok: true},        // nearest @
	}
	for _, c := range cases {
		q, ok := mentionQuery(c.value, c.pos)
		if ok != c.ok || q != c.query {
			t.Errorf("mentionQuery(%q, %d) = (%q, %v), want (%q, %v)",
				c.value, c.pos, q, ok, c.query, c.ok)
		}
	}
}

func TestMentionMatches(t *testing.T) {
	m := newTestModel()
	m.mention.open = true
	m.mention.dir = "."
	m.mention.all = []string{"docs/", "go.mod", "main.go"}
	m.input.SetValue("@go")
	m.input.SetCursor(3)

	matches := m.mentionMatches()
	// "go" is a subsequence of "go.mod" (prefix) and "main.go" too; prefix wins rank.
	if len(matches) != 2 || matches[0] != "go.mod" {
		t.Errorf("matches = %v, want [go.mod main.go] with go.mod first", matches)
	}
}

func TestMentionFuzzyRanking(t *testing.T) {
	m := newTestModel()
	m.mention.open = true
	m.mention.all = []string{
		"internal/agt/core.go",
		"internal/agent/core.go",
		"cmd/main.go",
		"README.md",
	}
	// Query "agtcore" is not a prefix of anything but is a subsequence of both
	// "internal/agt/core.go" and "internal/agent/core.go". The tighter span match
	// ("agt/core" vs "agent/core") should rank first.
	m.input.SetValue("@agtcore")
	m.input.SetCursor(8)

	matches := m.mentionMatches()
	wantFirst := "internal/agt/core.go"
	if len(matches) == 0 || matches[0] != wantFirst {
		t.Errorf("fuzzy top = %v, want %q first", matches, wantFirst)
	}
	for _, m := range matches {
		if !strings.Contains(m, "core.go") {
			t.Errorf("unexpected fuzzy match %q", m)
		}
	}
}

func TestFuzzyScore(t *testing.T) {
	cases := []struct {
		name, query string
		want        bool
	}{
		{"go.mod", "gm", true},     // subsequence
		{"go.mod", "gd", true},     // subsequence (g..d)
		{"go.mod", "xyz", false},   // not a subsequence
		{"go.mod", "go.mod", true}, // exact
		{"go.mod", "g", true},
	}
	for _, c := range cases {
		_, ok := fuzzyScore(c.name, c.query)
		if ok != c.want {
			t.Errorf("fuzzyScore(%q, %q) ok=%v, want %v", c.name, c.query, ok, c.want)
		}
	}
	// Prefix must always beat non-prefix.
	px, _ := fuzzyScore("go.mod", "go")
	np, _ := fuzzyScore("main.go", "go")
	if px >= np {
		t.Errorf("prefix score %d should be < non-prefix %d", px, np)
	}
}

func TestMentionCommitReplacesQuery(t *testing.T) {
	m := newTestModel()
	m.mention.open = true
	m.mention.dir = "."
	m.mention.all = []string{"docs/", "go.mod", "main.go"}
	m.input.SetValue("read @go")
	m.input.SetCursor(8) // after @go
	m.mention.selected = 0

	*m = m.commitMention()
	want := "read @go.mod "
	if m.input.Value() != want {
		t.Errorf("after commit: %q, want %q", m.input.Value(), want)
	}
	if m.mention.open {
		t.Errorf("expected mention popup to close")
	}
}
