// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// WritePrivateFile keeps concurrent readers from seeing a partially written
// credential or session file while another process rotates an OAuth token.
func WritePrivateFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replacing credential file: %w", err)
	}
	return nil
}
