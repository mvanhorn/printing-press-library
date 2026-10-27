//go:build !windows

// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import "syscall"

// processUmask is the process umask, captured once at package init —
// before any goroutines run — because reading it requires the
// non-thread-safe set-and-restore dance (syscall.Umask has no pure
// getter). Used to publish downloaded files at the mode a plain
// os.Create would have produced (0666 &^ umask) instead of
// os.CreateTemp's private 0600, WITHOUT widening beyond the user's
// umask policy: a umask-077 user keeps 0600 for private-repo content.
var processUmask = func() int {
	u := syscall.Umask(0)
	syscall.Umask(u)
	return u
}()
