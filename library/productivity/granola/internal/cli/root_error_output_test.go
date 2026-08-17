// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// PATCH(single-error-output): cmd/granola-pp-cli/main.go prints the error and
// owns the exit code. Cobra's own error printer must stay silenced or every
// failure is emitted twice. That was cosmetic while errors were one line; the
// migrated-scheme diagnostic is a paragraph, and printing it twice buries the
// remedy. main.go is generated and marked DO NOT EDIT, so this side is the one
// that has to hold.
func TestRootCmd_SilencesCobraErrorPrinting(t *testing.T) {
	var flags rootFlags
	rootCmd := newRootCmd(&flags)

	if !rootCmd.SilenceErrors {
		t.Error("SilenceErrors is false: cobra will print the error and main.go will print it again, duplicating every failure")
	}
	if !rootCmd.SilenceUsage {
		t.Error("SilenceUsage is false: a runtime failure will dump the full usage block after the error")
	}
}
