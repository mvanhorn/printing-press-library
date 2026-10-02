// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CanonicalPath gives aliases to the same credential file one lock and write
// target. A missing file is allowed when its parent directory already exists.
// A dangling symlink is rejected so an atomic write cannot replace the link.
func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if info, statErr := os.Lstat(abs); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("credential path is a dangling symlink")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(abs)), nil
}

// WritePrivateFile keeps concurrent readers from seeing a partially written
// credential or session file while another process rotates an OAuth token.
func WritePrivateFile(path string, data []byte) error {
	target, err := CanonicalPath(path)
	if err != nil {
		return fmt.Errorf("resolving credential file: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".config-*.tmp")
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
	if err := os.Rename(file.Name(), target); err != nil {
		return fmt.Errorf("replacing credential file: %w", err)
	}
	return nil
}
