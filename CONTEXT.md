# Agent Harness

A minimal Go harness for learning how agentic loops work at a first-principles level.
The conversation model and agent logic are deliberately kept independent of any UI
surface and of the wire protocol used to talk to an inference API.

## Language

**Core**: The part of the harness that is independent of protocol and UI: the agent
loop, conversation state, and tool registry. It must not import `net/http` or know which
endpoint or wire protocol it talks to. _Avoid_: backend, engine

**Event**: A unit of progress the core emits while it runs an agent loop: an assistant
step, a tool call, a tool result, or the final answer. The core writes Events onto a
channel; a UI consumes them (for example, forwarding into a Bubble Tea program via
`Send`).

**Skill**: A reusable bundle of knowledge: a directory containing a `SKILL.md` (YAML
frontmatter + markdown body) and optionally bundled `scripts/`, `references/`, and
`assets/` that the agent can load to perform a class of tasks. Conforms to the Agent
Skills specification (https://agentskills.io). A Skill shapes how the model acts; a Tool
is a discrete callable action. Loaded progressively: catalog first, body on activation,
resources on demand.

**Skill catalog**: The tier-1 disclosure of available Skills shown to the model at
startup - each Skill's `name` and `description` (and optionally `location`) - so the
model knows what it can load without paying for the full instruction bodies up front.

**Skill invocation**: The trigger that decides a Skill should be loaded: either the
model acting on the catalog (model-driven invocation) or the user issuing a command
(user-explicit invocation). Distinct from activation: invocation is the decision,
activation is the loading. _Avoid_: activating a skill when you mean invoking it

**Skill activation**: The tier-2 operation that delivers a Skill's full instructions
into the conversation context: resolving the Skill by name, reading/injecting its
`SKILL.md`, and nudging the model to follow it. In this harness, activation currently
injects a pointer message rather than the body, because the model already has a `bash`
tool that can read `SKILL.md`. A Skill's `disable-model-invocation` frontmatter field
creates Skills that can only be activated via user-explicit invocation.

**Model-driven invocation**: Skill invocation triggered by the model, which reads the
catalog and decides a Skill is relevant, then activates it (for example via `bash`).
Opposed to user-explicit invocation.

**User-explicit invocation**: Skill invocation triggered by the user, not the model: a
`/skill:<name>` style command the harness intercepts, resolves, and activates directly.
This is the path that keeps a Skill usable when `disable-model-invocation` hides it from
the catalog.

**Transport boundary**: The edge of the program where the transport lives: the endpoint,
the `http.Client`, and the request/response marshalling. It implements the core's
`Model` interface, so the core never touches HTTP. The OpenAI-compatible Chat
Completions JSON vocabulary itself is shared: its DTO types live in the `internal/model`
package and are imported by both the core and the transport. _Avoid_: protocol layer,
API client, network layer
