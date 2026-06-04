// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

// applyAttributesOverlay runs an entity's promoted attribute rules over an
// already-canonicalized value and returns the resulting axis map. Axes left
// unset by the rules are simply absent from the map, which is what marks them
// as candidates for the LLM-tail classifier.
func applyAttributesOverlay(canon string, ent normalizecfg.Entity) map[string]string {
	return applyAttributeRules(canon, ent.Rules)
}

// mapVocab tests whether raw matches any member of set under Layer-A
// canonicalization. On a match it returns the canonical form of the matched set
// member and true. On no match it returns ("", false); the caller records the
// value as "(unclassified)" for the vocab entity.
//
// The set is the final merged vocabulary the caller assembled (e.g. via
// normalizecfg.Merge); mapVocab is pure and performs no store access.
func mapVocab(raw string, set []string) (canonical string, known bool) {
	canonRaw := canonicalizeName(raw)
	for _, member := range set {
		if canonicalizeName(member) == canonRaw {
			return canonicalizeName(member), true
		}
	}
	return "", false
}
