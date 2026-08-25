// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"syscall"
)

// withLedgerLock serializes read-modify-write access to the download ledger
// across concurrent CLI processes sharing the same --dir. A lock file next to
// the ledger is flock-exclusive for the duration of the operation.
func withLedgerLock(dir string, fn func() error) error {
	lockPath := filepath.Join(dir, ".extron-downloads.lock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
