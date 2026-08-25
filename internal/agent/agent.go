// SPDX-License-Identifier: GPL-3.0-only

// Package agent implements a basic and protocol-agnostic agentic loop.
package agent

import (
	"context"
	"fmt"
	"os"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"

	"thing/internal/config"
	"thing/internal/model"
	"thing/internal/provider"
	"thing/internal/skills"
	"thing/internal/tools"
)

// Agent represents an agent. It holds the conversation state.
type Agent struct {
	Tools     *tools.Registry
	Transport provider.Provider
	ModelInfo *catwalk.Model
	Chat      model.Chat
	skills    *skills.Registry // retained so run can resolve /skill:<name> activation

	usageMu sync.RWMutex
	usage   Usage
}

// Usage is a point-in-time snapshot of the live context usage of the most recent
// model response. PromptTokens/CompletionTokens/CachedTokens are the size of the
// prompt, completion and cached-prompt tokens of the last request (a gauge, not a
// cumulative tally); CachedTokensRatio is that response's cache hit rate (cached
// prompt tokens / total prompt tokens).
type Usage struct {
	PromptTokens      int
	CompletionTokens  int
	CachedTokens      int
	CachedTokensRatio float64
}

// NewAgent creates a new agent with the given model transport and configuration. If a
// skill registry is supplied and it has skills, its catalog is injected into the
// opening prompt so the model knows what it can load; with no skills the catalog is
// omitted.
func NewAgent(t provider.Provider, cfg config.Config, reg ...*skills.Registry) (*Agent, error) {
	toolRegistry := tools.NewRegistry()

	var skillsRegistry *skills.Registry
	if len(reg) > 0 {
		skillsRegistry = reg[0]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	systemPrompt, err := buildSystemPrompt(cwd, skillsRegistry)
	if err != nil {
		return nil, err
	}

	return &Agent{
		Tools:     toolRegistry,
		Transport: t,
		ModelInfo: findModel(cfg.Model),
		skills:    skillsRegistry,
		Chat: model.Chat{
			Model:           cfg.Model,
			ReasoningEffort: cfg.ReasoningEffort,
			SessionID:       model.NewSessionID(),
			Messages:        []model.Message{{Role: model.MessageRoleSystem, Content: systemPrompt}},
			Tools:           toolRegistry.Tools(),
		},
	}, nil
}

// Run drives the agent loop. It appends message to the conversation, then repeatedly
// calls the model, sends each complete conversation message on messages, and runs any
// requested tools. The messages channel is closed exactly once when the run finishes.
// Transport and other terminal errors are sent on errors, which is buffered to allow a
// consumer to drain messages without also selecting on errors. Both channels are
// closed when the run ends.
//
// The message channel contains the user message, complete assistant messages returned
// by the provider, and tool-result messages. An assistant message with ToolCalls is
// emitted before those calls are executed. Tool failures are returned to the model as
// tool-result messages and are not sent on errors.
func (a *Agent) Run(ctx context.Context, message *model.Message) (<-chan model.Message, <-chan error) {
	messages := make(chan model.Message)
	errors := make(chan error, 1)
	go func() {
		defer close(messages)
		defer close(errors)
		a.run(ctx, message, messages, errors)
	}()
	return messages, errors
}

func (a *Agent) run(ctx context.Context, message *model.Message, messages chan<- model.Message, errors chan<- error) {
	a.Chat.Messages = append(a.Chat.Messages, *message)
	if !a.emitMessage(ctx, messages, *message) {
		return
	}

	for {
		response, err := a.Transport.Complete(ctx, a.Chat)
		if err != nil {
			a.emitError(errors, err)
			return
		}

		// We don't explicitly set "n" and its default is "1", hence we expect exactly
		// one choice.
		if len(response.Choices) != 1 {
			a.emitError(errors, fmt.Errorf("model returned %d choices, expected 1", len(response.Choices)))
			return
		}

		msg := response.Choices[0].Message
		a.Chat.Messages = append(a.Chat.Messages, msg)
		a.accumulateUsage(response.Usage)
		if !a.emitMessage(ctx, messages, msg) {
			return
		}

		if len(msg.ToolCalls) == 0 {
			return
		}

		for _, toolCall := range msg.ToolCalls {
			result, err := a.Tools.Run(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				// A failed tool call must not terminate the run: feed the failure back
				// to the model as a tool result so it can see the error and react
				// (retry, apologise, or pick a different approach).
				result = "error: " + err.Error()
			}

			toolMessage := model.Message{
				Role:       model.MessageRoleTool,
				ToolCallID: toolCall.ID,
				Content:    result,
			}
			a.Chat.Messages = append(a.Chat.Messages, toolMessage)
			if !a.emitMessage(ctx, messages, toolMessage) {
				return
			}
		}
	}
}

// emitMessage delivers a message, returning false when ctx is cancelled so the run
// can stop even if the consumer has stopped draining the channel.
func (a *Agent) emitMessage(ctx context.Context, messages chan<- model.Message, message model.Message) bool {
	select {
	case messages <- message:
		return true
	case <-ctx.Done():
		return false
	}
}

// emitError reports a terminal error. The error channel has capacity for the single
// terminal error a run can produce, so reporting it does not depend on the consumer
// draining the channel while the producer is finishing.
func (a *Agent) emitError(errors chan<- error, err error) {
	errors <- err
}

// accumulateUsage records the live context usage of the most recent model response.
// The Chat Completions API reports prompt_tokens as the size of the context sent for
// that request, which grows as the conversation lengthens — accumulating it would
// vastly over-count the context window. So we replace (not add to) the running usage
// with the latest response's, keeping the counters as a gauge of live context rather
// than throughput.
func (a *Agent) accumulateUsage(usage *model.ResponseUsage) {
	if usage == nil {
		return
	}
	u := Usage{PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens}
	if details := usage.PromptTokensDetails; details != nil {
		u.CachedTokens = details.CachedTokens
		u.CachedTokensRatio = float64(u.CachedTokens) / float64(u.PromptTokens)
	}
	a.usageMu.Lock()
	a.usage = u
	a.usageMu.Unlock()
}

// Usage returns a snapshot of the live context usage of the most recent model
// response, safe to call from any goroutine (e.g. a UI render loop) while a run is
// in flight.
func (a *Agent) Usage() Usage {
	a.usageMu.RLock()
	defer a.usageMu.RUnlock()
	return a.usage
}

// ActivateSkill is the core-side activation operation for user-explicit skill
// invocation. It resolves name against the skill registry and returns a short pointer
// string nudging the model to read that skill's SKILL.md, then passes the remaining task
// through. It never decides whether input is a command — parsing the "/skill:" syntax is
// the interaction surface's job. It returns a non-nil error when no skill by that name
// is in the registry (in which case no pointer is produced and the caller should not
// start a run).
func (a *Agent) ActivateSkill(name, task string) (string, error) {
	if a.skills == nil {
		return "", fmt.Errorf("no skills available to activate")
	}
	skill, found := a.skills.Get(name)
	if !found {
		return "", fmt.Errorf("unknown skill %q", name)
	}
	if task == "" {
		return fmt.Sprintf(
			"The user has manually activated the skill %q. Read its instructions at %s and follow them.",
			skill.Name, skill.Location), nil
	}
	return fmt.Sprintf(
		"The user has manually activated the skill %q. Read its instructions at %s to follow them, then help with: %s",
		skill.Name, skill.Location, task), nil
}

// Skills returns the already-discovered skill catalog (name + description) for the
// UI to filter and render. It is a read-only accessor so the TUI never re-discovers
// skills from disk or leaks the registry type.
func (a *Agent) Skills() []skills.Skill {
	if a.skills == nil {
		return nil
	}
	return a.skills.Catalog()
}

// findModel returns metadata for modelID from Catwalk's embedded catalog.
// It returns nil when the model is not in the offline catalog.
func findModel(modelID string) *catwalk.Model {
	for _, provider := range embedded.GetAll() {
		for i := range provider.Models {
			if provider.Models[i].ID == modelID {
				return &provider.Models[i]
			}
		}
	}
	return nil
}
