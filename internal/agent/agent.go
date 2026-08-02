// SPDX-License-Identifier: GPL-3.0-only

// Package agent implements a basic and protocol-agnostic agentic loop.
package agent

import (
	"context"

	"thing/internal/model"
	"thing/internal/tools"
)

const systemPrompt = `
You are an expert assistant operating inside an agent harness.
`

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

// NewAgent creates a new agent with the given model transport and model name.
func NewAgent(m Model, modelName string) *Agent {
	toolRegistry := tools.NewToolRegistry()

	return &Agent{
		Tools: toolRegistry,
		Model: m,
		Chat: model.Chat{
			Model:    modelName,
			Messages: []model.Message{{Role: model.MessageRoleDeveloper, Content: systemPrompt}},
			Tools:    toolRegistry.Tools(),
		},
	}
}

// SendMessage appends a user message to the conversation.
func (a *Agent) SendMessage(content string) {
	a.Chat.Messages = append(a.Chat.Messages, model.Message{Role: model.MessageRoleUser, Content: content})
}

// ProcessResponse appends the assistant's message to the conversation and updates usage
// stats.
func (a *Agent) ProcessResponse(response *model.Response) (bool, error) {
	a.Chat.Messages = append(a.Chat.Messages, response.Choices[0].Message)

	for _, toolCall := range response.Choices[0].Message.ToolCalls {
		result, err := a.Tools.Run(toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			return false, err
		}

		a.Chat.Messages = append(a.Chat.Messages, model.Message{
			Role:       model.MessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    result,
		})
	}

	if response.Usage != nil {
		a.TotalPromptTokens += response.Usage.PromptTokens
		a.TotalCompletionTokens += response.Usage.CompletionTokens
		if details := response.Usage.PromptTokensDetails; details != nil {
			a.TotalCachedTokens += details.CachedTokens
			a.TotalCachedTokensRatio = float64(a.TotalCachedTokens) / float64(a.TotalPromptTokens)
		}
	}

	return len(response.Choices[0].Message.ToolCalls) > 0, nil
}
