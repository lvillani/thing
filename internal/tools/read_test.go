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

func TestRead_ReturnsFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	want := "first line\nsecond line\n"
	if err := os.WriteFile(path, []byte(want), 0644); err != nil {
		t.Fatal(err)
	}

	input, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}

	got, err := (&read{}).Run(context.Background(), model.ToolCallFunctionArguments(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestRead_MissingFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")

	input, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}

	_, err = (&read{}).Run(context.Background(), model.ToolCallFunctionArguments(input))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !errors.Is(err, errReadCannotReadFile) {
		t.Errorf("error = %q, want errReadCannotReadFile", err)
	}
}

func TestRead_InvalidArgumentsReturnsError(t *testing.T) {
	_, err := (&read{}).Run(context.Background(), model.ToolCallFunctionArguments(`{"path":`))
	if err == nil {
		t.Fatal("expected an error for invalid arguments")
	}
	if !errors.Is(err, errToolBadArguments) {
		t.Errorf("error = %q, want errToolBadArguments", err)
	}
}
