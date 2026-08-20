// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"os"
	"path/filepath"
)

// ConfigDir returns ~/.config/notebooklm-pp-cli
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "notebooklm-pp-cli"), nil
}

// DataDir returns ~/.local/share/notebooklm-pp-cli
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "notebooklm-pp-cli"), nil
}

// CacheDir returns XDG cache dir for HTTP response cache files.
func CacheDir() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "notebooklm-pp-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "notebooklm-pp-cli"), nil
}
