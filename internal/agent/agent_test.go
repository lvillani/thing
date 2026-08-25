// SPDX-License-Identifier: GPL-3.0-only

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"thing/internal/config"
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

func (t *fakeTool) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	return t.out, nil
}

// collect drains both run channels with a timeout so a leaked producer cannot hang
// the test.
type runResult struct {
	messages []model.Message
	errors   []error
}

func collect(t *testing.T, messages <-chan model.Message, errors <-chan error) runResult {
	t.Helper()
	result := runResult{}
	for messages != nil || errors != nil {
		select {
		case message, ok := <-messages:
			if !ok {
				messages = nil
				continue
			}
			result.messages = append(result.messages, message)
		case err, ok := <-errors:
			if !ok {
				errors = nil
				continue
			}
			result.errors = append(result.errors, err)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for run; got %+v", result)
		}
	}
	return result
}

func collectRun(t *testing.T, a *Agent, ctx context.Context, message *model.Message) runResult {
	t.Helper()
	messages, errors := a.Run(ctx, message)
	return collect(t, messages, errors)
}

func sameMessage(a, b model.Message) bool {
	return reflect.DeepEqual(a, b)
}

// errTool always fails, used to exercise tool-error propagation in the loop.
type errTool struct{}

func (t *errTool) Describe() model.Tool {
	return model.Tool{Type: model.ToolTypeFunction, Function: model.ToolFunctionDefinition{Name: "boom"}}
}

func (t *errTool) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	return "", errors.New("kaboom")
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

	a, _ := NewAgent(f, config.Config{Model: "fake-model"})
	run := collectRun(t, a, context.Background(), model.NewUserMessage("hi"))

	if len(run.messages) != 2 || run.messages[0].Role != model.MessageRoleUser ||
		run.messages[0].Content != "hi" || !sameMessage(run.messages[1], reply) {
		t.Fatalf("messages = %+v, want user 'hi' then the complete reply %+v", run.messages, reply)
	}
	if len(run.errors) != 0 {
		t.Fatalf("errors = %+v, want none", run.errors)
	}

	// Conversation ends with the assistant reply; usage is accumulated in the core.
	last := a.Chat.Messages[len(a.Chat.Messages)-1]
	if !sameMessage(last, reply) {
		t.Errorf("last message = %+v, want assistant %+v", last, reply)
	}
	if u := a.Usage(); u.PromptTokens != 10 || u.CompletionTokens != 4 || u.CachedTokens != 3 {
		t.Errorf("usage = in %d/out %d/cached %d, want 10/4/3",
			u.PromptTokens, u.CompletionTokens, u.CachedTokens)
	}
}

func TestRun_ToolRoundThenFinal(t *testing.T) {
	toolResp := model.Message{Role: model.MessageRoleAssistant, Content: "", ToolCalls: []model.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string                          `json:"name"`
			Arguments model.ToolCallFunctionArguments `json:"arguments"`
		}{Name: "echo", Arguments: `{"text":"x"}`},
	}}}
	final := model.Message{Role: model.MessageRoleAssistant, Content: "done"}
	f := &fakeModel{responses: []*model.Response{
		finalResponse(toolResp),
		finalResponse(final),
	}}

	a, _ := NewAgent(f, config.Config{Model: "fake-model"})
	a.Tools.Register(&fakeTool{out: "echoed"})

	run := collectRun(t, a, context.Background(), model.NewUserMessage("please echo"))

	if len(run.messages) != 4 {
		t.Fatalf("messages = %+v, want user, assistant tool call, tool result, final", run.messages)
	}
	if run.messages[0].Role != model.MessageRoleUser || run.messages[0].Content != "please echo" {
		t.Errorf("user message = %+v", run.messages[0])
	}
	if !sameMessage(run.messages[1], toolResp) {
		t.Errorf("assistant message = %+v, want complete model message %+v", run.messages[1], toolResp)
	}
	if run.messages[2].Role != model.MessageRoleTool || run.messages[2].ToolCallID != "call_1" ||
		run.messages[2].Content != "echoed" {
		t.Errorf("tool result = %+v, want tool result for call_1", run.messages[2])
	}
	if !sameMessage(run.messages[3], final) {
		t.Errorf("final message = %+v, want %+v", run.messages[3], final)
	}
	if len(run.errors) != 0 {
		t.Fatalf("errors = %+v, want none", run.errors)
	}

	// The tool result was fed back into the conversation as a tool message.
	last := a.Chat.Messages[len(a.Chat.Messages)-1]
	if !sameMessage(last, final) {
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
	a, _ := NewAgent(&errModel{err: errors.New("boom")}, config.Config{Model: "fake-model"})
	run := collectRun(t, a, context.Background(), model.NewUserMessage("hi"))

	if len(run.messages) != 1 || run.messages[0].Role != model.MessageRoleUser || run.messages[0].Content != "hi" {
		t.Fatalf("messages = %+v, want user 'hi'", run.messages)
	}
	if len(run.errors) != 1 || run.errors[0].Error() != "boom" {
		t.Fatalf("errors = %+v, want a single error 'boom'", run.errors)
	}
}

