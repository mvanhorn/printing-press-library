//go:build !windows

// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"os"
	"syscall"
)

func tryLockPostmarkSync(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errPostmarkSyncLockBusy
	}
	return err
}

func unlockPostmarkSync(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
