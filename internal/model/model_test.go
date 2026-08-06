// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChatSerializesSessionID(t *testing.T) {
	chat := Chat{
		Model:     "m",
		SessionID: "abc-123",
		Messages:  []Message{{Role: MessageRoleUser, Content: "hi"}},
	}

	b, err := json.Marshal(chat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(b), `"session_id":"abc-123"`) {
		t.Errorf("marshal missed session_id: %s", b)
	}
}

func TestChatSessionIDOmittedWhenEmpty(t *testing.T) {
	chat := Chat{Model: "m", Messages: []Message{{Role: MessageRoleUser, Content: "hi"}}}

	b, err := json.Marshal(chat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(b), "session_id") {
		t.Errorf("empty session_id should be omitted, got: %s", b)
	}
}

func TestChatRoundTripsSessionID(t *testing.T) {
	in := Chat{Model: "m", SessionID: "abc-123", Messages: []Message{{Role: MessageRoleUser, Content: "hi"}}}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out Chat
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.SessionID != "abc-123" {
		t.Errorf("round trip session_id = %q, want abc-123", out.SessionID)
	}
}
