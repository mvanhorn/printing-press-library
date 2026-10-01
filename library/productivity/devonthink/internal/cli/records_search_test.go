// Copyright 2026 rowdy and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRecordsSearchStrategyKeepsSmartGroupLive(t *testing.T) {
	if got := recordsSearchStrategy("Project Scope"); got != "live" {
		t.Fatalf("strategy = %q, want live", got)
	}
	if got := recordsSearchStrategy(""); got != "auto" {
		t.Fatalf("unscoped strategy = %q, want auto", got)
	}
}
