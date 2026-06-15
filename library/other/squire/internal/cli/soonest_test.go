// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored smoke test for the soonest command (Phase 3).

package cli

import "testing"

func TestNovelSoonestCommandConstructs(t *testing.T) {
	cmd := newNovelSoonestCmd(&rootFlags{})
	if cmd == nil || cmd.Use == "" {
		t.Fatalf("soonest command should construct with a non-empty Use")
	}
	if cmd.RunE == nil {
		t.Fatalf("soonest command should have a RunE")
	}
}
