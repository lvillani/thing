// SPDX-License-Identifier: GPL-3.0-only

// Package model contains the data model used in the application which is, for the most
// part, a subset of the OpenAPI Chat Completion API data model.
package model

// MessageRole represents the role of a message in a chat conversation.
type MessageRole string

const (
	MessageRoleDeveloper MessageRole = "developer" // Developer-provided instructions that the model should follow.
	MessageRoleUser      MessageRole = "user"      // Messages sent by an end user.
	MessageRoleAssistant MessageRole = "assistant" // Messages sent by the model in response to user messages.
	MessageRoleTool      MessageRole = "tool"      // Messages sent by a tool in response to a model's tool call.
)

// ToolType represents the type of a tool used in the chat conversation.
type ToolType string

const (
	ToolTypeFunction ToolType = "function" // A function tool that can be called by the model to generate a response.
)

// Chat represents a chat conversation with a model, including the messages exchanged
// and any tools used.
type Chat struct {
	Model    string    `json:"model"`
	Tools    []Tool    `json:"tools"`
	Messages []Message `json:"messages"`
}

// Message represents a single message in a chat conversation.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a call to a function tool made by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // Should always be "function".
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // Should be in JSON format.
	} `json:"function"`
}

// Tool represents a function tool that can be used to generate a response.
type Tool struct {
	Type     ToolType               `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition represents the definition of a function tool.
type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Strict      bool           `json:"strict"`                // Must always be true.
	Description string         `json:"description,omitempty"` // Optional description of the tool.
	Parameters  map[string]any `json:"parameters,omitempty"`  // JSON Schema describing the function's parameters.
}

// NewTool creates a new Tool instance with the given name and parameters.
func NewTool(name string, parameters map[string]any) *Tool {
	return &Tool{
		Type: ToolTypeFunction,
		Function: ToolFunctionDefinition{
			Name:        name,
			Strict:      true,
			Description: "",
			Parameters:  parameters,
		},
	}
}

// Response represents the response from the model.
type Response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *ResponseUsage `json:"usage,omitempty"`
}

// ResponseUsage represents the usage information for a model response.
type ResponseUsage struct {
	PromptTokens        int                   `json:"prompt_tokens"`
	CompletionTokens    int                   `json:"completion_tokens"`
	TotalTokens         int                   `json:"total_tokens"`
	PromptTokensDetails *ResponseUsageDetails `json:"prompt_tokens_details,omitempty"`
}

// ResponseUsageDetails represents detailed usage information for a model response.
type ResponseUsageDetails struct {
	CachedTokens int `json:"cached_tokens"`
}
