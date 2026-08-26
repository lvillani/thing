// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"thing/internal/model"
)

func TestWrite_CreatesParentDirectoriesAndWritesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "directory", "file.txt")
	content := "written content\n"

	input, err := json.Marshal(map[string]string{"path": path, "content": content})
	if err != nil {
		t.Fatal(err)
	}

	got, err := (&write{}).Run(context.Background(), model.ToolCallFunctionArguments(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "wrote: " + path; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(file) != content {
		t.Errorf("content = %q, want %q", file, content)
	}
}

func TestWrite_OverwritesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	input, err := json.Marshal(map[string]string{"path": path, "content": "new content"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = (&write{}).Run(context.Background(), model.ToolCallFunctionArguments(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new content" {
		t.Errorf("content = %q, want %q", content, "new content")
	}
}

func TestWrite_InvalidArgumentsReturnsError(t *testing.T) {
	_, err := (&write{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":`))
	if err == nil {
		t.Fatal("expected an error for invalid arguments")
	}
	if !errors.Is(err, errToolBadArguments) {
		t.Errorf("error = %q, want errToolBadArguments", err)
	}
}
