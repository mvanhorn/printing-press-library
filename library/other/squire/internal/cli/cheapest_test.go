// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored smoke test for the cheapest command (Phase 3).

package cli

import "testing"

func TestNovelCheapestCommandConstructs(t *testing.T) {
	cmd := newNovelCheapestCmd(&rootFlags{})
	if cmd == nil || cmd.Use == "" {
		t.Fatalf("cheapest command should construct with a non-empty Use")
	}
	if cmd.RunE == nil {
		t.Fatalf("cheapest command should have a RunE")
	}
}
