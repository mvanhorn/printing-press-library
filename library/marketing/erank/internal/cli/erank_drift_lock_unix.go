//go:build !windows

// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"syscall"
)

func lockDriftFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockDriftFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
