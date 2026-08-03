// SPDX-License-Identifier: GPL-3.0-only

// Package backend is the transport boundary. It implements the agent core's Model
// interface for an OpenAI-compatible Chat Completions endpoint. The core never
// imports net/http or knows an endpoint; networking lives here.
package backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"thing/internal/model"
)

// sessionIDHeader is the request header that associates a request with a logical
// session. Providers may use it to group the requests that belong to one
// conversation so they can track usage, latency, or caching together.
const sessionIDHeader = "X-Session-Id"

// OpenAI is a transport for an OpenAI-compatible Chat Completions endpoint.
// Each instance carries a session ID shared by every request it sends, so all
// calls in one session are attributable to the same logical conversation.
type OpenAI struct {
	token     string
	endpoint  string
	client    *http.Client
	sessionID string
}

// NewOpenAI creates a transport for the given endpoint and bearer token. A fresh
// session ID is generated so the requests this transport makes are grouped into a
// single session by providers that support session association.
func NewOpenAI(token, endpoint string, client *http.Client) *OpenAI {
	return &OpenAI{token: token, endpoint: endpoint, client: client, sessionID: newSessionID()}
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
	req.Header.Set(sessionIDHeader, o.sessionID)

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
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}

	return &result, nil
}

// newSessionID returns a random UUIDv4 used to identify a session. It needs no
// shared state across requests, just enough entropy that sessions cannot collide.
// The system CSPRNG is not expected to fail on supported platforms, so an error is
// unrecoverable — panic rather than silently hand out a colliding ID.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("backend: generate session id: %v", err))
	}
	// RFC 4122 version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
