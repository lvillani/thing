// SPDX-License-Identifier: GPL-3.0-only

// Package provider provides a transport for an OpenAI-compatible Chat Completions
// endpoint.
package provider

import (
	"context"

	"thing/internal/model"
)

// Provider is a transport for a model provider. It can send a conversation and return
// the model's reply.
type Provider interface {
	// Complete sends the conversation to the model and returns the reply.
	Complete(ctx context.Context, chat model.Chat) (*model.Response, error)
}
