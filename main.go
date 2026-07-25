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
	"time"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const model = "xiaomi/mimo-v2.5-pro"

const systemPrompt = `
You are an expert assistant operating inside an agent harness.
`

type messages []message

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletion struct {
	Model    string   `json:"model"`
	Messages messages `json:"messages"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
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

		// Send request
		reqBody, err := json.Marshal(chatCompletion{
			Model:    model,
			Messages: messages,
		})
		if err != nil {
			fmt.Println("Error marshaling request body:", err)
			continue
		}
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
		if err != nil {
			fmt.Println("Error creating request:", err)
			continue
		}
		req.Header.Add("Authorization", "Bearer "+token)

		// Read response
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("Error sending request:", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Println("Error: received non-OK HTTP status:", resp.Status)
			resp.Body.Close()
			continue
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			resp.Body.Close()
			continue
		}
		var chat chatCompletionResponse
		err = json.Unmarshal(respBody, &chat)
		if err != nil {
			fmt.Println("Error unmarshaling response body:", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		// Print response
		if len(chat.Choices) > 0 {
			if len(chat.Choices) != 1 {
				fmt.Println("Warning: received multiple choices, using the first one.")
			}

			messages = append(messages, message{
				Role:    "assistant",
				Content: chat.Choices[0].Message.Content,
			})

			fmt.Println(chat.Choices[0].Message.Content)
		} else {
			fmt.Println("No response from the model.")
		}
	}
}
