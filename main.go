// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"thing/internal/agent"
	"thing/internal/backend"
	"thing/internal/config"
	"thing/internal/keychain"
	"thing/internal/skills"
	"thing/internal/tui"
)

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
	token, err := keychain.GetApiToken()
	if err != nil {
		fmt.Print("Please enter your API token: ")
		var token string
		fmt.Scanln(&token)
		if err := keychain.StoreApiToken(token); err != nil {
			fmt.Fprintln(os.Stderr, "error storing API token:", err)
			os.Exit(1)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	timeout := time.Duration(cfg.ConnectionTimeout) * time.Second
	a, _ := agent.NewAgent(backend.NewOpenAI(token, cfg.Endpoint, timeout), *cfg, skillsRegistry())
	t := tui.NewTui(a)
	if err := t.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
