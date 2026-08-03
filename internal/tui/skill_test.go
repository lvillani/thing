// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thing/internal/agent"
	"thing/internal/model"
	"thing/internal/skills"
)

func TestParseSkillCommand(t *testing.T) {
	cases := []struct {
		in         string
		name, task string
		isCmd      bool
	}{
		{in: "/skill:git", name: "git", isCmd: true},
		{in: "/skill:git make a commit", name: "git", task: "make a commit", isCmd: true},
		{in: "/skill:my-skill  with spaces", name: "my-skill", task: "with spaces", isCmd: true},
		{in: "/skill:", name: "", isCmd: true},
		{in: "git", isCmd: false},
		{in: "/other:git", isCmd: false},
		{in: "", isCmd: false},
	}
	for _, c := range cases {
		name, task, ok := parseSkillCommand(c.in)
		if ok != c.isCmd || name != c.name || task != c.task {
			t.Errorf("parseSkillCommand(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, name, task, ok, c.name, c.task, c.isCmd)
		}
	}
}

func TestSkillQuery(t *testing.T) {
	cases := []struct {
		value string
		pos   int
		query string
		ok    bool
	}{
		{value: "", pos: 0, query: "", ok: false},
		{value: "/skill:", pos: 7, query: "", ok: true},
		{value: "/skill:git", pos: 10, query: "git", ok: true},
		{value: "/skill:git ", pos: 11, query: "", ok: false},         // trailing space means task mode
		{value: "/skill:git commit", pos: 10, query: "git", ok: true}, // cursor before task
		{value: "hello /skill:", pos: 13, query: "", ok: true},
	}
	for _, c := range cases {
		q, ok := skillQuery(c.value, c.pos)
		if ok != c.ok || q != c.query {
			t.Errorf("skillQuery(%q, %d) = (%q, %v), want (%q, %v)",
				c.value, c.pos, q, ok, c.query, c.ok)
		}
	}
}

// stubModel satisfies the agent.Model seam so agent.NewAgent can build an agent that
// is never actually run in these tests.
type stubModel struct{}

func (stubModel) Complete(context.Context, model.Chat) (*model.Response, error) {
	return nil, nil
}

// newSkillModel builds a TUI model wired to a real agent whose registry contains the
// given "name:description" skills. It exercises the real core accessor rather than a
// stub so the popup's single-source-of-truth is under test.
func newSkillModel(t *testing.T, specs []string) *Model {
	t.Helper()
	root := t.TempDir()
	for _, spec := range specs {
		name, desc, _ := strings.Cut(spec, ":")
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := skills.New(root)
	if err != nil {
		t.Fatalf("skills.New: %v", err)
	}
	m := newModel(agent.NewAgent(stubModel{}, "fake-model", reg))
	m.input.CharLimit = 0
	return m
}

// TestRefreshSkillOpensOnPrefix ensures typing "/skill:git" opens the popup over the
// real catalog from the core.
func TestRefreshSkillOpensOnPrefix(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before committing"})
	m.input.SetValue("/skill:git")
	m.input.SetCursor(10)

	*m = m.refreshSkill()
	if !m.skill.open {
		t.Fatalf("typing /skill: did not open skill popup")
	}
	if len(m.skill.all) != 1 || m.skill.all[0].Name != "git-commit" {
		t.Fatalf("skill popup loaded wrong catalog: %+v", m.skill.all)
	}
}

func TestRefreshSkillClosesOnSpace(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before committing"})
	m.skill.open = true
	m.skill.all = []skills.Skill{{Name: "git-commit", Description: "commit"}}
	m.input.SetValue("/skill:git ")
	m.input.SetCursor(11)

	*m = m.refreshSkill()
	if m.skill.open {
		t.Fatalf("skill popup should close once a task is typed")
	}
}

func TestRefreshSkillClosesOnNoAgent(t *testing.T) {
	m := newTestModel() // agent is nil
	m.skill.open = true
	m.skill.all = []skills.Skill{{Name: "git", Description: "x"}}
	m.input.SetValue("/skill:git")
	m.input.SetCursor(10)

	*m = m.refreshSkill()
	if m.skill.open {
		t.Fatalf("skill popup should close with no agent")
	}
}

func TestSkillMatches(t *testing.T) {
	m := newSkillModel(t, []string{
		"git-commit:read before committing",
		"tdd:write tests first",
		"teach:teach a concept",
	})
	m.skill.open = true
	m.skill.all = m.agent.Skills()
	m.input.SetValue("/skill:te")
	m.input.SetCursor(9)

	matches := m.skillMatches()
	if len(matches) != 2 || matches[0].Name != "teach" {
		t.Errorf("matches = %v, want [teach tdd] with teach first", names(matches))
	}
}

func TestSkillMatchByDescription(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before making a commit"})
	m.skill.open = true
	m.skill.all = m.agent.Skills()
	m.input.SetValue("/skill:commit")
	m.input.SetCursor(13)

	if matches := m.skillMatches(); len(matches) != 1 || matches[0].Name != "git-commit" {
		t.Errorf("description match failed: %v", names(matches))
	}
}

func TestSkillCommitReplacesPrefix(t *testing.T) {
	m := newSkillModel(t, []string{
		"git-commit:read before committing",
		"tdd:write tests first",
	})
	m.skill.open = true
	m.skill.all = m.agent.Skills()
	m.input.SetValue("/skill:git")
	m.input.SetCursor(10)
	m.skill.selected = 0

	*m = m.commitSkill()
	want := "/skill:git-commit "
	if m.input.Value() != want {
		t.Errorf("after commit: %q, want %q", m.input.Value(), want)
	}
	if m.skill.open {
		t.Errorf("skill popup should close after commit")
	}
}

func names(s []skills.Skill) []string {
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[i].Name
	}
	return out
}

func TestResolveInputActivatesSkill(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before committing"})
	out, err := m.resolveInput("/skill:git-commit make a commit")
	if err != nil {
		t.Fatalf("resolveInput: %v", err)
	}
	if !strings.Contains(out, "git-commit") || !strings.Contains(out, "make a commit") {
		t.Errorf("resolved pointer incomplete: %q", out)
	}
}

func TestResolveInputNonCommandPassesThrough(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before committing"})
	out, err := m.resolveInput("just a normal prompt")
	if err != nil || out != "just a normal prompt" {
		t.Errorf("resolveInput(%q) = %q, %v; want passthrough", "just a normal prompt", out, err)
	}
}

func TestResolveInputUnknownSkillErrors(t *testing.T) {
	m := newSkillModel(t, []string{"git-commit:read before committing"})
	_, err := m.resolveInput("/skill:does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should name skill: %v", err)
	}
}
