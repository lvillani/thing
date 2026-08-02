# Core owns the agentic loop and emits events on a channel

`internal/agent` runs the agent loop itself (`Agent.Run(ctx, input)`) and reports
progress to whatever UI is attached by writing typed `Event`s onto a channel, rather
than letting the UI drive each request/response round. We chose this so every UI surface
(TUI now, ACP/AHP/RPC later) shares one implementation of "call the model, run any tool
calls, repeat until a final answer" instead of each re-implementing the loop, and so the
loop — the whole point of this project — lives in the core where we can read it. The
channel is over a synchronous callback because the project doubles as a Go-concurrency
exercise; a stream's lifecycle (producer closes the channel exactly once, consumer
selects on `ctx.Done()`) is treated as an explicit contract.

*Status: accepted*

*Considered Options:* sync observer callback (simpler, no goroutines — rejected,
forfeits the concurrency lesson the user wants); UI-driven loop (current code —
rejected, duplicates the loop per surface).
