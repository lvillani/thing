// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_TOKEN not set")
		os.Exit(1)
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Minute}
	reader := bufio.NewReader(os.Stdin)
	messages := []Message{{Role: "system", Content: systemPrompt}}

	for {
		fmt.Printf("[%d] > ", len(messages))
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "read error:", err)
			return
		}

		messages = append(messages, Message{Role: "user", Content: input})

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
					fmt.Println(msg.Content)
				}
				break
			}

			// Execute each tool call and feed results back.
			for _, tc := range msg.ToolCalls {
				result := executeTool(tc)
				fmt.Print(result)
				messages = append(messages, Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
			// Loop back: send tool results to the model.
		}
	}
}
