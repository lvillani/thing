// SPDX-License-Identifier: GPL-3.0-only

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"thing/internal/model"
)

// OpenAI is a transport for an OpenAI-compatible Chat Completions endpoint.
type OpenAI struct {
	token    string
	endpoint string
	timeout  time.Duration
}

// NewOpenAI creates a transport for the given endpoint, bearer token, and request
// timeout. The timeout applies to each model request.
func NewOpenAI(token, endpoint string, timeout time.Duration) *OpenAI {
	return &OpenAI{token: token, endpoint: endpoint, timeout: timeout}
}

// Complete sends the conversation to the model and returns the assistant's reply.
func (o *OpenAI) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	requestCtx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	body, err := json.Marshal(chat)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, o.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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

	return &result, nil
}
