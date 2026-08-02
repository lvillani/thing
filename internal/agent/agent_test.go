// SPDX-License-Identifier: GPL-3.0-only

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thing/internal/model"
	"thing/internal/skills"
)

// fakeModel returns a canned response per call so the agent can be driven with no
// network. It errors when its script is exhausted, which also surfaces any
// unexpected extra calls.
type fakeModel struct {
	responses []*model.Response
}

func (f *fakeModel) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	if len(f.responses) == 0 {
		return nil, errors.New("fakeModel: unexpected extra call")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

// errModel always fails, to exercise error propagation and cancellation.
type errModel struct{ err error }

func (e *errModel) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	if e.err != nil {
		return nil, e.err
	}
	return nil, ctx.Err()
}

// blockingModel blocks inside Complete until ctx is done, like a long inference.
type blockingModel struct{}

func (blockingModel) Complete(ctx context.Context, chat model.Chat) (*model.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// fakeTool is a deterministic tool used to exercise tool routing in the loop.
type fakeTool struct{ out string }

func (t *fakeTool) Describe() model.Tool {
	return model.Tool{Type: model.ToolTypeFunction, Function: model.ToolFunctionDefinition{Name: "echo"}}
}

func (t *fakeTool) Run(input string) (string, error) {
	return t.out, nil
}

// collect drains events with a timeout so a leaked producer cannot hang the test.
func collect(t *testing.T, ch <-chan Event) []Event {
	t.Helper()
	var evs []Event
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return evs
			}
			evs = append(evs, ev)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for events; got %+v", evs)
		}
	}
}

func finalResponse(msg model.Message) *model.Response {
	return &model.Response{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: msg}},
	}
}

func TestRun_StraightFinal(t *testing.T) {
	// Independent source of truth for the reply and usage.
	reply := model.Message{Role: model.MessageRoleAssistant, Content: "hello there"}
	f := &fakeModel{responses: []*model.Response{{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: reply}},
		Usage: &model.ResponseUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14,
			PromptTokensDetails: &model.ResponseUsageDetails{CachedTokens: 3}},
	}}}

	a := NewAgent(f, "fake-model")
	evs := collect(t, a.Run(context.Background(), "hi"))

	if len(evs) != 1 || evs[0].Kind != KindFinal || evs[0].Message != "hello there" {
		t.Fatalf("events = %+v, want a single final 'hello there'", evs)
	}
	if evs[0].PromptTokens != 10 || evs[0].CompletionTokens != 4 || evs[0].CachedTokens != 3 {
		t.Errorf("final event usage = in %d/out %d/cached %d, want 10/4/3",
			evs[0].PromptTokens, evs[0].CompletionTokens, evs[0].CachedTokens)
	}

	// Conversation ends with the assistant reply; usage accumulated in the core.
	last := a.Chat.Messages[len(a.Chat.Messages)-1]
	if last.Role != model.MessageRoleAssistant || last.Content != "hello there" {
		t.Errorf("last message = %+v, want assistant 'hello there'", last)
	}
	if a.TotalPromptTokens != 10 || a.TotalCompletionTokens != 4 || a.TotalCachedTokens != 3 {
		t.Errorf("usage = in %d/out %d/cached %d, want 10/4/3",
			a.TotalPromptTokens, a.TotalCompletionTokens, a.TotalCachedTokens)
	}
}

func TestRun_ToolRoundThenFinal(t *testing.T) {
	toolResp := model.Message{Role: model.MessageRoleAssistant, Content: "", ToolCalls: []model.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "echo", Arguments: `{"text":"x"}`},
	}}}
	f := &fakeModel{responses: []*model.Response{
		finalResponse(toolResp),
		finalResponse(model.Message{Role: model.MessageRoleAssistant, Content: "done"}),
	}}

	a := NewAgent(f, "fake-model")
	a.Tools.Register(&fakeTool{out: "echoed"})

	evs := collect(t, a.Run(context.Background(), "please echo"))

	want := []EventKind{KindToolCall, KindToolResult, KindFinal}
	if len(evs) != len(want) {
		t.Fatalf("events = %+v, want %v", evs, want)
	}
	for i, k := range want {
		if evs[i].Kind != k {
			t.Errorf("event[%d] = %s, want %s (%+v)", i, evs[i].Kind, k, evs[i])
		}
	}
	if evs[0].Tool != "echo" || evs[1].Tool != "echo" {
		t.Errorf("tool events = %+v, %+v, both want tool 'echo'", evs[0], evs[1])
	}
	if evs[1].Message != "echoed" {
		t.Errorf("tool result = %q, want 'echoed'", evs[1].Message)
	}
	if evs[2].Message != "done" {
		t.Errorf("final = %q, want 'done'", evs[2].Message)
	}

	// The tool result was fed back into the conversation as a tool message.
	last := a.Chat.Messages[len(a.Chat.Messages)-1]
	if last.Role != model.MessageRoleAssistant || last.Content != "done" {
		t.Errorf("last message = %+v, want the final assistant message", last)
	}
	foundToolResult := false
	for _, m := range a.Chat.Messages {
		if m.Role == model.MessageRoleTool && m.ToolCallID == "call_1" && m.Content == "echoed" {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Error("conversation lacks the tool result fed back for call_1")
	}
}

