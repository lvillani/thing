// SPDX-License-Identifier: GPL-3.0-only

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill creates root/<name>/SKILL.md and returns the file's path.
func writeSkill(t *testing.T, root, dir, content string) string {
	t.Helper()
	path := filepath.Join(root, dir, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNew_DiscoversFromAllRootsAndSorts(t *testing.T) {
	user := t.TempDir()
	proj := t.TempDir()

	writeSkill(t, user, "pdf", "---\nname: pdf\ndescription: extract text from PDFs\n---\nbody\n")
	writeSkill(t, proj, "git", "---\nname: git\ndescription: follow repo conventions\n---\nbody\n")
	// A non-skill dir (no SKILL.md) and .git must be ignored.
	if err := os.MkdirAll(filepath.Join(user, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(user, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}

	reg, err := New(user, proj)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cat := reg.Catalog()
	if len(cat) != 2 {
		t.Fatalf("catalog has %d skills, want 2: %+v", len(cat), cat)
	}
	if cat[0].Name != "git" || cat[1].Name != "pdf" {
		t.Errorf("catalog not sorted by name: %+v", cat)
	}
	for _, s := range cat {
		if s.Description == "" || s.Location == "" {
			t.Errorf("skill %q missing description or location: %+v", s.Name, s)
		}
	}
}

func TestNew_ProjectOverridesUser(t *testing.T) {
	user := t.TempDir()
	proj := t.TempDir()

	writeSkill(t, user, "code-review", "---\nname: code-review\ndescription: user-level review\n---\nbody\n")
	writeSkill(t, proj, "code-review", "---\nname: code-review\ndescription: project-level review\n---\nbody\n")

	reg, err := New(user, proj)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cat := reg.Catalog()
	if len(cat) != 1 {
		t.Fatalf("catalog has %d skills, want 1 (project shadowed user): %+v", len(cat), cat)
	}
	if cat[0].Description != "project-level review" {
		t.Errorf("description = %q, want the project-level one to win", cat[0].Description)
	}
}

func TestNew_LenientParsing(t *testing.T) {
	root := t.TempDir()

	// Name/directory mismatch warns but still loads.
	writeSkill(t, root, "mismatch-dir", "---\nname: actual-name\ndescription: a skill\n---\nbody\n")
	// Unquoted colon in the value must not break parsing.
	writeSkill(t, root, "pdf", "---\nname: pdf\ndescription: use when: the user shares a PDF\n---\nbody\n")
	// A horizontal rule (`---`) in the body must not truncate the frontmatter.
	writeSkill(t, root, "body-hr", "---\nname: body-hr\ndescription: has a rule below\n---\n\n---\n\nmarkdown body\n")
	// A lone apostrophe must not be mangled; a matching quote pair is stripped.
	writeSkill(t, root, "apostrophe", "---\nname: apostrophe\ndescription: can't handle that\n---\nbody\n")
	writeSkill(t, root, "quoted", "---\nname: quoted\ndescription: \"wrapped in quotes\"\n---\nbody\n")
	// A file beginning with `----` is not frontmatter.
	writeSkill(t, root, "dashes", "----\nnot frontmatter\n")
	// Description-less single-key frontmatter is skipped.
	writeSkill(t, root, "no-desc", "---\nname: no-desc\n---\nbody\n")
	// No frontmatter is skipped.
	writeSkill(t, root, "plain", "# Just a heading\n\nno frontmatter here\n")
	// Non-SKILL files in a dir are ignored (ensure it isn't picked up).
	dir := filepath.Join(root, "no-desc")

	if err := os.WriteFile(filepath.Join(dir, "ignore.me"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := New(root)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	got := map[string]Skill{}
	for _, s := range reg.Catalog() {
		got[s.Name] = s
	}

	if _, ok := got["actual-name"]; !ok {
		t.Error("mismatched name/dir skill was not loaded (should warn and load)")
	}
	if s, ok := got["pdf"]; !ok || s.Description != "use when: the user shares a PDF" {
		t.Errorf("lenient colon description not preserved: %+v", got["pdf"])
	}
	if s, ok := got["body-hr"]; !ok || !strings.Contains(s.Description, "rule below") {
		t.Errorf("body horizontal rule truncated the frontmatter: %+v", got["body-hr"])
	}
	if s, ok := got["apostrophe"]; !ok || s.Description != "can't handle that" {
		t.Errorf("apostrophe was mangled: %+v", got["apostrophe"])
	}
	if s, ok := got["quoted"]; !ok || s.Description != "wrapped in quotes" {
		t.Errorf("matching quote pair not stripped: %+v", got["quoted"])
	}
	if _, ok := got["dashes"]; ok {
		t.Error("file starting with ---- was treated as frontmatter")
	}
	if _, ok := got["no-desc"]; ok {
		t.Error("description-less skill was not skipped")
	}
	if _, ok := got["plain"]; ok {
		t.Error("frontmatter-less markdown was not skipped")
	}
}

func TestNew_MissingRootIsNotAnError(t *testing.T) {
	reg, err := New(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing root returned error: %v", err)
	}
	if len(reg.Catalog()) != 0 {
		t.Errorf("catalog = %+v, want empty", reg.Catalog())
	}
}
