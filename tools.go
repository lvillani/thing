// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// toolDefinitions returns the tool specs sent to the API.
// To add a new tool: add it here and add a case in executeTool.
func toolDefinitions() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "bash",
				Description: "Execute a bash command. The command is passed via stdin to bash.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The bash command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}
}

// executeTool dispatches a tool call and returns the result string.
func executeTool(tc ToolCall) string {
	switch tc.Function.Name {
	case "bash":
		return runBash(tc)
	default:
		return fmt.Sprintf("unknown tool: %s", tc.Function.Name)
	}
}

// --- tool implementations ---

func runBash(tc ToolCall) string {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("bad arguments: %v", err)
	}

	fmt.Printf("[bash] %s\n", args.Command)

	cmd := exec.Command("bash")
	cmd.Stdin = bytes.NewBufferString(args.Command)
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\n" + err.Error()
	}
	return result
}
