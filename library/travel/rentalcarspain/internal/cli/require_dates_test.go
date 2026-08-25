// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// delpaso/compare take <pickup> <dropoff> with no location arg; a mistaken
// location must produce a clear usage error, not a silent empty result.
func TestRequireDateArgs(t *testing.T) {
	// Valid dates (both accepted formats) pass.
	if err := requireDateArgs("delpaso", "15/09/2026", "2026-09-22"); err != nil {
		t.Errorf("valid dates should pass, got %v", err)
	}
	// A location passed as the first arg is rejected, naming the value and command.
	err := requireDateArgs("delpaso", "AGP", "15/09/2026")
	if err == nil {
		t.Fatal("a non-date pickup should error")
	}
	if !strings.Contains(err.Error(), "AGP") || !strings.Contains(err.Error(), "delpaso") {
		t.Errorf("error should name the bad value and command, got %q", err.Error())
	}
	// It must be a usage error (exit code 2), not a runtime error.
	if code := ExitCode(err); code != 2 {
		t.Errorf("date-arg error should be a usage error (exit 2), got %d", code)
	}
	// A bad dropoff is caught too.
	if err := requireDateArgs("compare", "15/09/2026", "next-week"); err == nil {
		t.Error("a non-date dropoff should error")
	}
}
