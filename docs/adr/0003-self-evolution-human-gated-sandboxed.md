# Self-evolution is human-gated and secured by OS sandboxing, not per-call confirmation

- Status: accepted

## Context and Problem Statement

The harness improves itself by exercising real tools on its own source: bash now, and
read/write/edit/glob later. Inside the core's tool loop, the agent has effectively
unlimited host access (`bash` can write any file). Asking the human to confirm every
tool call is tuned out in practice and breaks flow.

## Decision

We do not ask the human to confirm each tool call as it runs. The trust model is: the
human decides what to ask (the question-level gate), and the real technical constraint
is left to OS sandboxing primitives (Seatbelt on macOS and Landlock on Linux). These
give the agent read-only access to system and configuration paths, and read-write access
only to the project directory. The sandbox is implemented later, not now; until then the
harness runs YOLO. Capability additions (file tools) and the sandbox are deferred
together.

## Consequences

- The human decides only what to ask; per-tool confirmation is not in the critical path.
- Until the sandbox lands, the harness runs with effectively unlimited host access.
- Rejected: per-tool-execution y/N confirmation. It is intrusive and gets tuned out.
- Deferred: a constrained staged-edit tool set. It is more machinery than we need now.
