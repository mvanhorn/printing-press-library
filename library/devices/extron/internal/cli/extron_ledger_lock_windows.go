// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build windows

package cli

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// withLedgerLock serializes read-modify-write access to the download ledger
// across concurrent CLI processes sharing the same --dir. Uses the Win32
// LockFileEx range lock on a lock file next to the ledger, so two Windows
// processes cannot load the same snapshot and independently replace it.
func withLedgerLock(dir string, fn func() error) error {
	lockPath := filepath.Join(dir, ".extron-downloads.lock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pathp, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(pathp,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		return err
	}
	defer windows.UnlockFileEx(h, 0, 1, 0, ol)
	return fn()
}