func TestRun_CancellationStopsLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a, _ := NewAgent(blockingModel{}, config.Config{Model: "fake-model"})
	messages, errors := a.Run(ctx, model.NewUserMessage("hi"))

	// Cancel while the producer is (or soon will be) blocked inside Complete.
	cancel()

	// The run must terminate and both channels must close; if the producer leaks, this
	// times out.
	collect(t, messages, errors)
}

func TestRun_CompleteAssistantMessagePrecedesToolResult(t *testing.T) {
	// The complete assistant message, including its content and tool calls, must be
	// emitted before the tool result.
	thinking := model.Message{Role: model.MessageRoleAssistant, Content: "let me check that", ToolCalls: []model.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string                          `json:"name"`
			Arguments model.ToolCallFunctionArguments `json:"arguments"`
		}{Name: "echo", Arguments: `{}`},
	}}}
	final := model.Message{Role: model.MessageRoleAssistant, Content: "done"}
	f := &fakeModel{responses: []*model.Response{
		finalResponse(thinking),
		finalResponse(final),
	}}

	a, _ := NewAgent(f, config.Config{Model: "fake-model"})
	a.Tools.Register(&fakeTool{out: "echoed"})
	run := collectRun(t, a, context.Background(), model.NewUserMessage("please check"))

	if len(run.messages) != 4 {
		t.Fatalf("messages = %+v, want user, assistant, tool result, final", run.messages)
	}
	if !sameMessage(run.messages[1], thinking) {
		t.Errorf("assistant message = %+v, want %+v", run.messages[1], thinking)
	}
	if run.messages[2].Role != model.MessageRoleTool || run.messages[2].Content != "echoed" {
		t.Errorf("tool result = %+v, want echoed tool result", run.messages[2])
	}
	if !sameMessage(run.messages[3], final) {
		t.Errorf("final message = %+v, want %+v", run.messages[3], final)
	}
}

func TestRun_ToolErrorFeedsBackToModel(t *testing.T) {
	// A tool call that errors must not terminate the run. The failure is fed back to
	// the model as a tool-result message, so the model can see and react to it.
	toolResp := model.Message{Role: model.MessageRoleAssistant, ToolCalls: []model.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string                          `json:"name"`
			Arguments model.ToolCallFunctionArguments `json:"arguments"`
		}{Name: "boom", Arguments: `{}`},
	}}}
	final := model.Message{Role: model.MessageRoleAssistant, Content: "ok, noted the failure"}
	f := &fakeModel{responses: []*model.Response{
		finalResponse(toolResp),
		finalResponse(final),
	}}

	a, _ := NewAgent(f, config.Config{Model: "fake-model"})
	a.Tools.Register(&errTool{})

	run := collectRun(t, a, context.Background(), model.NewUserMessage("trigger a failure"))

	if len(run.messages) != 4 {
		t.Fatalf("messages = %+v, want user, assistant, failed tool result, final", run.messages)
	}
	if !sameMessage(run.messages[1], toolResp) {
		t.Errorf("assistant message = %+v, want %+v", run.messages[1], toolResp)
	}
	if run.messages[2].Role != model.MessageRoleTool || !strings.Contains(run.messages[2].Content, "kaboom") {
		t.Errorf("tool result = %+v, want it to carry tool error 'kaboom'", run.messages[2])
	}
	if !sameMessage(run.messages[3], final) {
		t.Errorf("final = %+v, want %+v", run.messages[3], final)
	}
	if len(run.errors) != 0 {
		t.Fatalf("errors = %+v, want none for a recoverable tool error", run.errors)
	}

	found := false
	for _, m := range a.Chat.Messages {
		if m.Role == model.MessageRoleTool && m.ToolCallID == "call_1" && strings.Contains(m.Content, "kaboom") {
			found = true
		}
	}
	if !found {
		t.Error("conversation lacks the tool error fed back to the model for call_1")
	}
}

// growingContextModel reports a distinct live context size per request, growing as the
// conversation lengthens — the shape of a real agentic conversation where each round
// re-sends the accumulated history. Used to prove usage tracks the live context gauge,
// not a cumulative throughput tally.
type growingContextModel struct {
	prompts []int
	rounds  []model.Message
}

