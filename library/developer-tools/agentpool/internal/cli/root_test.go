// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"
)

func TestShouldPrintError(t *testing.T) {
	if ShouldPrintError(commandExitError{code: 2}) {
		t.Fatal("delegated command exits should not be printed again")
	}
	if !ShouldPrintError(errors.New("bad command")) {
		t.Fatal("cobra/root errors should be printed")
	}
}
