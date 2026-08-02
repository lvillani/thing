# Agent Harness

A minimal Go harness for learning how agentic loops work at a first-principles level.
The conversation model and agent logic are deliberately kept independent of any UI
surface and of the wire protocol used to talk to an inference API.

## Language

**Core**: The protocol- and UI-independent part of the harness: the agent loop,
conversation state, and tool registry. It must not import `net/http` or know which
endpoint or wire protocol it talks to. _Avoid_: backend, engine

**Event**: A unit of progress emitted by the core while it runs an agent loop — an
assistant step, a tool call, a tool result, or the final answer. The core writes Events
onto a channel; a UI consumes them (e.g. forwarding into a Bubble Tea program via
`Send`).

**Skill**: A reusable bundle of knowledge — a directory containing a `SKILL.md` (YAML
frontmatter + markdown body) and optionally bundled `scripts/`, `references/`, and
`assets/` — that the agent can load to perform a class of tasks. Conforms to the Agent
Skills specification (https://agentskills.io). A Skill shapes how the model acts; a Tool
is a discrete callable action. Loaded progressively: catalog first, body on activation,
resources on demand.

**Skill catalog**: The tier-1 disclosure of available Skills shown to the model at
startup — each Skill's `name` and `description` (and optionally `location`) — so the
model knows what it can load without paying for full instruction bodies up front.

**Transport boundary**: The edge of the program where the transport lives: the endpoint,
the `http.Client`, and the request/response marshalling. It implements the core's
`Model` interface, so the core never touches HTTP. The OpenAI-compatible Chat
Completions JSON vocabulary itself is shared — its DTO types live in the model package
and are imported by both the core and the transport. _Avoid_: protocol layer, API
client, network layer
