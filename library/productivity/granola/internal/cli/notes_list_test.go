// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNotesListSupportsFolderFilter(t *testing.T) {
	cmd := newNotesListCmd(&rootFlags{})
	flag := cmd.Flags().Lookup("folder-id")
	if flag == nil {
		t.Fatal("notes list is missing --folder-id")
	}
	if flag.DefValue != "" {
		t.Fatalf("--folder-id default = %q, want empty", flag.DefValue)
	}
}