func (g *growingContextModel) Complete(_ context.Context, chat model.Chat) (*model.Response, error) {
	var msg model.Message
	if len(g.rounds) > 0 {
		msg = g.rounds[0]
		g.rounds = g.rounds[1:]
	} else {
		msg = model.Message{Role: model.MessageRoleAssistant, Content: "final"}
	}
	prompt := g.prompts[0]
	g.prompts = g.prompts[1:]
	return &model.Response{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: msg}},
		Usage: &model.ResponseUsage{
			PromptTokens:        prompt,
			CompletionTokens:    4,
			TotalTokens:         prompt + 4,
			PromptTokensDetails: &model.ResponseUsageDetails{CachedTokens: prompt / 2},
		},
	}, nil
}

func TestRun_UsageTracksLiveContextNotAccumulatedThroughput(t *testing.T) {
	// Three requests with growing context sizes: 1000, 2000, 3000. The live-context
	// gauge must report 3000/4 cached 1500 — not the sum 6000.
	toolMsg := model.Message{Role: model.MessageRoleAssistant,
		ToolCalls: []model.ToolCall{{ID: "call_1", Type: "function", Function: struct {
			Name      string                          `json:"name"`
			Arguments model.ToolCallFunctionArguments `json:"arguments"`
		}{Name: "echo", Arguments: `{}`}}}}
	g := &growingContextModel{
		prompts: []int{1000, 2000, 3000},
		rounds:  []model.Message{toolMsg, toolMsg},
	}
	a, _ := NewAgent(g, config.Config{Model: "fake-model"})
	a.Tools.Register(&fakeTool{out: "echoed"})

	collectRun(t, a, context.Background(), model.NewUserMessage("grow"))

	if u := a.Usage(); u.PromptTokens != 3000 {
		t.Errorf("PromptTokens = %d, want 3000 (live context of last request, not cumulative)", u.PromptTokens)
	}
	if u := a.Usage(); u.CompletionTokens != 4 {
		t.Errorf("CompletionTokens = %d, want 4", u.CompletionTokens)
	}
	if u := a.Usage(); u.CachedTokens != 1500 {
		t.Errorf("CachedTokens = %d, want 1500 (cached 3000/2 of last request)", u.CachedTokens)
	}
	if u := a.Usage(); u.CachedTokensRatio != 0.5 {
		t.Errorf("CachedTokensRatio = %v, want 0.5 (per-request cache hit rate)", u.CachedTokensRatio)
	}
}

// recordingModel records the user message the agent sent so activation can be
// asserted, then returns a single final response.
type recordingModel struct {
	gotUser string
}

func (f *recordingModel) Complete(_ context.Context, chat model.Chat) (*model.Response, error) {
	for _, m := range chat.Messages {
		if m.Role == model.MessageRoleUser {
			f.gotUser = m.Content
		}
	}
	return &model.Response{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: model.Message{Role: model.MessageRoleAssistant, Content: "done"}}},
	}, nil
}

func newSkillReg(t *testing.T, dir, content string) *skills.Registry {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, dir, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := skills.New(root)
	if err != nil {
		t.Fatalf("skills.New: %v", err)
	}
	return reg
}

func TestActivateSkillNudgesModel(t *testing.T) {
	content := "---\nname: git\ndescription: follow repo conventions\n---\nbody\n"
	reg := newSkillReg(t, "git", content)
	skill, _ := reg.Get("git")

	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"}, reg)
	pointer, err := a.ActivateSkill("git", "make a commit")
	if err != nil {
		t.Fatalf("ActivateSkill: %v", err)
	}
	if !strings.Contains(pointer, skill.Name) {
		t.Errorf("pointer lacks skill name: %q", pointer)
	}
	if !strings.Contains(pointer, skill.Location) {
		t.Errorf("pointer lacks skill location %q: %q", skill.Location, pointer)
	}
	if !strings.Contains(pointer, "make a commit") {
		t.Errorf("pointer lost the trailing task: %q", pointer)
	}
}

func TestActivateSkillWithoutTask(t *testing.T) {
	reg := newSkillReg(t, "git", "---\nname: git\ndescription: x\n---\n")
	skill, _ := reg.Get("git")

	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"}, reg)
	pointer, err := a.ActivateSkill("git", "")
	if err != nil {
		t.Fatalf("ActivateSkill: %v", err)
	}
	if !strings.Contains(pointer, skill.Name) || !strings.Contains(pointer, skill.Location) {
		t.Errorf("pointer incomplete: %q", pointer)
	}
}

