// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestExecRequiresArguments(t *testing.T) {
	cmd := newExecCmd()

	err := cmd.RunE(cmd, nil)

	if err == nil {
		t.Fatal("expected usage error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2", got)
	}
}

func TestExecRequiresArgumentsAfterSeparator(t *testing.T) {
	cmd := newExecCmd()

	err := cmd.RunE(cmd, []string{"--"})

	if err == nil {
		t.Fatal("expected usage error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2", got)
	}
}
