// SPDX-License-Identifier: GPL-3.0-only

package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"thing/internal/model"
)

func TestOpenAIComplete_SendsRequestAndDecodes(t *testing.T) {
	var gotAuth, gotContentType, gotSessionID, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotSessionID = r.Header.Get(sessionIDHeader)
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, srv.Client())
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
	if !isUUIDv4(gotSessionID) {
		t.Errorf("X-Session-Id = %q, want a UUIDv4", gotSessionID)
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

// TestOpenAIComplete_StableSessionIDAcrossRequests verifies that every request sent by
// one transport instance carries the same session identifier, so the provider can
// group them into a single session.
func TestOpenAIComplete_StableSessionIDAcrossRequests(t *testing.T) {
	var ids []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = append(ids, r.Header.Get(sessionIDHeader))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, srv.Client())
	for i := 0; i < 3; i++ {
		if _, err := o.Complete(context.Background(), model.Chat{}); err != nil {
			t.Fatalf("Complete #%d error: %v", i, err)
		}
	}
	if len(ids) != 3 {
		t.Fatalf("got %d requests, want 3", len(ids))
	}
	if ids[0] == "" || ids[0] != ids[1] || ids[1] != ids[2] {
		t.Errorf("session IDs not stable across requests: %v", ids)
	}
}

func TestOpenAIComplete_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, srv.Client())
	_, err := o.Complete(context.Background(), model.Chat{})
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to include HTTP status", err.Error())
	}
}

func TestOpenAIComplete_NoChoicesReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	o := NewOpenAI("secret", srv.URL, srv.Client())
	if _, err := o.Complete(context.Background(), model.Chat{}); err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

// uuidv4Re matches the canonical 8-4-4-4-12 UUIDv4 form.
var uuidv4Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func isUUIDv4(s string) bool { return uuidv4Re.MatchString(s) }
