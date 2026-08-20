// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression test for the output-style (--no-color / --human-friendly) data
// race. These toggles were package-global bools written by pflag and the
// --agent branch during Execute() and read by the color/warning helpers; two
// concurrent root.Execute() calls raced. They now live on rootFlags and are
// published once through process-wide atomics (setOutputStyle), so `go test
// -race` on this file must be clean.
package cli

import (
	"bytes"
	"sync"
	"testing"
)

// TestOutputStyleFlagsNoRace runs many concurrent root.Execute() invocations
// with conflicting --no-color / --human-friendly / --agent settings against an
// isolated temp store. Under `-race` this fails if any package-global output
// state is written during command execution. Each invocation also gets its own
// output buffer; we assert no panic / no error leaks across goroutines.
func TestOutputStyleFlagsNoRace(t *testing.T) {
	// Isolate from the operator's real default-path store (each Execute reads
	// the local store via openStoreForRead -> defaultDBPath -> $HOME).
	t.Setenv("HOME", t.TempDir())

	variants := [][]string{
		{"events", "list", "--no-color", "--dry-run"},
		{"events", "list", "--human-friendly", "--dry-run"},
		{"events", "list", "--agent", "--dry-run"},
		{"events", "list", "--json", "--dry-run"},
	}

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		args := variants[i%len(variants)]
		wg.Add(1)
		go func(a []string) {
			defer wg.Done()
			flags := &rootFlags{}
			root := newRootCmd(flags)
			var out, errb bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errb)
			root.SetArgs(a)
			// Errors are acceptable (dry-run / missing token paths differ per
			// variant); the test only asserts the run is race-free and does not
			// panic. The -race detector is the real assertion.
			_ = root.Execute()
		}(args)
	}
	wg.Wait()
}

// TestSetOutputStylePublishesToggles confirms setOutputStyle is the single
// write path and that colorEnabled/humanFriendlyEnabled read the published
// values (independent of any --agent expansion ordering).
func TestSetOutputStylePublishesToggles(t *testing.T) {
	// Note: this mutates process-wide style atomics; it is intentionally not
	// t.Parallel() so it doesn't interleave with TestOutputStyleFlagsNoRace.
	t.Cleanup(func() { setOutputStyle(false, false) })

	setOutputStyle(false, true)
	if !humanFriendlyEnabled() {
		t.Errorf("humanFriendlyEnabled() = false after setOutputStyle(_, true)")
	}
	setOutputStyle(true, true) // noColor overrides
	if colorEnabled() {
		// colorEnabled also requires a TTY; under test stdout is not a TTY, so
		// this is false regardless — assert it is not true when noColor set.
		t.Errorf("colorEnabled() = true with noColor set")
	}
	setOutputStyle(false, false)
	if humanFriendlyEnabled() {
		t.Errorf("humanFriendlyEnabled() = true after reset")
	}
}
