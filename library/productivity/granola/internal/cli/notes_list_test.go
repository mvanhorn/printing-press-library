// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
)

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

func TestNotesListLocalFallbackRejectsFolderScope(t *testing.T) {
	_, _, err := resolveLocal(context.Background(), "notes", true, "/v1/notes", map[string]string{
		"folder_id": "fol_4y6LduVdwSKC27",
	}, "user_requested")
	if err == nil {
		t.Fatal("folder-scoped local note listing unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "requires the live API") {
		t.Fatalf("unexpected error: %v", err)
	}
}
