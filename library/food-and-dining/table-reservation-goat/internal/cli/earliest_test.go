// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
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

// TestSummarizeEarliest covers issue #406 failure 4: zero-resolution
// requests previously rendered as `{}` (via the --select path), making
// "couldn't resolve any input" look identical to "checked, no slots."
// The new meta envelope and unresolved[] always carry the distinction.
func TestSummarizeEarliest(t *testing.T) {
	cases := []struct {
		name                string
		venues              []string
		rows                []earliestRow
		wantRequested       int
		wantResolved        int
		wantUnresolved      int
		wantAvailable       int
		wantUnresolvedNames []string
	}{
		{
			name:   "all resolved, none available",
			venues: []string{"canlis", "spinasse"},
			rows: []earliestRow{
				{Venue: "canlis", Network: "tock", Available: false, Reason: "tock canlis: no open slots for party=2"},
				{Venue: "spinasse", Network: "opentable", Available: false, Reason: "opentable spinasse: no open slots in 14-day window for party=2"},
			},
			wantRequested: 2, wantResolved: 2, wantUnresolved: 0, wantAvailable: 0,
		},
		{
			name:   "all resolved, all available",
			venues: []string{"canlis", "alinea"},
			rows: []earliestRow{
				{Venue: "canlis", Network: "tock", Available: true, SlotAt: "2026-05-15T17:00"},
				{Venue: "alinea", Network: "tock", Available: true, SlotAt: "2026-05-15T19:00"},
			},
			wantRequested: 2, wantResolved: 2, wantUnresolved: 0, wantAvailable: 2,
		},
		{
			name:   "all unresolved",
			venues: []string{"daniels-broiler-bellevue", "joey-bellevue"},
			rows: []earliestRow{
				{Venue: "daniels-broiler-bellevue", Network: "unknown", Available: false, Reason: "could not resolve venue on OpenTable or Tock"},
				{Venue: "joey-bellevue", Network: "", Available: false, Reason: "auth error"},
			},
			wantRequested: 2, wantResolved: 0, wantUnresolved: 2, wantAvailable: 0,
			wantUnresolvedNames: []string{"daniels-broiler-bellevue", "joey-bellevue"},
		},
		{
			name:   "mixed: some resolve, some don't, one has slots",
			venues: []string{"canlis", "fake-venue", "spinasse"},
			rows: []earliestRow{
				{Venue: "canlis", Network: "tock", Available: true, SlotAt: "..."},
				{Venue: "fake-venue", Network: "unknown", Reason: "could not resolve"},
				{Venue: "spinasse", Network: "opentable", Available: false, Reason: "no slots"},
			},
			wantRequested: 3, wantResolved: 2, wantUnresolved: 1, wantAvailable: 1,
			wantUnresolvedNames: []string{"fake-venue"},
		},
		{
			name:          "empty input",
			venues:        []string{},
			rows:          []earliestRow{},
			wantRequested: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, results, unresolved := summarizeEarliest(tc.venues, tc.rows)
			if meta.VenuesRequested != tc.wantRequested {
				t.Errorf("VenuesRequested = %d; want %d", meta.VenuesRequested, tc.wantRequested)
			}
			if meta.Resolved != tc.wantResolved {
				t.Errorf("Resolved = %d; want %d", meta.Resolved, tc.wantResolved)
			}
			if meta.Unresolved != tc.wantUnresolved {
				t.Errorf("Unresolved = %d; want %d", meta.Unresolved, tc.wantUnresolved)
			}
			if meta.Available != tc.wantAvailable {
				t.Errorf("Available = %d; want %d", meta.Available, tc.wantAvailable)
			}
			if len(unresolved) != len(tc.wantUnresolvedNames) {
				t.Errorf("unresolved len = %d (%v); want %d (%v)", len(unresolved), unresolved, len(tc.wantUnresolvedNames), tc.wantUnresolvedNames)
			}
			for i, name := range tc.wantUnresolvedNames {
				if i >= len(unresolved) {
					break
				}
				if unresolved[i].Venue != name {
					t.Errorf("unresolved[%d].Venue = %q; want %q", i, unresolved[i].Venue, name)
				}
			}
			// PR #424 round-2 Greptile finding: unresolved venues must
			// NOT appear in both results[] and unresolved[]. Verify the
			// partition is disjoint.
			if len(results) != tc.wantResolved {
				t.Errorf("results len = %d; want %d (must equal Resolved count)", len(results), tc.wantResolved)
			}
			for _, r := range results {
				if r.Network == "" || r.Network == "unknown" {
					t.Errorf("results[] leaked unresolved venue %q (Network=%q) — partition broken", r.Venue, r.Network)
				}
			}
			unresolvedSet := map[string]bool{}
			for _, u := range unresolved {
				unresolvedSet[u.Venue] = true
			}
			for _, r := range results {
				if unresolvedSet[r.Venue] {
					t.Errorf("venue %q appears in BOTH results[] and unresolved[] — duplication bug", r.Venue)
				}
			}
		})
	}
}

