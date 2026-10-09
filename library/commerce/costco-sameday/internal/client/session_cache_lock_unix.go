//go:build unix

// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"os"
	"syscall"
)

// lockSessionCache takes an exclusive advisory lock on lockPath for other
// processes. flock is per-process, so sessionCacheWriteMu still serializes
// goroutines in this process. If the lock cannot be taken, the caller writes
// anyway: the file is a derived cache, and a lost merge only forces the next
// lookup.
func lockSessionCache(lockPath string) func() {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- app-owned lock beside the state file.
	if err != nil {
		return func() {}
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return func() {}
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
}
