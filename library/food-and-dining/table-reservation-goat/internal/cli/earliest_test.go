// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
)

// TestResolveEarliestForVenue_TockNumericRejected verifies the typed-error
// short-circuit for `tock:<digits>` — Tock venues are addressed by
// domain-name slug, never numeric ID. Issue #406 failure 2 reported
// `availability check 3688` and `opentable:3688` were both rejected; this
// PR adds OT-side acceptance and explicit Tock-side rejection so the
// agent gets a clear category error instead of running a doomed Calendar
// fetch.
func TestResolveEarliestForVenue_TockNumericRejected(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{"bare numeric on tock", "tock:3688", "Tock venues are addressed by domain-name slug"},
		{"large numeric", "tock:1183597", "domain-name slug"},
		// Small two-digit numeric — verifies the rejection isn't gated
		// on a minimum ID length. (Prior label "trailing whitespace
		// tolerated" was wrong: strconv.Atoi("42 ") errors, so the
		// rejection here is purely about the digit-shape predicate.)
		{"small numeric rejected", "tock:42", "domain-name slug"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := resolveEarliestForVenue(context.Background(), nil, tc.input, 2, "2026-05-15", 1, false)
			if row.Available {
				t.Errorf("expected Available=false for %q; got %+v", tc.input, row)
			}
			if !strings.Contains(row.Reason, tc.expect) {
				t.Errorf("reason missing expected hint %q; got %q", tc.expect, row.Reason)
			}
			if row.Network != "tock" {
				t.Errorf("Network = %q; want tock", row.Network)
			}
		})
	}
}

// TestResolveEarliestForVenue_BareNumericIsAmbiguous verifies that a bare
// numeric (no network prefix) doesn't trip the Tock rejection — bare
// numerics are still tried on OpenTable. This pinpoints that the Tock
// rejection only fires when the caller EXPLICITLY said `tock:`.
func TestResolveEarliestForVenue_BareNumericIsAmbiguous(t *testing.T) {
	// Bare "3688" with nil session — the Tock rejection must NOT fire
	// (no `tock:` prefix), and the OT path will fail at opentable.New(nil),
	// but importantly the failure must not be the Tock category error.
	row := resolveEarliestForVenue(context.Background(), nil, "3688", 2, "2026-05-15", 1, false)
	if strings.Contains(row.Reason, "Tock venues are addressed") {
		t.Errorf("bare numeric should not trigger the Tock-numeric category error; got %q", row.Reason)
	}
}
