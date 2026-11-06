// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/cli"
)

func TestLinkCreationExcludedFromReadOnlyMirrorSelection(t *testing.T) {
	// Endpoint mirrors are currently hidden; test the selection predicate on
	// the actual Cobra commands so a future mirror mode keeps the same guard.
	for _, path := range [][]string{{"videos", "preview"}, {"videos", "live-preview", "get-link"}, {"videos", "recording-link", "get"}} {
		cmd, remaining, err := cli.RootCmd().Find(path)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("command %v missing: %v", path, err)
		}
		if mirrorAllowedWhenMCPReadOnly(cmd) {
			t.Fatalf("link-creating command %v selected as read-only", path)
		}
	}
}