func TestRun_ModelError(t *testing.T) {
	a := NewAgent(&errModel{err: errors.New("boom")}, "fake-model")
	evs := collect(t, a.Run(context.Background(), "hi"))

	if len(evs) != 1 || evs[0].Kind != KindError || evs[0].Message != "boom" {
		t.Fatalf("events = %+v, want a single error 'boom'", evs)
	}
}

func TestRun_CancellationStopsLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := NewAgent(blockingModel{}, "fake-model")
	ch := a.Run(ctx, "hi")

	// Cancel while the producer is (or soon will be) blocked inside Complete.
	cancel()

	// The run must terminate and the channel close; if the producer leaks, this
	// times out. The channel closing proves the producer goroutine exited.
	collect(t, ch)
}

func TestRun_AssistantPrecedesToolCall(t *testing.T) {
	// The model emits text before deciding to call a tool: an assistant event must
	// surface before the tool activity.
	thinking := model.Message{Role: model.MessageRoleAssistant, Content: "let me check that", ToolCalls: []model.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "echo", Arguments: `{}`},
	}}}
	f := &fakeModel{responses: []*model.Response{
		finalResponse(thinking),
		finalResponse(model.Message{Role: model.MessageRoleAssistant, Content: "done"}),
	}}

	a := NewAgent(f, "fake-model")
	a.Tools.Register(&fakeTool{out: "echoed"})
	evs := collect(t, a.Run(context.Background(), "please check"))

	want := []struct {
		kind EventKind
		msg  string
	}{
		{KindAssistant, "let me check that"},
		{KindToolCall, ""},
		{KindToolResult, "echoed"},
		{KindFinal, "done"},
	}
	if len(evs) != len(want) {
		t.Fatalf("events = %+v, want %d events", evs, len(want))
	}
	for i, w := range want {
		if evs[i].Kind != w.kind {
			t.Errorf("event[%d].Kind = %s, want %s", i, evs[i].Kind, w.kind)
		}
		if w.msg != "" && evs[i].Message != w.msg {
			t.Errorf("event[%d].Message = %q, want %q", i, evs[i].Message, w.msg)
		}
	}
}

func TestNewAgent_InjectsCatalogWhenSkillsExist(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pdf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"),
		[]byte("---\nname: pdf\ndescription: extract text from PDFs\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := skills.New(root)
	if err != nil {
		t.Fatalf("skills.New: %v", err)
	}

	a := NewAgent(&errModel{err: errors.New("never called")}, "m", reg)
	dev := a.Chat.Messages[0].Content

	if !strings.Contains(dev, "read the SKILL.md") {
		t.Errorf("opening prompt lacks the activation instruction: %q", dev)
	}
	if !strings.Contains(dev, "pdf") || !strings.Contains(dev, "extract text from PDFs") {
		t.Errorf("opening prompt lacks the catalog entry: %q", dev)
	}
	if !strings.Contains(dev, filepath.Join(root, "pdf", "SKILL.md")) {
		t.Errorf("opening prompt lacks the skill location for bash-read activation: %q", dev)
	}
}

func TestNewAgent_OmitsCatalogWhenNoSkills(t *testing.T) {
	empty, err := skills.New(t.TempDir())
	if err != nil {
		t.Fatalf("skills.New: %v", err)
	}
	for _, a := range []*Agent{
		NewAgent(&errModel{err: errors.New("never called")}, "m", empty),
		NewAgent(&errModel{err: errors.New("never called")}, "m"), // no registry at all
	} {
		dev := a.Chat.Messages[0].Content
		if strings.Contains(dev, "skills provide specialized") {
			t.Errorf("catalog/instruction leaked into the prompt when no skills exist: %q", dev)
		}
	}
}
