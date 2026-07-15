// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package seeds

import "github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"

// Seeds returns the concatenation of every domain seed slice, with
// the source defaulted to "seeded" for any row that doesn't carry
// one. This is the function the v4->v5 schema migration consumes.
//
// Why a function instead of a package var: tests in this package
// occasionally manipulate the returned slice (sort, filter), and a
// var would be mutated globally. The function returns a fresh slice
// per call so test isolation is cheap.
//
// Ordering: country seeds first (largest set, ~750 rows), then sports
// (~240 rows), then generic (reserved, 0 today). Order does not
// affect the resulting table content because the migration uses
// INSERT OR IGNORE on the (kind, canonical, value) primary key, but
// keeping a stable order makes diff-based reviews of the seed batch
// readable.
func Seeds() []lookups.LookupRow {
	out := make([]lookups.LookupRow, 0, len(Countries)+len(Sports)+len(Generic))
	out = append(out, Countries...)
	out = append(out, Sports...)
	out = append(out, Generic...)
	for i := range out {
		if out[i].Source == "" {
			out[i].Source = "seeded"
		}
	}
	return out
}
