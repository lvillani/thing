// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func TestLoad_DefaultsConnectionTimeout(t *testing.T) {
	configHome := t.TempDir()
	setConfigHome(t, configHome)
	writeConfig(t, configHome, "model = \"m\"\nendpoint = \"https://example.com\"\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ConnectionTimeout != DefaultConnectionTimeoutSeconds {
		t.Errorf("ConnectionTimeout = %d, want %d", cfg.ConnectionTimeout, DefaultConnectionTimeoutSeconds)
	}
}

func TestLoad_PreservesConnectionTimeout(t *testing.T) {
	configHome := t.TempDir()
	setConfigHome(t, configHome)
	writeConfig(t, configHome, "connection_timeout = 42\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ConnectionTimeout != 42 {
		t.Errorf("ConnectionTimeout = %d, want 42", cfg.ConnectionTimeout)
	}
}

func TestLoad_RejectsNegativeConnectionTimeout(t *testing.T) {
	configHome := t.TempDir()
	setConfigHome(t, configHome)
	writeConfig(t, configHome, "connection_timeout = -1\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want negative timeout error")
	}
}

func setConfigHome(t *testing.T, configHome string) {
	t.Helper()

	oldConfigHome, wasSet := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", configHome); err != nil {
		t.Fatal(err)
	}
	xdg.Reload()
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
		xdg.Reload()
	})
}

func writeConfig(t *testing.T, configHome, contents string) {
	t.Helper()

	path := filepath.Join(configHome, "thing", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
