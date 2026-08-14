// SPDX-License-Identifier: GPL-3.0-only

// Package config contains the configuration for the application.
package config

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"

	"thing/internal/model"
)

// Config represents the configuration for the application.
type Config struct {
	Model           string                `toml:"model"`
	Endpoint        string                `toml:"endpoint"`
	ReasoningEffort model.ReasoningEffort `toml:"reasoning_effort"`
}

// Load loads the configuration file from the XDG config directory.
func Load() (*Config, error) {
	configFilePath, err := xdg.ConfigFile("thing/config.toml")
	if err != nil {
		return nil, err
	}

	var config Config
	f, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := toml.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	if config.ReasoningEffort != "" && !config.ReasoningEffort.Valid() {
		return nil, fmt.Errorf("invalid reasoning_effort %q: want minimal, low, medium, high, xhigh, or max", config.ReasoningEffort)
	}

	return &config, nil
}
