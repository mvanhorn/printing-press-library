// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The --home flag sets a process-wide path override (cliutil.SetHomeOverride)
// in PersistentPreRunE. This file scopes it to the invocation that set it, so
// a second command run in the same process (tests, embedders) never inherits
// a previous run's --home.

import (
	"sync"

	"github.com/spf13/cobra"
)

var homeScope struct {
	mu sync.Mutex
	// restore reinstates the override that was active before the current
	// invocation applied --home (nil when nothing is pending).
	restore func()
	// inExecute counts Execute() calls in progress. Execute restores the
	// override itself as its last step, after its own post-run work (the
	// journal entry and the run receipt land in the --home directories).
	inExecute int
}

func init() {
	// Finalizers run after every Cobra command, whether RunE failed or not.
	cobra.OnFinalize(finishHomeScope)
}

// scopeHomeOverride takes the result of cliutil.SetHomeOverride (applied for
// --home in PersistentPreRunE) and remembers how to undo it. Only the first
// override of an invocation is remembered, so the restore always returns to
// the value from before the invocation.
func scopeHomeOverride(restore func(), err error) error {
	if err != nil {
		return err
	}
	homeScope.mu.Lock()
	if homeScope.restore == nil {
		homeScope.restore = restore
	}
	homeScope.mu.Unlock()
	return nil
}

// finishHomeScope restores the override as soon as a directly executed
// command returns; under Execute() it leaves that to Execute's own defer.
func finishHomeScope() {
	homeScope.mu.Lock()
	deferred := homeScope.inExecute > 0
	homeScope.mu.Unlock()
	if !deferred {
		restoreHomeOverride()
	}
}

func restoreHomeOverride() {
	homeScope.mu.Lock()
	fn := homeScope.restore
	homeScope.restore = nil
	homeScope.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// executeHomeScope marks Execute() as the driver of the command tree and
// returns the function that ends the scope. Execute defers it before any
// other deferred step, so it runs last.
func executeHomeScope() func() {
	homeScope.mu.Lock()
	homeScope.inExecute++
	homeScope.mu.Unlock()
	return func() {
		homeScope.mu.Lock()
		homeScope.inExecute--
		homeScope.mu.Unlock()
		restoreHomeOverride()
	}
}
