// SPDX-License-Identifier: GPL-3.0-only

// Package model contains the data model used in the application which is, for the most
// part, a subset of the OpenAPI Chat Completion API data model.
package model

import (
	"crypto/rand"
	"fmt"
)

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

// Chat represents a chat conversation with a model, including the messages exchanged,
// any tools used, and a session identifier. The session identifier is transported in
// the request body because providers such as OpenRouter can use it to group requests
// together to maximize cache hits. Keeping it here means it serializes with the
// conversation and so is restored naturally when deserializing it.
type Chat struct {
	SessionID string    `json:"session_id,omitempty"`
	Model     string    `json:"model"`
	Tools     []Tool    `json:"tools"`
	Messages  []Message `json:"messages"`
}

// NewSessionID returns a random UUIDv4 used to identify a conversation.
func NewSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("model: generate session id: %v", err))
	}

	// RFC 4122 version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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

// Message represents a single message in a chat conversation.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// NewUserMessage creates a new Message instance with the role set to "user" and the
// given content.
func NewUserMessage(userInput string) *Message {
	return &Message{Role: MessageRoleUser, Content: userInput}
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
