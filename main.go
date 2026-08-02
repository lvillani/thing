// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"thing/internal/agent"
	"thing/internal/backend"
	"thing/internal/skills"
	"thing/internal/tui"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"
const modelName = "deepseek/deepseek-v4-flash-0731"

// skillsRegistry builds the skill registry from the user-level and project-level skill
// locations, with the project overriding the user on a name collision. It returns nil
// when no skills should be loaded.
func skillsRegistry() *skills.Registry {
	home, _ := os.UserHomeDir()
	var roots []string
	if home != "" {
		roots = append(roots, filepath.Join(home, ".agents", "skills"))
	}
	roots = append(roots, ".agents/skills")

	reg, err := skills.New(roots...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skills:", err)
		return nil
	}
	return reg
}

func main() {
	token := os.Getenv("OPENROUTER_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_TOKEN not set")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	a := agent.NewAgent(backend.NewOpenAI(token, endpoint, client), modelName, skillsRegistry())

	app := tui.New(a)
	if _, err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	// The footer stays on the final rendered lines; emit a fresh line so the shell
	// prompt isn't appended right after it.
	fmt.Println()
}
