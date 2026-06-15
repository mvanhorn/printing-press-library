// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored smoke test for the roster command (Phase 3).

package cli

import "testing"

func TestNovelRosterCommandConstructs(t *testing.T) {
	cmd := newNovelRosterCmd(&rootFlags{})
	if cmd == nil || cmd.Use == "" {
		t.Fatalf("roster command should construct with a non-empty Use")
	}
	if cmd.RunE == nil {
		t.Fatalf("roster command should have a RunE")
	}
}
