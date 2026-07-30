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

	"thing/internal/agent"
)

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_TOKEN not set")
		os.Exit(1)
	}

	ctx := context.Background()
	agent := agent.NewAgent(modelName)
	client := &http.Client{Timeout: 10 * time.Minute}

	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		rl.Write(fmt.Appendf(nil, "─ ctx: %d in / %d out  cache: %.1f%% (%d/%d)\n",
			agent.TotalPromptTokens,
			agent.TotalCompletionTokens,
			agent.TotalCachedTokensRatio*100,
			agent.TotalCachedTokens,
			agent.TotalPromptTokens,
		))
		input, err := rl.Readline()
		if err != nil {
			break
		}

		agent.SendMessage(input)

		for {
			response, err := callAPI(ctx, client, token, agent.Chat)
			if err != nil {
				fmt.Fprintln(os.Stderr, "API error:", err)
				break
			}

			hasToolCalls, err := agent.ProcessResponse(response)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				break
			}

			if hasToolCalls {
				continue
			}

			// Final answer
			out, err := glamour.Render(response.Choices[0].Message.Content, "dark")
			if err != nil {
				fmt.Println(response.Choices[0].Message.Content)
			} else {
				fmt.Print(out)
			}

			break
		}
	}
}
