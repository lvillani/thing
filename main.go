// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec" // NEW
	"time"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const model = "xiaomi/mimo-v2.5-pro"

const systemPrompt = `
You are an expert assistant operating inside an agent harness.
`

type messages []message

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type chatCompletion struct {
	Model    string   `json:"model"`
	Messages messages `json:"messages"`
	Tools    []tool   `json:"tools,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

var tools = []tool{
	{
		Type: "function",
		Function: toolFunction{
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

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Println("Error: OPENROUTER_API_TOKEN environment variable is not set.")
		return
	}

	ctx := context.Background()

	httpClient := &http.Client{Timeout: 600 * time.Second}

	messages := messages{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		// Read prompt
		fmt.Printf("[%d] > ", len(messages))
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		// Add user message
		messages = append(messages, message{
			Role:    "user",
			Content: input,
		})

		for {
			// Send request
			reqBody, err := json.Marshal(chatCompletion{
				Model:    model,
				Messages: messages,
				Tools:    tools, // NEW
			})
			if err != nil {
				fmt.Println("Error marshaling request body:", err)
				break
			}
			req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
			if err != nil {
				fmt.Println("Error creating request:", err)
				break
			}
			req.Header.Add("Authorization", "Bearer "+token)

			// Read response
			resp, err := httpClient.Do(req)
			if err != nil {
				fmt.Println("Error sending request:", err)
				break
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Println("Error: received non-OK HTTP status:", resp.Status)
				resp.Body.Close()
				break
			}
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error reading response body:", err)
				resp.Body.Close()
				break
			}
			var chat chatCompletionResponse
			err = json.Unmarshal(respBody, &chat)
			if err != nil {
				fmt.Println("Error unmarshaling response body:", err)
				resp.Body.Close()
				break
			}
			resp.Body.Close()

			if len(chat.Choices) == 0 {
				fmt.Println("No response from the model.")
				break
			}

			msg := chat.Choices[0].Message
			messages = append(messages, msg)

			// NEW: handle tool calls
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if tc.Function.Name == "bash" {
						var args struct {
							Command string `json:"command"`
						}
						json.Unmarshal([]byte(tc.Function.Arguments), &args)
						fmt.Printf("[bash] %s\n", args.Command)
						cmd := exec.Command("bash")
						cmd.Stdin = bytes.NewBufferString(args.Command)
						out, err := cmd.CombinedOutput()
						result := string(out)
						if err != nil {
							result += "\n" + err.Error()
						}
						fmt.Print(result)
						messages = append(messages, message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    result,
						})
					}
				}
				continue // loop back to send tool results to the model
			}

			if msg.Content != "" {
				fmt.Println(msg.Content)
			}
			break
		}
	}
}
