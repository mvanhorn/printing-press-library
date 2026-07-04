// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package seeds

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"
)

// TestSeedCounts is a floor check that the documented row totals are
// at or above their advertised minimums. Adding more seed entries is
// fine; the failure mode this guards against is silently losing half
// the roster in a bad refactor.
func TestSeedCounts(t *testing.T) {
	t.Parallel()
	if len(Countries) < 500 {
		t.Errorf("Countries has %d rows, want >= 500", len(Countries))
	}
	if len(Sports) < 250 {
		t.Errorf("Sports has %d rows, want >= 250", len(Sports))
	}
	t.Logf("seed totals: countries=%d sports=%d generic=%d total=%d",
		len(Countries), len(Sports), len(Generic),
		len(Countries)+len(Sports)+len(Generic))
}

// TestSeeds_NoEmptyValues guards against accidental empty fields in
// the seed rows. The migration would happily insert empty strings,
// but lookups would never resolve them.
func TestSeeds_NoEmptyValues(t *testing.T) {
	t.Parallel()
	all := Seeds()
	for i, r := range all {
		if r.Kind == "" {
			t.Errorf("Seeds[%d]: Kind empty (canonical=%q value=%q)", i, r.Canonical, r.Value)
		}
		if r.Canonical == "" {
			t.Errorf("Seeds[%d]: Canonical empty (kind=%q value=%q)", i, r.Kind, r.Value)
		}
		if r.Value == "" {
			t.Errorf("Seeds[%d]: Value empty (kind=%q canonical=%q)", i, r.Kind, r.Canonical)
		}
		if r.Source != "seeded" {
			t.Errorf("Seeds[%d]: Source = %q, want %q", i, r.Source, "seeded")
		}
	}
}

// TestSeeds_NoComputedKinds_InSeedData verifies no computed-kind row
// slipped into the seed slices. Computed-kind rows would be
// shadowed by computedLookup at Lookup time and waste a DB row.
func TestSeeds_NoComputedKinds_InSeedData(t *testing.T) {
	t.Parallel()
	for i, r := range Seeds() {
		if lookups.IsComputedKind(r.Kind) {
			t.Errorf("Seeds[%d]: kind %q is a computed kind; do not seed it (canonical=%q)",
				i, r.Kind, r.Canonical)
		}
	}
}
