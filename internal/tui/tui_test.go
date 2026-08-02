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
	frame := stripANSI(m.View())
	if !strings.Contains(frame, "> ") || !strings.Contains(frame, "enter send") {
		t.Errorf("footer missing input or help: %q", frame)
	}
	if !strings.Contains(frame, "working") {
		t.Errorf("footer missing spinner while running: %q", frame)
	}
}

// mergeLines collapses a multi-line string to a single line (whitespace joined) so
// glamour line-wrapping/padding doesn't affect substring equality.
func mergeLines(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
