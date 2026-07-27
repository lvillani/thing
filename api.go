// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const model = "deepseek/deepseek-v4-flash"

const systemPrompt = `
You are an expert assistant operating inside an agent harness.
`

// --- OpenAI-compatible API types ---

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ChatCompletion struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	PromptTokens          int                  `json:"prompt_tokens"`
	CompletionTokens      int                  `json:"completion_tokens"`
	TotalTokens           int                  `json:"total_tokens"`
	PromptTokensDetails   *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// Stats holds cumulative usage and cache metrics.
var stats struct {
	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalCachedTokens     int
}

// cacheSummary returns a string describing cached vs total prompt tokens.
func cacheSummary() string {
	if stats.TotalPromptTokens == 0 {
		return "—"
	}
	pct := float64(stats.TotalCachedTokens) / float64(stats.TotalPromptTokens) * 100
	return fmt.Sprintf("%.1f%% (%d/%d cached)",
		pct, stats.TotalCachedTokens, stats.TotalPromptTokens)
}

// callAPI sends the conversation to the model and returns the assistant's reply.
func callAPI(ctx context.Context, client *http.Client, token string, messages []Message) (*Message, error) {
	body, err := json.Marshal(ChatCompletion{
		Model:    model,
		Messages: messages,
		Tools:    toolDefinitions(),
	})
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

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}

	// Accumulate usage stats from the standardized response body.
	if result.Usage != nil {
		stats.TotalPromptTokens += result.Usage.PromptTokens
		stats.TotalCompletionTokens += result.Usage.CompletionTokens
		if d := result.Usage.PromptTokensDetails; d != nil {
			stats.TotalCachedTokens += d.CachedTokens
		}
	}

	return &result.Choices[0].Message, nil
}
