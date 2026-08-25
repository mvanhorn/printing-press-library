// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package seeds

import "github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"

// Generic is intentionally an empty slice today. The kinds typically
// expected here (lowercase, uppercase, kebab-case, capitalize-first,
// slug) are *computed* kinds — see lookups.computedLookup in the
// parent package. They are not table-backed because the transform is
// the same for any input string; storing seed rows for them would be
// either wasteful (one row per word in the dictionary) or wrong (a
// pre-computed lowercase for "Apple" is correct, but "apple" should
// also resolve, and an explicit row for every casing variant would
// be enumerating an infinite set).
//
// Why this file ships empty rather than being deleted:
//
//   - Symmetry with countries.go and sports.go makes the package
//     shape obvious to the next agent (one file per domain).
//
//   - A future plan that introduces table-backed generic mappings
//     (e.g., a "currency_iso4217" kind with explicit rows for "USD",
//     "EUR", ...) has a natural home here without restructuring.
//
//   - The init.go Seeds() concatenation is order-stable; keeping
//     this file in the chain avoids reordering when future entries
//     land.
var Generic = []lookups.LookupRow{
	// Reserved for future table-backed generic mappings. Computed
	// kinds (lowercase, uppercase, kebab-case, capitalize-first, slug)
	// resolve via ../store.go::computedLookup with no DB lookup.
}
