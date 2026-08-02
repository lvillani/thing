// SPDX-License-Identifier: GPL-3.0-only

// Package agent implements a basic and protocol-agnostic agentic loop.
package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"thing/internal/model"
	"thing/internal/skills"
	"thing/internal/tools"
)

// baseSystemPrompt is the static opening of every agent's system prompt.
const baseSystemPrompt = `
You are an expert assistant operating inside an agent harness.
`

// systemPromptWithCwd returns the system prompt prefixed with the current working
// directory so the model knows where in the filesystem it is running.
func systemPromptWithCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return baseSystemPrompt
	}
	return fmt.Sprintf("Your current working directory is %s.\n%s", cwd, baseSystemPrompt)
}

// Model is the seam to a model transport: something that can send a conversation and
// return the model's reply. The core depends on this interface, never on HTTP or a
// concrete backend.
type Model interface {
	Complete(ctx context.Context, chat model.Chat) (*model.Response, error)
}

// Agent represents an agent. It holds the conversation state.
type Agent struct {
	Tools *tools.ToolRegistry
	Model Model
	Chat  model.Chat

	TotalPromptTokens      int
	TotalCompletionTokens  int
	TotalCachedTokens      int
	TotalCachedTokensRatio float64
}

// NewAgent creates a new agent with the given model transport and model name. If a skill
// registry is supplied and it has skills, their catalog is injected into the opening
// prompt so the model knows what it can load; with no skills the catalog is omitted.
func NewAgent(m Model, modelName string, reg ...*skills.Registry) *Agent {
	toolRegistry := tools.NewToolRegistry()
	prompt := systemPromptWithCwd()
	if len(reg) > 0 && reg[0] != nil {
		if cat := reg[0].Catalog(); len(cat) > 0 {
			prompt = promptWithCatalog(prompt, cat)
		}
	}

	return &Agent{
		Tools: toolRegistry,
		Model: m,
		Chat: model.Chat{
			Model:    modelName,
			Messages: []model.Message{{Role: model.MessageRoleDeveloper, Content: prompt}},
			Tools:    toolRegistry.Tools(),
		},
	}
}

// promptWithCatalog appends the tier-1 skill catalog and a short activation instruction
// to the base prompt.
func promptWithCatalog(base string, cat []skills.Skill) string {
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\nThe following skills provide specialized instructions for specific tasks.\n")
	b.WriteString("When a task matches a skill's description, use bash to read the SKILL.md at its location before proceeding.\n")
	for _, s := range cat {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", s.Name, s.Description, s.Location)
	}
	return b.String()
}

// Run drives the agent loop: it appends the user's message, then repeatedly calls the
// model, runs any requested tools, and reports each step as an Event on the returned
// channel. The channel is closed exactly once when the run finishes. Cancelling ctx
// stops the loop and terminates the producer goroutine — even if the consumer stops
// draining the channel; cancellation is observed as soon as the model transport or
// event delivery returns to the loop.
func (a *Agent) Run(ctx context.Context, userInput string) <-chan Event {
	events := make(chan Event)
	go func() {
		defer close(events)
		a.run(ctx, userInput, events)
	}()
	return events
}

func (a *Agent) run(ctx context.Context, userInput string, events chan<- Event) {
	a.Chat.Messages = append(a.Chat.Messages, model.Message{Role: model.MessageRoleUser, Content: userInput})

	for {
		response, err := a.Model.Complete(ctx, a.Chat)
		if err != nil {
			a.emit(ctx, events, Event{Kind: KindError, Message: err.Error()})
			return
		}
		if len(response.Choices) == 0 {
			a.emit(ctx, events, Event{Kind: KindError, Message: "model returned no choices"})
			return
		}

		msg := response.Choices[0].Message
		a.Chat.Messages = append(a.Chat.Messages, msg)
		a.accumulateUsage(response.Usage)

		if len(msg.ToolCalls) == 0 {
			a.emit(ctx, events, Event{
				Kind:             KindFinal,
				Message:          msg.Content,
				PromptTokens:     a.TotalPromptTokens,
				CompletionTokens: a.TotalCompletionTokens,
				CachedTokens:     a.TotalCachedTokens,
			})
			return
		}

		if msg.Content != "" && !a.emit(ctx, events, Event{Kind: KindAssistant, Message: msg.Content}) {
			return
		}
		for _, toolCall := range msg.ToolCalls {
			if !a.emit(ctx, events, Event{Kind: KindToolCall, Tool: toolCall.Function.Name}) {
				return
			}
			result, err := a.Tools.Run(toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				// A failed tool call must not terminate the run: feed the failure back
				// to the model as a tool result so it can see the error and react
				// (retry, apologise, or pick a different approach). The event is a tool
				// result carrying the error text, not a terminal KindError.
				result = "error: " + err.Error()
				if !a.emit(ctx, events, Event{Kind: KindToolResult, Tool: toolCall.Function.Name, Message: result}) {
					return
				}
				a.Chat.Messages = append(a.Chat.Messages, model.Message{
					Role:       model.MessageRoleTool,
					ToolCallID: toolCall.ID,
					Content:    result,
				})
				continue
			}
			if !a.emit(ctx, events, Event{Kind: KindToolResult, Tool: toolCall.Function.Name, Message: result}) {
				return
			}
			a.Chat.Messages = append(a.Chat.Messages, model.Message{
				Role:       model.MessageRoleTool,
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}
	}
}

// emit delivers an event, returning false when ctx is cancelled so the run can stop
// even if the consumer has stopped draining the channel.
func (a *Agent) emit(ctx context.Context, events chan<- Event, ev Event) bool {
	select {
	case events <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// accumulateUsage adds a response's token accounting to the running totals.
func (a *Agent) accumulateUsage(usage *model.ResponseUsage) {
	if usage == nil {
		return
	}
	a.TotalPromptTokens += usage.PromptTokens
	a.TotalCompletionTokens += usage.CompletionTokens
	if details := usage.PromptTokensDetails; details != nil {
		a.TotalCachedTokens += details.CachedTokens
		a.TotalCachedTokensRatio = float64(a.TotalCachedTokens) / float64(a.TotalPromptTokens)
	}
}
