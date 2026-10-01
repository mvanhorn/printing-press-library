// Copyright 2026 jrimmer and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"strings"
	"testing"
)

func TestRepoDeleteRequiresYesWhenNonInteractive(t *testing.T) {
	root := RootCmd()
	root.SetArgs([]string{"repo", "delete", "example/project", "--no-input"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error deleting without --yes in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error %q should instruct the caller to pass --yes", err.Error())
	}
}
