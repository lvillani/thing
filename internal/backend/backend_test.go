// SPDX-License-Identifier: GPL-3.0-only

package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"thing/internal/model"
)

func TestOpenAIComplete_SendsRequestAndDecodes(t *testing.T) {
	var gotAuth, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, time.Minute)
	resp, err := o.Complete(context.Background(), model.Chat{Model: "m", Messages: []model.Message{{Role: model.MessageRoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if !strings.Contains(gotBody, `"model":"m"`) || !strings.Contains(gotBody, `"content":"hi"`) {
		t.Errorf("request body = %q, want marshalled chat", gotBody)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hello" {
		t.Errorf("response = %+v, want decoded assistant 'hello'", resp.Choices[0])
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 10 {
		t.Errorf("usage = %+v, want prompt_tokens 10", resp.Usage)
	}
}

func TestOpenAIComplete_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, time.Minute)
	_, err := o.Complete(context.Background(), model.Chat{})
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to include HTTP status", err.Error())
	}
}
