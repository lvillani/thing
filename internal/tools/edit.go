// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"thing/internal/model"
)

// edit is a tool that replaces unique text in a file.
type edit struct{}

// Describe returns the description of the edit tool.
func (e *edit) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "edit",
			Description: "Replace a unique text occurrence in a text file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file to edit",
					},
					"oldText": map[string]any{
						"type":        "string",
						"description": "Exact text to replace",
					},
					"newText": map[string]any{
						"type":        "string",
						"description": "Replacement text",
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "Replace all occurrences instead of requiring exactly one",
					},
				},
				"required": []string{"path", "oldText", "newText"},
			},
		},
	}
}

// Run replaces one unique occurrence of oldText with newText in the file.
func (e *edit) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	var args struct {
		Path       string `json:"path"`
		OldText    string `json:"oldText"`
		NewText    string `json:"newText"`
		ReplaceAll bool   `json:"replaceAll"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %v", err)
	}

	text := string(content)
	count := strings.Count(text, args.OldText)
	if count == 0 {
		return "", fmt.Errorf("text not found in %s", args.Path)
	}
	if count > 1 && !args.ReplaceAll {
		return "", fmt.Errorf("text found %d times in %s; expected one occurrence", count, args.Path)
	}

	replacements := 1
	if args.ReplaceAll {
		replacements = -1
	}
	updated := strings.Replace(text, args.OldText, args.NewText, replacements)
	if err := os.WriteFile(args.Path, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("cannot write file: %v", err)
	}

	return fmt.Sprintf("edited: %s", args.Path), nil
}
