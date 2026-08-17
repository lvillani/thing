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

	"github.com/hashicorp/go-retryablehttp"

	"thing/internal/model"
)

// OpenAI is a provider for an OpenAI-compatible Chat Completions endpoint.
type OpenAI struct {
	client   *retryablehttp.Client
	token    string
	endpoint string
	timeout  time.Duration
}

// NewOpenAI creates a provider for the given endpoint, bearer token, and request
// timeout. The timeout applies to each model request.
func NewOpenAI(token, endpoint string, timeout time.Duration) *OpenAI {
	c := retryablehttp.NewClient()
	c.CheckRetry = openAICheckRetry
	c.Logger = nil

	return &OpenAI{
		client:   c,
		token:    token,
		endpoint: endpoint,
		timeout:  timeout,
	}
}

// Complete sends the conversation to the model and returns the assistant's reply.
func (o *OpenAI) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	requestCtx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	body, err := json.Marshal(chat)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(requestCtx, http.MethodPost, o.endpoint, bytes.NewReader(body))
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

// openAICheckRetry is a retry policy for OpenAI API requests. OpenRouter's API docs say
// that a model can generate no content while warming from a cold start or while the
// system is scaling up, and recommend a simple retry mechanism. A successful response
// with choices == 0 represents that no-content condition.
//
// See also:
// https://openrouter.ai/docs/api_reference/errors-and-debugging#when-no-content-is-generated
func openAICheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, policyErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if shouldRetry || policyErr != nil {
		return shouldRetry, policyErr
	}

	// Only a successful model response can be checked for choices. Error responses
	// usually do not contain a choices field, but they must keep the retry behavior
	// from DefaultRetryPolicy.
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, readErr := io.ReadAll(resp.Body)

	// The payload read (and its error) is the relevant result here. A close error is
	// not actionable for this retry decision, so it is intentionally ignored.
	_ = resp.Body.Close()

	// CheckRetry reads the response before Client.Do decides whether to retry. Restore
	// it so Client.Do can drain it on a retry, or Complete can decode it when this is
	// the final response.
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		return false, fmt.Errorf("could not read response: %w", readErr)
	}

	var result model.Response
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return false, fmt.Errorf("could not decode response: %w", err)
	}

	return len(result.Choices) == 0, nil
}
