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
		query := buildConnectionQuery(cs)
		for _, m := range relayConnectionRe.FindAllStringSubmatch(query, -1) {
			field, args := m[1], m[2]
			if !strings.Contains(args, "first") && !strings.Contains(args, "last") {
				t.Errorf("connection query %q: field %q opens a Relay connection without a first/last pagination arg (args %q); DICE will reject it", name, field, args)
			}
		}
	}
}
