// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

// compiledRule pairs an entity rule's match pattern, compiled once, with its
// axis assignments so the regexp is not recompiled per source value in the
// classify inner loop.
type compiledRule struct {
	re  *regexp.Regexp
	set map[string]string
}

// compileRules compiles each rule's match pattern exactly once for reuse across
// every source value. A rule whose Match does not compile is skipped with a
// warning written to warn (never panics). Config-load validation
// (normalizecfg.validate) already rejects an uncompilable rule, so a skip here
// is a defense-in-depth diagnostic for any rule that reaches classify without
// passing through Parse — it makes a silently-disabled rule visible instead of
// matching nothing with no signal. Pass entityType for the warning context; a
// nil warn writer silences the diagnostic.
func compileRules(entityType string, rules []normalizecfg.Rule, warn io.Writer) []compiledRule {
	out := make([]compiledRule, 0, len(rules))
	for i, r := range rules {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			if warn != nil {
				fmt.Fprintf(warn, "warning: %s rule %d: skipping uncompilable match %q: %v\n", entityType, i, r.Match, err)
			}
			continue
		}
		out = append(out, compiledRule{re: re, set: r.Set})
	}
	return out
}

// applyCompiledRules runs an ordered set of precompiled match→set rules over an
// already-canonicalized value and returns the merged axis assignments. Rules
// are applied in order; a later matching rule overrides an earlier one for the
// same key. The returned map may be empty when no rule matches.
func applyCompiledRules(canon string, rules []compiledRule) map[string]string {
	out := map[string]string{}
	for _, r := range rules {
		if r.re.MatchString(canon) {
			for k, v := range r.set {
				out[k] = v
			}
		}
	}
	return out
}

// applyAttributeRules compiles the rules and runs them over canon in a single
// call. It is a convenience wrapper retained for callers (and tests) that do not
// hold a precompiled rule set; the classify hot path uses compileRules +
// applyCompiledRules so the patterns are compiled once per run, not per value.
// Uncompilable rules are silently skipped here (the warn path lives in
// compileRules); this preserves the original never-panics contract.
func applyAttributeRules(canon string, rules []normalizecfg.Rule) map[string]string {
	return applyCompiledRules(canon, compileRules("", rules, nil))
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
