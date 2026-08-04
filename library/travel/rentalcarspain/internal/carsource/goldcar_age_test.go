// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

// Goldcar's edadUsu is a band code (0 = 25+, 1 = 21–24), not a literal age. All
// under-25 drivers map to band 1; standard and unspecified ages map to band 0.
func TestGoldcarAgeBand(t *testing.T) {
	cases := map[int]int{
		0:  0, // unspecified → standard
		35: 0,
		25: 0, // 25 is standard
		24: 1,
		21: 1,
		20: 1, // under-21 still band 1 (declined via MinAge, not by suppressing the quote)
		18: 1,
	}
	for age, want := range cases {
		if got := goldcarAgeBand(age); got != want {
			t.Errorf("goldcarAgeBand(%d) = %d, want %d", age, got, want)
		}
	}
}
