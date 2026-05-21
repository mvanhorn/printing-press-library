// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// framerExtended holds fields that extend the generated Config without
// modifying config.go (which is marked DO NOT EDIT).
type framerExtended struct {
	ProjectURL string `toml:"project_url"`
}

// ProjectURL returns the Framer project URL needed for project-scoped API
// calls. It checks FRAMER_PROJECT_URL env var first, then falls back to the
// project_url field in the config TOML.
func ProjectURL() string {
	if v := os.Getenv("FRAMER_PROJECT_URL"); v != "" {
		return v
	}

	// Fall back to config file
	path := os.Getenv("FRAMER_CONFIG")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "framer-pp-cli", "config.toml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var ext framerExtended
	if err := toml.Unmarshal(data, &ext); err != nil {
		return ""
	}
	return ext.ProjectURL
}
