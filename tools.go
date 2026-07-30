// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"thing/internal/model"
)

// executeTool dispatches a tool call and returns the result string.
func executeTool(tc model.ToolCall) string {
	switch tc.Function.Name {
	case "bash":
		return runBash(tc)
	default:
		return fmt.Sprintf("unknown tool: %s", tc.Function.Name)
	}
}

// --- tool implementations ---

func runBash(tc model.ToolCall) string {
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
