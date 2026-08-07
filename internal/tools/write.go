// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"thing/internal/model"
)

// write is a tool that writes text to a file.
type write struct{}

// Describe returns the description of the write tool.
func (w *write) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "write",
			Description: "Write text to a file, creating it if it doesn't exist, overwriting existing content. Automatically creates parent directories",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file to write to",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}
}

// Run writes the specified content to the given file path, creating parent directories if necessary.
func (w *write) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
		return "", fmt.Errorf("cannot create parent directories: %v", err)
	}

	err := os.WriteFile(args.Path, []byte(args.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("cannot write to file: %v", err)
	}

	return fmt.Sprintf("wrote: %s", args.Path), nil
}
