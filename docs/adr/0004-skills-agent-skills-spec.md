# Skills implement the Agent Skills spec, loaded minimally via progressive disclosure

- Status: accepted

## Context and Problem Statement

Skills are a first-class core concept alongside Tools: a reusable bundle of knowledge
plus optional bundled resources, loaded progressively (catalog first, body on
activation, resources on demand). A dedicated activation tool would duplicate what
`bash` already gives us, and we ship no file-read tools yet.

## Decision

A Skill is a directory with a `SKILL.md` (YAML frontmatter + markdown body) that
conforms to the Agent Skills specification (https://agentskills.io), plus optional
`scripts/`, `references/`, and `assets/`. To stay minimal, the core only does tier-1
(discover + catalog) itself: a `SkillRegistry` scans the project (`.agents/skills/`) and
user (`~/.agents/skills/`) scopes. Project overrides user on a name collision. The
catalog (name + description + location) is injected into the system prompt. Activation
is the model reusing the existing `bash` tool to read `SKILL.md`—not a dedicated
`activate_skill` tool—because bash already grants full file access and we deferred the
read-tool set. We deliberately skip trust gating: loading skill instructions is just
text (no more risk than the model reading any file via bash), and executing bundled
scripts inherits ADR-0003's trust model.

## Consequences

- Minimal machinery for tier-1; activation rides on `bash` with no new tool.
- Deferred: slash-command activation, tier-3 resource enumeration, `allowed-tools`
  enforcement, `skills-ref` validation, repo-trust gating.
- Rejected: a dedicated `activate_skill` tool. It is redundant with bash-read given we
  ship no file-read tools and want minimalism.
