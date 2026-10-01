// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockPlacementFile takes the exclusive, non-blocking lock that serializes
// one checkout attempt per cart fingerprint. LockFileEx is Windows' analogue
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

// syncDir is a no-op on Windows: directories cannot be fsynced through a
// generic-read handle, and NTFS journals rename/delete metadata itself.
// Treating the inevitable error as fail-closed would make every checkout
// refuse on Windows, which is worse than relying on the journal.
func syncDir(string) error { return nil }
