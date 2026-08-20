// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

func TestStoreKeyForCityIncludesState(t *testing.T) {
	or := storeKeyFor("road bike", &offerup.Location{City: "Portland", State: "OR"})
	me := storeKeyFor("road bike", &offerup.Location{City: "Portland", State: "ME"})
	if or == me {
		t.Fatalf("same-name cities in different states must not share a store key: %q == %q", or, me)
	}

	// City without a state stays its own bucket, distinct from the stateful keys.
	noState := storeKeyFor("road bike", &offerup.Location{City: "Portland"})
	if noState == or || noState == me {
		t.Fatalf("city-without-state key %q must differ from stateful keys %q / %q", noState, or, me)
	}

	// Same city+state is deterministic and case/whitespace-insensitive.
	if got := storeKeyFor("road bike", &offerup.Location{City: " portland ", State: "or"}); got != or {
		t.Fatalf("store key must normalize case/whitespace: %q != %q", got, or)
	}

	// Zip and geo keys take precedence and are unaffected by city/state.
	if got := storeKeyFor("road bike", &offerup.Location{Zip: "97201", City: "Portland", State: "OR"}); got != "road bike@zip:97201" {
		t.Fatalf("zip must take precedence over city/state: got %q", got)
	}
	if got := storeKeyFor("road bike", &offerup.Location{Lat: "45.5", Lon: "-122.6", City: "Portland", State: "OR"}); got != "road bike@geo:45.5,-122.6" {
		t.Fatalf("geo must take precedence over city/state: got %q", got)
	}
}
