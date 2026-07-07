package cli

import (
	"errors"
	"testing"
)

func TestExitCodePreservesChildExitCode(t *testing.T) {
	if got := ExitCode(childExitError{code: 7}); got != 7 {
		t.Fatalf("ExitCode() = %d, want 7", got)
	}
}

func TestExitCodeDefaultsToOne(t *testing.T) {
	if got := ExitCode(errors.New("boom")); got != 1 {
		t.Fatalf("ExitCode() = %d, want 1", got)
	}
}
