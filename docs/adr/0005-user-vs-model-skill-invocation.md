# User-explicit skill invocation is a TUI concern; activation lives in the core

- Status: accepted

## Context and Problem Statement

Skills can be invoked two ways: model-driven (the model reads the catalog and loads a
skill) and user-explicit (the user issues a `/skill:<name>` command). These are two
distinct concepts: _invocation_ is the trigger, _activation_ is the loading. We follow
the agentskills.io spec in splitting them. (See the glossary for the precise
definitions.)

## Decision

### Boundary

Recognizing `/skill:<name> task` and splitting name from task is the job of the
interaction surface (the TUI), not the core loop. The core must not learn keyboard
shorthand. The activation operation (resolving the name against the registry and
producing the instructions to deliver) belongs in the core, where the skill registry
lives and from which any future frontend can reuse it.

### Contract

For now, user-explicit invocation is kept deliberately minimal, as a macro: it resolves
the skill and returns a short pointer message that nudges the model to read that skill's
`SKILL.md`, then passes the remaining task through.

- The core exposes `Agent.ActivateSkill(name, task) (string, error)`. It returns a
  pointer string and never decides whether the input is a command—that is the TUI's job.
- `Agent.Run(ctx, string)` is unchanged. Activation returns a plain string, so there is
  no second run entry point.
- The TUI parses `/skill:`. On a match it calls `ActivateSkill`, replaces the string it
  sends to the model with the resolved pointer, and on error prints the error into the
  scrollback without starting a run.
- Echo and history show the raw `/skill:` command the user typed; only the model sees
  the resolution.

### Autocomplete

To make user-explicit invocation discoverable, we ship an autocomplete widget now. As
the user types `/skill:`, an inline popup lists matching skill names alongside the input
(reusing the `@` file-mention popup's visual and key handling). The skill popup is a
separate, parallel state machine from the file mention, not a generalized single popup—
they have different data sources (skills vs. files) and different commit behaviors. They
are mutually exclusive: typing one closes the other.

The skill popup's data comes from the core: `Agent` exposes a read-only accessor
(`Agent.Skills()`) that returns the already-discovered catalog (name + description). The
TUI filters and renders from that—a single source of truth.

## Consequences

- The core learns no keyboard shorthand; any future frontend reuses activation.
- Autocomplete uses the core's catalog as the single source of truth.
- Rejected: a dedicated `ActivateSkill(name, task) (Message, error)` plus a
  message-level run path. It adds a second run entry point and a Message-typed return we
  don't need yet.
- Rejected: printing the error inline in the footer. We want it visible in the
  transcript.
- Rejected: the TUI re-discovering skills from disk. It duplicates logic and drifts from
  what the core loaded.
- Rejected: exposing the registry type directly. It leaks internals.
