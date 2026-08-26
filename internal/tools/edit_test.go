// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"thing/internal/model"
)

func TestEdit_ReplacesText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("before\nafter\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := (&edit{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":"`+path+`","oldText":"before","newText":"updated"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "edited: "+path {
		t.Errorf("out = %q, want %q", out, "edited: "+path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(content), "updated\nafter\n"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestEdit_ReplaceAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("same\nsame\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (&edit{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":"`+path+`","oldText":"same","newText":"new","replaceAll":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(content), "new\nnew\n"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestEdit_RejectsMissingText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (&edit{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":"`+path+`","oldText":"missing","newText":"new"}`))
	if !errors.Is(err, errEditTextNotFound) {
		t.Fatalf("error = %v, want errEditTextNotFound", err)
	}
}

func TestEdit_RejectsAmbiguousTextByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("same\nsame\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (&edit{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":"`+path+`","oldText":"same","newText":"new"}`))
	if !errors.Is(err, errEditTextFoundMultipleTimes) {
		t.Fatalf("error = %v, want errEditTextFoundMultipleTimes", err)
	}
}
