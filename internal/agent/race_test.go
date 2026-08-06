package agent

import (
	"context"
	"sync"
	"testing"

	"thing/internal/model"
)

// loopModel keeps yielding tool calls so the run goroutine writes usage repeatedly
// while a reader goroutine reads it concurrently — mirroring the TUI reading the
// usage gauge while a run is in flight.
type loopModel struct{ n int }

func (m *loopModel) Complete(_ context.Context, chat model.Chat) (*model.Response, error) {
	msg := model.Message{Role: model.MessageRoleAssistant, Content: "done", ToolCalls: []model.ToolCall{{
		ID: "c", Type: "function",
		Function: struct {
			Name      string                          `json:"name"`
			Arguments model.ToolCallFunctionArguments `json:"arguments"`
		}{Name: "echo", Arguments: `{}`},
	}}}
	if m.n <= 0 {
		msg.ToolCalls = nil
	}
	m.n--
	return &model.Response{
		Choices: []struct {
			Message model.Message `json:"message"`
		}{{Message: msg}},
		Usage: &model.ResponseUsage{PromptTokens: 1000, CompletionTokens: 4,
			PromptTokensDetails: &model.ResponseUsageDetails{CachedTokens: 500}},
	}, nil
}

func TestRace_UsageReadWhileRunInFlight(t *testing.T) {
	a, _ := NewAgent(&loopModel{n: 20000}, "fake-model")
	a.Tools.Register(&fakeTool{out: "ok"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := a.Run(ctx, model.NewUserMessage("go"))
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = a.Usage() // must be safe from another goroutine while run writes it
			}
		}
	}()

	for range events {
	}
	close(stop)
	wg.Wait()
}
