// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"charm.land/glamour/v2"
	"github.com/chzyer/readline"

	"thing/internal/model"
)

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_TOKEN not set")
		os.Exit(1)
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Minute}
	messages := []model.Message{{Role: model.MessageRoleDeveloper, Content: systemPrompt}}

	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		rl.Write(fmt.Appendf(nil, "─ ctx: %d in / %d out  cache: %s\n",
			stats.TotalPromptTokens,
			stats.TotalCompletionTokens,
			cacheSummary()))
		input, err := rl.Readline()
		if err != nil {
			break
		}

		messages = append(messages, model.Message{Role: model.MessageRoleUser, Content: input})

		// Keep talking to the model as long as it wants to use tools.
		for {
			msg, err := callAPI(ctx, client, token, messages)
			if err != nil {
				fmt.Fprintln(os.Stderr, "API error:", err)
				break
			}
			messages = append(messages, *msg)

			if len(msg.ToolCalls) == 0 {
				// Model produced a final answer.
				if msg.Content != "" {
					out, err := glamour.Render(msg.Content, "dark")
					if err != nil {
						fmt.Println(msg.Content)
					} else {
						fmt.Print(out)
					}
				}
				break
			}

			// Execute each tool call and feed results back.
			for _, tc := range msg.ToolCalls {
				result := executeTool(tc)
				messages = append(messages, model.Message{
					Role:       model.MessageRoleTool,
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
			// Loop back: send tool results to the model.
		}
	}
}