// TestEarliestResponse_JSONShapeContractsMeta verifies that the meta
// envelope is ALWAYS present in JSON output, even when results is empty.
// The user's original symptom was `--select results.X` returning `{}` on
// zero-resolution — the new shape makes meta.* available at the top level
// so agents can branch on it even when results is filtered out.
func TestEarliestResponse_JSONShapeContractsMeta(t *testing.T) {
	resp := earliestResponse{
		Venues:     []string{"x"},
		Party:      2,
		Within:     1,
		Meta:       earliestMeta{VenuesRequested: 1, Resolved: 0, Unresolved: 1, Available: 0},
		Results:    []earliestRow{},
		Unresolved: []unresolvedRow{{Venue: "x", Reason: "could not resolve"}},
		QueriedAt:  "2026-05-10T12:00:00Z",
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		`"meta":`, `"venues_requested":1`, `"resolved":0`, `"unresolved":1`, `"available":0`,
		`"results":[]`, `"unresolved":[`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("JSON missing %q; got %s", want, body)
		}
	}
}

// TestEarliestResponse_UnresolvedEmittedWhenEmpty pins TWO contracts
// the meta envelope's promise to agents relies on:
//
//  1. JSON marshaling of a nil slice produces `"unresolved":null` (Go
//     semantics, baseline we explicitly DON'T want).
//  2. The package contract is: `summarizeEarliest` initializes the
//     slice to `[]unresolvedRow{}` so the JSON contains
//     `"unresolved":[]`. Agents calling iterate-without-nil-checks
//     depend on this contract.
//
// Greptile P2 round-2 (PR #424): the prior shape of this test had a
// misleading comment ("explicitly nil — must still serialize as `[]`")
// alongside a weak `null || []` assertion. The reality is that a bare
// nil slice marshals to `null`, NOT `[]` — so the assertion was
// vacuously true and the comment claimed a guarantee the test didn't
// enforce.
//
// The fix: assert each contract separately, with the right expectation.
func TestEarliestResponse_UnresolvedEmittedWhenEmpty(t *testing.T) {
	// Case 1: a nil slice marshals to "null". This is Go's default
	// behavior; we explicitly DOCUMENT that we don't rely on it.
	respNil := earliestResponse{
		Venues:     []string{"canlis"},
		Party:      2,
		Within:     1,
		Meta:       earliestMeta{VenuesRequested: 1, Resolved: 1, Available: 1},
		Results:    []earliestRow{{Venue: "canlis", Network: "tock", Available: true}},
		Unresolved: nil,
		QueriedAt:  "2026-05-10T12:00:00Z",
	}
	rawNil, _ := json.Marshal(respNil)
	if !strings.Contains(string(rawNil), `"unresolved":null`) {
		t.Errorf("baseline: nil slice should marshal to null; got %s", string(rawNil))
	}

	// Case 2: an explicit empty slice marshals to `[]`. This is the
	// contract `summarizeEarliest` enforces — it ALWAYS returns
	// `[]unresolvedRow{}` (never nil) so JSON consumers iterate
	// without nil-checks.
	respEmpty := respNil
	respEmpty.Unresolved = []unresolvedRow{}
	rawEmpty, _ := json.Marshal(respEmpty)
	if !strings.Contains(string(rawEmpty), `"unresolved":[]`) {
		t.Errorf("contract: empty-slice unresolved must marshal to []; got %s", string(rawEmpty))
	}
	if strings.Contains(string(rawEmpty), `"unresolved":null`) {
		t.Errorf("contract: empty-slice unresolved must NOT marshal as null; got %s", string(rawEmpty))
	}

	// Case 3: verify summarizeEarliest itself produces the `[]` shape,
	// not nil. This pins the contract end-to-end at the call site
	// agents actually depend on.
	_, _, unresolved := summarizeEarliest([]string{"x"}, []earliestRow{
		{Venue: "x", Network: "tock", Available: true},
	})
	if unresolved == nil {
		t.Error("summarizeEarliest must return a non-nil unresolved slice (empty []), not nil — agents iterate without nil-checks")
	}
}
