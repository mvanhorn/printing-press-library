// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func checkPlacementDurability() error { return nil }

// lockPlacementFile takes the exclusive, non-blocking lock that serializes
// one checkout attempt per installation. LockFileEx is Windows' analogue
// of flock; FAIL_IMMEDIATELY mirrors LOCK_NB.
func lockPlacementFile(f *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped)
}

func unlockPlacementFile(f *os.File) {
	overlapped := new(windows.Overlapped)
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
}
