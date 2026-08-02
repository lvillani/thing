// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"thing/internal/model"
)

type bash struct{}

func (b *bash) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "bash",
			Description: "Execute a bash command.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The bash command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

func (b *bash) Run(input string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}

	cmd := exec.Command("bash")
	cmd.Stdin = bytes.NewBufferString(args.Command)

	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		return "", err
	}

	return result, err
}
