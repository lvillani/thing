# Core owns the agentic loop and emits events on a channel

- Status: accepted

## Context and Problem Statement

The agent loop ("call the model, run any tool calls, repeat until a final answer") is
the whole point of this project, and it needs to be shared by every UI surface (TUI now,
ACP/AHP/RPC later). If the UI drives each request/response round, each surface
re-implements the loop.

## Decision

`internal/agent` runs the agent loop itself (`Agent.Run(ctx, input)`) and reports
progress to whatever UI is attached by writing typed `Event`s onto a channel. We chose a
channel over a synchronous callback because the project doubles as a Go-concurrency
exercise; the stream's lifecycle (producer closes the channel exactly once, consumer
selects on `ctx.Done()`) is treated as an explicit contract.

## Consequences

- Every UI surface shares one implementation of the loop instead of re-implementing it,
  and the loop lives in the core where we can read it.
- Sync observer callback was rejected — simpler and goroutine-free, but it forfeits the
  concurrency lesson the user wants.
- UI-driven loop (the current code) was rejected — it duplicates the loop per surface.
