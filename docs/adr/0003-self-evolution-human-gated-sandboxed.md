# Self-evolution is human-gated and secured by OS sandboxing, not per-call confirmation

- Status: accepted

## Context and Problem Statement

The harness improves itself by exercising real tools (bash now; read/write/edit/glob
later) on its own source. The agent holds effectively unlimited host access inside the
core's tool loop (`bash` can write any file). Per-command confirmation is tuned out in
practice and breaks flow.

## Decision

We do not ask the human to confirm each tool call as it runs. The trust model is: the
human decides what to ask (the question-level gate), and the real technical constraint
is left to OS sandboxing primitives — Seatbelt on macOS and Landlock on Linux — pinned
to give the agent read-only access to system/configuration paths and read-write access
only to the project directory. Sandboxing is implemented later, not now; until then the
harness runs YOLO. Capability additions (file tools) and the sandbox are deferred
together.

## Consequences

- The human decides only what to ask; per-tool confirmation is not in the critical path.
- Until the sandbox lands, the harness runs with effectively unlimited host access.
- Per-tool-execution y/N confirmation was rejected — intrusive, tuned out.
- A constrained staged-edit tool set was deferred — more machinery than needed now.
