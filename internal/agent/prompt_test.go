// SPDX-License-Identifier: GPL-3.0-only

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thing/internal/skills"
)

const expectedSystemPromptNoSkills = `
You are an expert assistant operating inside an agent harness.

Be succint in your responses. Use ASD-STE100 Simplified Technical English.

Your current working directory is "/home/user/project".
`

const expectedSystemPromptWithSkills = `
You are an expert assistant operating inside an agent harness.

Be succint in your responses. Use ASD-STE100 Simplified Technical English.

Your current working directory is "/home/user/project".

The following skills provide specialized instructions for specific tasks. When a task
matches a skill's description, use the "read" tool to load the SKILL.md at the listed
location before proceeding. When a skill references relative paths, resolve them against
the skill's directory (the parent of SKILL.md) and use absolute paths in tool calls.

<available_skills>
  <skill>
    <name>test</name>
    <description>This is a test skill.</description>
    <location>@@TEMPDIR@@/test/SKILL.md</location>
  </skill>
</available_skills>
`

const testSkill = `
---
name: test
description: This is a test skill.
---

Test
`

const testSkillDisableModelInvocation = `
---
name: test-disable-model-invocation
description: This is a test skill to demonstrate the disable-model-invocation flag.
disable-model-invocation: true
---

Test
`

func TestBuildSystemPrompt_NoSkills(t *testing.T) {
	prompt, err := buildSystemPrompt("/home/user/project", nil)
	if err != nil {
		t.Fatalf("buildSystemPrompt returned unexpected error: %v", err)
	}

	if prompt != expectedSystemPromptNoSkills {
		t.Errorf("buildSystemPrompt returned unexpected prompt:\nGot:\n\"%s\"\nExpected:\n\"%s\"", prompt, expectedSystemPromptNoSkills)
	}
}

func TestBuildSystemPrompt_WithSkills(t *testing.T) {
	tempDir := t.TempDir()

	testData := [][2]string{
		{"test", testSkill},
		{"test-disable-model-invocation", testSkillDisableModelInvocation},
	}
	for _, data := range testData {
		skillDir := filepath.Join(tempDir, data[0])
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create test skill directory: %v", err)
		}
		if err := os.WriteFile(skillFile, []byte(strings.TrimSpace(data[1])), 0644); err != nil {
			t.Fatalf("failed to write test skill file: %v", err)
		}
	}

	skillsRegistry, err := skills.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create skills registry: %v", err)
	}

	prompt, err := buildSystemPrompt("/home/user/project", skillsRegistry)
	if err != nil {
		t.Fatalf("buildSystemPrompt returned unexpected error: %v", err)
	}

	expected := strings.ReplaceAll(expectedSystemPromptWithSkills, "@@TEMPDIR@@", tempDir)
	if prompt != expected {
		t.Errorf("buildSystemPrompt returned unexpected prompt:\nGot:\n\"%s\"\nExpected:\n\"%s\"", prompt, expected)
	}
}
