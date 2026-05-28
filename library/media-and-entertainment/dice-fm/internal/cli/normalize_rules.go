// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

// applyAttributeRules runs an ordered set of match→set rules over an
// already-canonicalized value and returns the merged axis assignments. Rules
// are applied in order; a later matching rule overrides an earlier one for the
// same key. A rule whose Match does not compile is skipped (never panics). The
// returned map may be empty when no rule matches.
func applyAttributeRules(canon string, rules []normalizecfg.Rule) map[string]string {
	out := map[string]string{}
	for _, r := range rules {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			continue
		}
		if re.MatchString(canon) {
			for k, v := range r.Set {
				out[k] = v
			}
		}
	}
	return out
}

// validateRule checks a candidate rule against a cache of already-classified
// names (name -> axis values). The rule passes only if it matches at least one
// cached name AND, for every cached name it matches, every (key, value) in its
// Set agrees with that name's cached axis value. Any disagreement is a false
// positive and fails the rule. A rule with an uncompilable Match fails.
//
// Empty-key contract: a Set key that is absent from a matched cached name's
// axis map resolves to "" (the Go map zero value). Consequently a Set value of
// "" passes against an absent or empty cached value, while a non-empty Set
// value correctly fails (counts as a false positive) against a cached name that
// lacks that key. A rule with an empty Set validates true for any matching name
// because it asserts nothing (assigns nothing).
func validateRule(r normalizecfg.Rule, cached map[string]map[string]string) bool {
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return false
	}
	matchedAny := false
	for name, axisValues := range cached {
		if !re.MatchString(name) {
			continue
		}
		matchedAny = true
		for k, v := range r.Set {
			if axisValues[k] != v {
				return false
			}
		}
	}
	return matchedAny
}

// parseBoolAxis parses a truthy axis token using the same token set as the
// flexBool import path: "true"/"1"/"yes" are true, everything else (including
// "false"/"0"/"no"/"") is false. Matching is case-insensitive and trims
// surrounding whitespace so config- and rule-derived axis values agree with
// imported values.
func parseBoolAxis(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// promoteRules returns the subset of candidate rules that pass validateRule
// against the cached classifications. Rules with false positives or zero
// matches are dropped.
//
// validateRule/promoteRules are the auto-validation primitives for the
// agent-driven rule-promotion loop. They are intentionally not yet wired into a
// command in Phase 1: a future promotion driver will feed cached classifications
// to promoteRules and persist the survivors as an entity's promoted rules. The
// functions ship now (with their tests) so the driver can be added without
// re-deriving the validation contract; the currently-unused export is deliberate
// scope, not an oversight.
func promoteRules(candidates []normalizecfg.Rule, cached map[string]map[string]string) []normalizecfg.Rule {
	var out []normalizecfg.Rule
	for _, c := range candidates {
		if validateRule(c, cached) {
			out = append(out, c)
		}
	}
	return out
}
