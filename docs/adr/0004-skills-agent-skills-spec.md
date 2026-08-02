# Skills implement the Agent Skills spec, loaded minimally via progressive disclosure

Skills are a first-class core concept alongside Tools: a Skill is a directory with a
`SKILL.md` (YAML frontmatter + markdown body) conforming to the Agent Skills
specification (https://agentskills.io), plus optional `scripts/`, `references/`, and
`assets/`. To stay minimal the core only does tier-1 (discover + catalog) itself: a
`SkillRegistry` scans the project (`.agents/skills/`) and user (`~/.agents/skills/`)
scopes, project overrides user on a name collision, and the catalog (name + description
+ location) is injected into the system prompt. Activation is the model reusing the
existing `bash` tool to read `SKILL.md` — not a dedicated `activate_skill` tool —
because bash already grants full file access and we deferred the read-tool set; a
dedicated tool's control benefits (frontmatter stripping, resource enumeration, an
enum-constrained name) don't justify new machinery at this stage. Trust gating is
deliberately absent: loading skill instructions is text (no more risk than the model
reading any file via bash), and executing bundled scripts inherits ADR-0003's trust
model.

*Status: accepted*

*Deferred:* slash-command activation, tier-3 resource enumeration, `allowed-tools`
enforcement, `skills-ref` validation, repo-trust gating.

*Considered Options:* dedicated `activate_skill` tool (rejected — redundant with
bash-read given we ship no file-read tools and want minimalism).
