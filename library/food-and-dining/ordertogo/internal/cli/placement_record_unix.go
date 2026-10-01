// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !windows

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func pendingPlacementLocation(path string) string { return path }

func readPendingPlacement(path string) ([]byte, error) { return os.ReadFile(path) }

// A temp file, file fsync, rename, and directory fsync make the new record
// durable before the checkout POST can begin.
func writePendingPlacement(path string, record pendingPlace) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-place-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("syncing record directory: %w", err)
	}
	readBack, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verifying written record: %w", err)
	}
	var verify pendingPlace
	if json.Unmarshal(readBack, &verify) != nil || verify.RequestID != record.RequestID {
		return fmt.Errorf("written record did not read back intact")
	}
	return nil
}

func clearPendingPlacement(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDir(filepath.Dir(path))
}
