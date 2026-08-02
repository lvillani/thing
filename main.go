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
	"thing/internal/backend"
)

// const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const endpoint = "http://localhost:8080/v1/chat/completions"

// const modelName = "deepseek/deepseek-v4-flash"
const modelName = "gemma-4-26b-a4b-it"

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_TOKEN not set")
		os.Exit(1)
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Minute}
	a := agent.NewAgent(backend.NewOpenAI(token, endpoint, client), modelName)

	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		rl.Write(fmt.Appendf(nil, "─ ctx: %d in / %d out  cache: %.1f%% (%d/%d)\n",
			a.TotalPromptTokens,
			a.TotalCompletionTokens,
			a.TotalCachedTokensRatio*100,
			a.TotalCachedTokens,
			a.TotalPromptTokens,
		))
		input, err := rl.Readline()
		if err != nil {
			break
		}

		for ev := range a.Run(ctx, input) {
			switch ev.Kind {
			case agent.KindToolCall:
				fmt.Printf("  → %s\n", ev.Tool)
			case agent.KindToolResult:
				fmt.Printf("  %s\n", ev.Message)
			case agent.KindError:
				fmt.Fprintln(os.Stderr, "error:", ev.Message)
			case agent.KindFinal:
				out, err := glamour.Render(ev.Message, "dark")
				if err != nil {
					fmt.Println(ev.Message)
				} else {
					fmt.Print(out)
				}
			}
		}
	}
}
