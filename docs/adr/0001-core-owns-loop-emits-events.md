# Core owns the agentic loop and emits events on a channel

- Status: accepted

## Context and Problem Statement

The agent loop is the heart of this project. It calls the model, runs any tool calls,
and repeats until it reaches a final answer. Every UI surface (the TUI now, and
ACP/AHP/RPC later) needs this loop. If the UI drives each request/response round, every
surface ends up re-implementing the same loop.

## Decision

`internal/agent` runs the loop itself with `Agent.Run(ctx, input)`. It reports progress
to whatever UI is attached by writing typed `Event`s onto a channel. We picked a channel
over a synchronous callback because the project doubles as a Go-concurrency exercise.
The stream's lifecycle is a contract: the producer closes the channel exactly once, and
the consumer selects on `ctx.Done()`.

## Consequences

- Every UI surface shares one loop implementation instead of re-implementing it.
- The loop lives in the core, where we can read it.
- Rejected: a synchronous observer callback. It is simpler and goroutine-free, but it
  forfeits the concurrency lesson the user wants.
- Rejected: a UI-driven loop (the current code). It duplicates the loop per surface.
