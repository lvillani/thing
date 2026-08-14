// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thing/internal/model"
)

func TestGlobReturnsFilesByModificationTime(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.go")
	new := filepath.Join(dir, "new.go")
	if err := os.WriteFile(old, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(new, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(new, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	out, err := (&glob{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"directory":"`+dir+`","pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := strings.Split(out, "\n"), []string{new, old}; !equalStrings(got, want) {
		t.Errorf("paths = %q, want %q", got, want)
	}
}

func TestGlobIgnoresGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\nignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored", "nested.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := (&glob{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"directory":"`+dir+`","pattern":"*.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "ignored") || !strings.Contains(out, "kept.txt") {
		t.Errorf("paths = %q, want only kept.txt", out)
	}
}

func TestGlobUsesDefaultLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(dir, string(rune('a'+i))+".txt"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := (&glob{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"directory":"`+dir+`","pattern":"*.txt","limit":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, "\n") + 1; got != 1 {
		t.Errorf("matches = %d, want 1", got)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
