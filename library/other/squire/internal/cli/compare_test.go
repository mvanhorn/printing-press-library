// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored smoke test for the compare command (Phase 3).

package cli

import "testing"

func TestNovelCompareCommandConstructs(t *testing.T) {
	cmd := newNovelCompareCmd(&rootFlags{})
	if cmd == nil || cmd.Use == "" {
		t.Fatalf("compare command should construct with a non-empty Use")
	}
	if cmd.RunE == nil {
		t.Fatalf("compare command should have a RunE")
	}
}
