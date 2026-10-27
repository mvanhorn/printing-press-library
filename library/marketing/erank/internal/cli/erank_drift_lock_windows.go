//go:build windows

// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockDriftFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol)
}

func unlockDriftFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
