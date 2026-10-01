// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !windows

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquirePlacementLock takes an exclusive, non-blocking advisory lock that
// serializes every checkout attempt for this CLI installation.
func acquirePlacementLock(recordPath string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(recordPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another checkout is already in progress (checkout lock held); wait for it to finish: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// syncDir fsyncs a directory so a just-renamed or just-removed record entry
// survives a crash.
func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return err
	}
	return dir.Close()
}