func TestActivateSkillUnknownName(t *testing.T) {
	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"}, newSkillReg(t, "git", "---\nname: git\ndescription: x\n---\n"))
	_, err := a.ActivateSkill("does-not-exist", "")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should name the skill: %v", err)
	}
}

func TestActivateSkillNoRegistry(t *testing.T) {
	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"})
	_, err := a.ActivateSkill("git", "")
	if err == nil {
		t.Fatal("expected error when no registry")
	}
}

func TestRun_NormalInputPassesThroughUnchanged(t *testing.T) {
	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"}, newSkillReg(t, "git", "---\nname: git\ndescription: x\n---\n"))
	// A "/skill:" command is passed through to Run unchanged: parsing is the TUI's
	// job, the core's Run should never see it as a resolved pointer.
	collectRun(t, a, context.Background(), model.NewUserMessage("/skill:git make a commit"))
	if m.gotUser != "/skill:git make a commit" {
		t.Errorf("Run should pass command through unchanged, got %q", m.gotUser)
	}
}

func TestSkillsAccessor(t *testing.T) {
	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"}, newSkillReg(t, "git", "---\nname: git\ndescription: do git things\n---\n"))
	cat := a.Skills()
	if len(cat) != 1 || cat[0].Name != "git" || cat[0].Description != "do git things" {
		t.Errorf("Skills() = %+v, want single git skill", cat)
	}
}

func TestSkillsAccessorNoRegistry(t *testing.T) {
	m := &recordingModel{}
	a, _ := NewAgent(m, config.Config{Model: "fake-model"})
	if cat := a.Skills(); len(cat) != 0 {
		t.Errorf("Skills() = %+v, want empty with no registry", cat)
	}
}

func TestNewAgent_HidesDisabledSkillFromSystemPrompt(t *testing.T) {
	visRoot := t.TempDir()
	hidRoot := t.TempDir()
	writeSkillAt(t, visRoot, "visible", "model may load", false)
	writeSkillAt(t, hidRoot, "hidden", "user only", true)

	combined, err := skills.New(visRoot, hidRoot)
	if err != nil {
		t.Fatalf("skills.New: %v", err)
	}

	a, _ := NewAgent(&errModel{err: errors.New("never called")}, config.Config{Model: "m"}, combined)
	sys := a.Chat.Messages[0].Content

	if !strings.Contains(sys, "visible") || !strings.Contains(sys, "model may load") {
		t.Errorf("visible skill missing from system prompt: %q", sys)
	}
	if strings.Contains(sys, "hidden") || strings.Contains(sys, "user only") {
		t.Errorf("disabled skill leaked into system prompt: %q", sys)
	}
	if !containsName(a.Skills(), "hidden") {
		t.Errorf("disabled skill missing from Skills() autocomplete catalog: %+v", a.Skills())
	}
	if !containsName(a.Skills(), "visible") {
		t.Errorf("visible skill missing from Skills(): %+v", a.Skills())
	}
}

func writeSkillAt(t *testing.T, root, name, desc string, disabled bool) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	flag := ""
	if disabled {
		flag = "disable-model-invocation: true\n"
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n" + flag + "---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "SKILL.md")
}

func containsName(cat []skills.Skill, name string) bool {
	for _, s := range cat {
		if s.Name == name {
			return true
		}
	}
	return false
}

// TestNewAgentSeedsSessionIDOnChat verifies a fresh conversation starts with a
// non-empty session id carried on the Chat (as serialized body metadata), so it can be
// restored by future history save/reload.
func TestNewAgentSeedsSessionIDOnChat(t *testing.T) {
	a, _ := NewAgent(&errModel{err: errors.New("never called")}, config.Config{Model: "fake-model"})
	if a.Chat.SessionID == "" {
		t.Fatal("NewAgent did not seed a session id on the conversation")
	}
}

func TestNewAgentLoadsEmbeddedModelInfo(t *testing.T) {
	a, err := NewAgent(&errModel{err: errors.New("never called")}, config.Config{Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}
	if a.ModelInfo == nil {
		t.Fatal("NewAgent did not load model metadata")
	}
	if a.ModelInfo.ID != "gpt-4o" {
		t.Errorf("ModelInfo.ID = %q, want %q", a.ModelInfo.ID, "gpt-4o")
	}
	if a.ModelInfo.ContextWindow == 0 {
		t.Error("ModelInfo.ContextWindow = 0, want model context metadata")
	}
}

func TestNewAgentAllowsUnknownModel(t *testing.T) {
	a, err := NewAgent(&errModel{err: errors.New("never called")}, config.Config{Model: "unknown-model"})
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}
	if a.ModelInfo != nil {
		t.Errorf("ModelInfo = %#v, want nil for unknown model", a.ModelInfo)
	}
}
