// SPDX-License-Identifier: GPL-3.0-only

// Package backend is the transport boundary. It implements the agent core's Model
// interface for an OpenAI-compatible Chat Completions endpoint. The core never
// imports net/http or knows an endpoint; networking lives here.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"thing/internal/model"
)

// OpenAI is a transport for an OpenAI-compatible Chat Completions endpoint.
type OpenAI struct {
	token    string
	endpoint string
	client   *http.Client
}

// NewOpenAI creates a transport for the given endpoint and bearer token.
func NewOpenAI(token, endpoint string, client *http.Client) *OpenAI {
	return &OpenAI{token: token, endpoint: endpoint, client: client}
}

// Complete sends the conversation to the model and returns the assistant's reply.
func (o *OpenAI) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	body, err := json.Marshal(chat)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
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
