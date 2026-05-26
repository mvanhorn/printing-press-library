// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Regression tests for the hand-authored DICE GraphQL query layer.
package cli

import (
	"regexp"
	"strings"
	"testing"
)

// relayConnectionRe matches a GraphQL field that opens a Relay connection
// (`<field>(<args>) { edges` or `<field> { edges`). DICE rejects any connection
// lacking a first/last pagination arg with "You must either supply :first or
// :last", so every connection the query emits — including ones nested inside a
// node selection — must carry one.
var relayConnectionRe = regexp.MustCompile(`(\w+)\s*(\([^)]*\))?\s*\{\s*edges\b`)

// Every connection rendered by buildConnectionQuery, at any depth, must declare
// first/last. Guards against the genres regression where the nested
// genreType.genres connection shipped without pagination args.
func TestBuiltConnectionQueriesBoundEveryConnection(t *testing.T) {
	for name, cs := range diceConnections {
		// Both pagination directions: forward (first/after) and the latest-only
		// backward variant (last/before).
		for _, latest := range []bool{false, true} {
			query := buildConnectionQuery(cs, latest)
			for _, m := range relayConnectionRe.FindAllStringSubmatch(query, -1) {
				field, args := m[1], m[2]
				if !strings.Contains(args, "first") && !strings.Contains(args, "last") {
					t.Errorf("connection query %q (latest=%v): field %q opens a Relay connection without a first/last pagination arg (args %q); DICE will reject it", name, latest, field, args)
				}
			}
		}
	}
}

// The latest-only variant must page backward (last) so it returns the newest
// records, not page 1 (oldest) of an oldest-first connection.
func TestLatestQueryPagesBackward(t *testing.T) {
	q := buildConnectionQuery(diceConnections["orders"], true)
	if !strings.Contains(q, "last: $last") || !strings.Contains(q, "before: $before") {
		t.Errorf("latest-only orders query should page backward via last/before; got:\n%s", q)
	}
	if strings.Contains(q, "first: $first") {
		t.Errorf("latest-only query must not use forward first/after pagination; got:\n%s", q)
	}
}
