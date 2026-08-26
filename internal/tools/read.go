// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"thing/internal/model"
)

// read is a tool that reads the contents of a file.
type read struct{}

// Describe returns the description of the read tool.
func (r *read) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "read",
			Description: "Reads the text content of a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file to read from",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// Run reads the content of the specified file path.
func (r *read) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %w", errToolBadArguments, err)
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errReadCannotReadFile, err)
	}

	return string(content), nil
}
