// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored smoke test for the watch command (Phase 3).

package cli

import "testing"

func TestNovelWatchCommandConstructs(t *testing.T) {
	cmd := newNovelWatchCmd(&rootFlags{})
	if cmd == nil || cmd.Use == "" {
		t.Fatalf("watch command should construct with a non-empty Use")
	}
	if cmd.RunE == nil {
		t.Fatalf("watch command should have a RunE")
	}
}
