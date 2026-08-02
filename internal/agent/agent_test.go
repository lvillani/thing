// SPDX-License-Identifier: GPL-3.0-only

package agent

import (
	"context"
	"testing"

	"thing/internal/model"
)

// fakeModel returns a canned response so the agent can be driven with no network.
type fakeModel struct {
	responses []*model.Response
}

func (f *fakeModel) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func TestAgent_DrivesInjectedModelWithNoNetwork(t *testing.T) {
	// An independent source of truth for the expected reply and usage.
	f := &fakeModel{responses: []*model.Response{{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: model.Message{Role: model.MessageRoleAssistant, Content: "hello there"}}},
		Usage: &model.ResponseUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
	}}}

	a := NewAgent(f, "fake-model")
	a.SendMessage("hi")

	if got := a.Model.Complete; got == nil {
		t.Fatal("agent has no Model to call")
	}
	resp, err := a.Model.Complete(context.Background(), a.Chat)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	hasTools, err := a.ProcessResponse(resp)
	if err != nil {
		t.Fatalf("ProcessResponse returned error: %v", err)
	}
	if hasTools {
		t.Fatalf("ProcessResponse = %v, want false for a final answer", hasTools)
	}

	last := a.Chat.Messages[len(a.Chat.Messages)-1]
	if last.Role != model.MessageRoleAssistant || last.Content != "hello there" {
		t.Errorf("last message = %+v, want assistant 'hello there'", last)
	}
	if a.TotalPromptTokens != 10 || a.TotalCompletionTokens != 4 {
		t.Errorf("usage = in %d / out %d, want 10/4", a.TotalPromptTokens, a.TotalCompletionTokens)
	}
}
