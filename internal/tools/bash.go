// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"thing/internal/model"
)

// bash is a tool that executes bash commands.
type bash struct{}

// Describe returns the description of the bash tool.
func (b *bash) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "bash",
			Description: "Execute a bash command",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The bash command to execute",
					},
					"timeout": map[string]any{
						"type":        "integer",
						"description": "Timeout in seconds",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// Run executes the bash command provided in the input.
func (b *bash) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	var args struct {
		Command string `json:"command"`
		Timeout int64  `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %w", errToolBadArguments, err)
	}

	cmdCtx := ctx
	if args.Timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, time.Duration(args.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(cmdCtx, "bash")
	cmd.Stdin = bytes.NewBufferString(args.Command)
	cmd.WaitDelay = 1 * time.Second

	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		// Feed the command's own output (stdout+stderr) back with the error so the
		// model can see what actually went wrong (e.g. rc != 0 producing stderr).
		if strings.TrimSpace(result) == "" {
			return "", fmt.Errorf("%w: %w", errBashCommandFailed, err)
		}
		return "", fmt.Errorf("%w: %w (rc=%v)\n%s", errBashCommandFailed, err, cmd.ProcessState.ExitCode(), result)
	}

	return result, err
}
