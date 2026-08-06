// SPDX-License-Identifier: GPL-3.0-only

package agent

import (
	"html/template"
	"strings"
	"thing/internal/skills"
)

// developerPromptTemplate is the template of the initial prompt used to steer model
// behavior.
const developerPromptTemplate = `
You are an expert assistant operating inside an agent harness.

Be succint in your responses. Use ASD-STE100 Simplified Technical English (STE).

Your current working directory is "{{.cwd}}".
{{- if .skillsCatalog}}

The following skills provide specialized instructions for specific tasks. When a task
matches a skill's description, use your file-read tool to load the SKILL.md at the
listed location before proceeding. When a skill references relative paths, resolve
them against the skill's directory (the parent of SKILL.md) and use absolute paths in
tool calls.

<available_skills>
  {{- range .skillsCatalog}}
  <skill>
    <name>{{.Name}}</name>
    <description>{{.Description}}</description>
    <location>{{.Location}}</location>
  </skill>
  {{- end}}
</available_skills>
{{- end}}
`

// buildDeveloperPrompt builds the initial prompt used to steer model behavior,
// including the current working directory and the skill catalog (if any). It returns
// the prompt string or an error if template expansion fails.
func buildDeveloperPrompt(cwd string, skillsRegistry *skills.Registry) (string, error) {
	t, err := template.New("developerPrompt").Parse(developerPromptTemplate)
	if err != nil {
		return "", err
	}

	var skillsCatalog []skills.Skill
	if skillsRegistry != nil {
		skillsCatalog = skillsRegistry.ModelCatalog()
	}

	sb := &strings.Builder{}
	if err := t.Execute(sb, map[string]any{
		"cwd":           cwd,
		"skillsCatalog": skillsCatalog,
	}); err != nil {
		return "", err
	}

	return sb.String(), nil
}
