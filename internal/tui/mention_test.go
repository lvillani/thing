// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListEntriesFlattensSubdirectories(t *testing.T) {
	dir := t.TempDir()
	must := func(e error) { if e != nil { t.Fatal(e) } }
	must(os.MkdirAll(filepath.Join(dir, "docs/sub/deep"), 0o755))
	must(os.MkdirAll(filepath.Join(dir, "internal/agt"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0o644))
	must(os.WriteFile(filepath.Join(dir, "docs/readme.md"), nil, 0o644))
	must(os.WriteFile(filepath.Join(dir, "docs/sub/a.txt"), nil, 0o644))
	must(os.WriteFile(filepath.Join(dir, "docs/sub/deep/d.md"), nil, 0o644))
	must(os.WriteFile(filepath.Join(dir, "internal/agt/core.go"), nil, 0o644))
	// hidden
	must(os.MkdirAll(filepath.Join(dir, ".hdir"), 0o755))
	must(os.WriteFile(filepath.Join(dir, ".hdir/f.txt"), nil, 0o644))
	must(os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o644))

	got := listEntries(dir)
	for _, g := range got {
		t.Logf("entry: %s", g)
	}
	want := []string{
		"docs/", "docs/readme.md", "docs/sub/", "docs/sub/a.txt",
		"docs/sub/deep/", "docs/sub/deep/d.md",
		"go.mod", "internal/", "internal/agt/", "internal/agt/core.go",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d:\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// hidden entries must never appear
	for _, g := range got {
		if containsDotPrefix(g) {
			t.Fatalf("hidden entry leaked: %q", g)
		}
	}
}

func containsDotPrefix(s string) bool {
	for _, part := range strings.Split(s, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
