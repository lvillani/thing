// SPDX-License-Identifier: GPL-3.0-only

package agent

import "thing/internal/model"

// EventKind identifies the kind of progress an Event reports.
type EventKind string

const (
	// KindUser reports that the user sent a message to the model.
	KindUser EventKind = "user"
	// KindAssistant reports an assistant message produced before tool calls.
	KindAssistant EventKind = "assistant"
	// KindToolCall reports that the model invoked a tool.
	KindToolCall EventKind = "tool_call"
	// KindToolResult reports a tool's output.
	KindToolResult EventKind = "tool_result"
	// KindError reports a terminal failure of the run.
	KindError EventKind = "error"
	// KindFinal reports the terminal answer that ends the run.
	KindFinal EventKind = "final"
)

// Event is a unit of progress emitted by the core while it runs an agent loop. The
// producer writes Events onto the channel returned by Run and closes it exactly once
// when the run ends.
type Event struct {
	Kind EventKind
	// Message carries assistant/final content, a tool result's output, or an error
	// text, depending on Kind.
	Message string
	// Tool names the tool for KindToolCall and KindToolResult.
	Tool string
	// ToolInput carries the raw arguments passed to the tool (e.g. the bash command),
	// populated on KindToolCall so the UI can display what actually ran.
	ToolInput model.ToolCallFunctionArguments
	// Live context usage of the most recent response, populated on KindFinal.
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
}
