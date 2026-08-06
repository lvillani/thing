// SPDX-License-Identifier: GPL-3.0-only

// Package config contains the configuration for the application.
package config

import (
	"os"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

// Config represents the configuration for the application.
type Config struct {
	Model    string `toml:"model"`
	Endpoint string `toml:"endpoint"`
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

	return &config, nil
}
