// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

//go:build !windows

package granola

import (
	"fmt"
	"os"
	"syscall"
)

// checkOwnerOnly refuses a session file that another account can read or that
// this user does not own.
//
// Both halves matter. A group- or world-readable credential is readable by
// anyone on a shared box; a file owned by someone else is one they can rewrite,
// which would let them choose which account this CLI talks to.
func checkOwnerOnly(info os.FileInfo, path string) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s is mode %#o, want 0600", ErrSessionPermissions, path, info.Mode().Perm())
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if uint32(st.Uid) != uint32(os.Getuid()) {
		return fmt.Errorf("%w: %s is owned by uid %d, not %d", ErrSessionPermissions, path, st.Uid, os.Getuid())
	}
	return nil
}

// syncDir flushes the directory entry so the rename that published the session
// survives a crash. Best-effort: a filesystem that rejects the open still
// leaves the renamed file in place.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}
