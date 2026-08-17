// SPDX-License-Identifier: GPL-3.0-only

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"thing/internal/model"
)

func TestOpenAIComplete_SendsRequestAndDecodesResponse(t *testing.T) {
	var gotMethod string
	var gotAuthorization string
	var gotContentType string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuthorization = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`)
	}))
	defer server.Close()

	chat := model.Chat{
		Model:    "test-model",
		Messages: []model.Message{{Role: model.MessageRoleUser, Content: "hello"}},
	}
	provider := NewOpenAI("test-token", server.URL, time.Second)

	response, err := provider.Complete(context.Background(), chat)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotAuthorization != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuthorization, "Bearer test-token")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}

	var gotChat model.Chat
	if err := json.Unmarshal(gotBody, &gotChat); err != nil {
		t.Fatalf("request body is not valid chat JSON: %v", err)
	}
	if !reflect.DeepEqual(gotChat, chat) {
		t.Errorf("request chat = %#v, want %#v", gotChat, chat)
	}

	if len(response.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "hello" {
		t.Errorf("message content = %q, want %q", response.Choices[0].Message.Content, "hello")
	}
	if response.Usage == nil || response.Usage.PromptTokens != 10 || response.Usage.TotalTokens != 14 {
		t.Errorf("usage = %#v, want prompt=10 and total=14", response.Usage)
	}
}

func TestOpenAIComplete_RetriesEmptyChoicesAndReusesRequestBody(t *testing.T) {
	var attempts int
	var requestBodies [][]byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		requestBodies = append(requestBodies, body)

		switch attempts {
		case 1:
			_, _ = io.WriteString(w, `{"choices":[]}`)
		case 2:
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ready"}}]}`)
		default:
			http.Error(w, "unexpected extra request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	provider := NewOpenAI("test-token", server.URL, time.Second)
	provider.client.RetryWaitMin = 0
	provider.client.RetryWaitMax = 0
	provider.client.RetryMax = 1

	chat := model.Chat{
		Model:    "test-model",
		Messages: []model.Message{{Role: model.MessageRoleUser, Content: "retry me"}},
	}
	response, err := provider.Complete(context.Background(), chat)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("request attempts = %d, want 2", attempts)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("captured request bodies = %d, want 2", len(requestBodies))
	}
	if !bytes.Equal(requestBodies[0], requestBodies[1]) {
		t.Errorf("retry request body = %q, want first request body %q", requestBodies[1], requestBodies[0])
	}
	if len(response.Choices) != 1 || response.Choices[0].Message.Content != "ready" {
		t.Errorf("response = %#v, want one choice with content %q", response, "ready")
	}
}

func TestOpenAIComplete_ReturnsNonOKErrorWithoutRetry(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	provider := NewOpenAI("test-token", server.URL, time.Second)
	provider.client.RetryWaitMin = 0
	provider.client.RetryWaitMax = 0
	provider.client.RetryMax = 1

	_, err := provider.Complete(context.Background(), model.Chat{Model: "test-model"})
	if err == nil {
		t.Fatal("Complete returned nil error, want HTTP error")
	}
	if !strings.Contains(err.Error(), "API returned 400") || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error = %q, want status and response body", err)
	}
	if attempts != 1 {
		t.Errorf("request attempts = %d, want 1", attempts)
	}
}
