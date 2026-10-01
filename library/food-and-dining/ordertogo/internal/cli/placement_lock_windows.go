// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build windows

package cli

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

var errPlacementDurabilityUnsupported = errors.New("checkout is disabled on Windows because crash-safe idempotency records cannot be confirmed; use the web checkout or a Unix CLI")

func checkPlacementDurability() error { return errPlacementDurabilityUnsupported }

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

// A successful no-op would claim a reservation was durable before payment.
func syncDir(string) error { return errPlacementDurabilityUnsupported }
