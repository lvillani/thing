// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"thing/internal/model"
)

// const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const endpoint = "http://localhost:8080/v1/chat/completions"

// const modelName = "deepseek/deepseek-v4-flash"
const modelName = "gemma-4-26b-a4b-it"

// callAPI sends the conversation to the model and returns the assistant's reply.
func callAPI(ctx context.Context, client *http.Client, token string, chat model.Chat) (*model.Response, error) {
	body, err := json.Marshal(chat)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, respBody)
	}

	var result model.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}

	return &result, nil
}
