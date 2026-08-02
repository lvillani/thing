# Self-evolution is human-gated and secured by OS sandboxing, not per-call confirmation

The harness improves itself by exercising real tools (bash now; read/write/edit/glob
later) on its own source, and we deliberately do *not* ask the human to confirm each
tool call as it runs — per-command confirmation is tuned out in practice and breaks
flow. The agent holds effectively unlimited host access inside the core's tool loop
(`bash` can write any file). The trust model is therefore: the human decides what to ask
(the question-level gate), and the *real* technical constraint is left to OS sandboxing
primitives — Seatbelt on macOS and Landlock on Linux — pinned to give the agent
read-only access to system/configuration paths and read-write access only to the project
directory. Sandboxing is implemented later, not now; until then the harness runs YOLO.
Capability additions (file tools) and the sandbox are deferred together.

*Status: accepted*

*Considered Options:* per-tool-execution y/N confirmation (rejected — intrusive, tuned
out); constrained staged-edit tool set (deferred — more machinery than needed now).
