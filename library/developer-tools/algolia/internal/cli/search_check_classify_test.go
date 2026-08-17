// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

// TestClassifySearchCheckMisses_ExhaustiveScanReportsMissing confirms that
// an unseen expected objectID from a *complete* (uncapped) scan is a genuine
// confirmed-missing result — the original, still-correct behavior for a
// real assertion failure.
func TestClassifySearchCheckMisses_ExhaustiveScanReportsMissing(t *testing.T) {
	seen := map[string]bool{"found1": true}
	missing, inconclusive := classifySearchCheckMisses([]string{"found1", "gone1"}, seen, false)

	if want := []string{"gone1"}; !reflect.DeepEqual(missing, want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	if len(inconclusive) != 0 {
		t.Fatalf("inconclusive = %v, want empty for an exhaustive scan", inconclusive)
	}
}

// TestClassifySearchCheckMisses_CappedScanReportsInconclusive is the
// regression test for the Greptile finding: when the browse scan hits its
// batch cap before every expected objectID is located, an unseen ID must be
// reported as inconclusive (unverified), never as a confirmed "missing"
// assertion failure — the scan simply never reached the rest of the index.
func TestClassifySearchCheckMisses_CappedScanReportsInconclusive(t *testing.T) {
	seen := map[string]bool{"found1": true}
	missing, inconclusive := classifySearchCheckMisses([]string{"found1", "unverified1", "unverified2"}, seen, true)

	if len(missing) != 0 {
		t.Fatalf("missing = %v, want empty — a capped scan cannot confirm absence", missing)
	}
	if want := []string{"unverified1", "unverified2"}; !reflect.DeepEqual(inconclusive, want) {
		t.Fatalf("inconclusive = %v, want %v", inconclusive, want)
	}
}

// TestClassifySearchCheckMisses_AllFoundIsUnaffectedByCapFlag confirms the
// cap flag is irrelevant once every expected ID was actually found — no
// unseen IDs means nothing to classify either way.
func TestClassifySearchCheckMisses_AllFoundIsUnaffectedByCapFlag(t *testing.T) {
	seen := map[string]bool{"a": true, "b": true}
	for _, capped := range []bool{true, false} {
		missing, inconclusive := classifySearchCheckMisses([]string{"a", "b"}, seen, capped)
		if len(missing) != 0 || len(inconclusive) != 0 {
			t.Fatalf("capped=%v: missing=%v inconclusive=%v, want both empty", capped, missing, inconclusive)
		}
	}
}
