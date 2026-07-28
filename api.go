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

const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const modelName = "deepseek/deepseek-v4-flash"

const systemPrompt = `
You are an expert assistant operating inside an agent harness.
`

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
func callAPI(ctx context.Context, client *http.Client, token string, messages []model.Message) (*model.Message, error) {
	body, err := json.Marshal(model.Chat{
		Model:    modelName,
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

	var result model.Response
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
