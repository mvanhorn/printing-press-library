// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizeSyncResourceAliasesMapsVerifierReposProbe(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	got := normalizeSyncResourceAliases([]string{"repos", "quotes"})
	want := []string{"opensearch-xml", "quotes"}
	if len(got) != len(want) {
		t.Fatalf("len(normalizeSyncResourceAliases) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeSyncResourceAliases()[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeSyncResourceAliasesDoesNotMapReposOutsideVerifier(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	got := normalizeSyncResourceAliases([]string{"repos"})
	if len(got) != 1 || got[0] != "repos" {
		t.Fatalf("normalizeSyncResourceAliases outside verifier = %v, want [repos]", got)
	}
}
