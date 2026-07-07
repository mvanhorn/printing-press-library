// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored `normalize promote-rules` command: the "learn" step of the
// LLM-tail loop. It reads the method="manual" corpus (operator/agent decisions
// imported via `normalize --import`), proposes deterministic regex rules whose
// token consistently predicts an axis value, validates each candidate against
// the same corpus (promoteRules rejects any false positive), and emits the
// survivors as a normalize-config fragment. This graduates recurring manual
// classifications into reusable rules so the next `normalize` run resolves them
// deterministically instead of re-exporting them as unmatched.
// This file is NOT generated and survives `generate --force`.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// promoteToken is a candidate-rule token: a single canonicalized word that, in
// the manual corpus, co-occurs with exactly one value for a given axis.
type promoteToken struct {
	Token string
	Axis  string
	Value string
}

// tokenWordRE splits a canonicalized name into word tokens for candidate-rule
// generation. Canonicalization already lowercased and collapsed whitespace, so
// a simple word-character run is sufficient.
var tokenWordRE = regexp.MustCompile(`[a-z0-9]+`)

// buildManualAxisCache reads the method="manual" corpus for entityType and
// returns a cache keyed by the canonicalized source value, mapping each name to
// the axis values a human/agent assigned it. This is the exact shape consumed by
// validateRule/promoteRules. ticket_type axes come from the typed tier_attributes
// table; every other entity's axes come from the generic entity_attributes table.
// Only manual rows contribute, so promotion learns from operator decisions, never
// from machine-derived classifications.
func buildManualAxisCache(s *store.Store, entityType string) (map[string]map[string]string, int, error) {
	var stripRE *regexp.Regexp
	if cfg, err := loadNormalizeConfig(); err != nil {
		return nil, 0, fmt.Errorf("loading normalize config: %w", err)
	} else if ent, ok := cfg.Entities[entityType]; ok && ent.StripPattern != "" {
		stripRE, err = regexp.Compile(ent.StripPattern)
		if err != nil {
			return nil, 0, fmt.Errorf("compiling %s strip_pattern %q: %w", entityType, ent.StripPattern, err)
		}
	}

	// Manual crosswalk rows give us the (source_value → canonical_id) pairs that
	// a human/agent classified. canonical_id ties them to their axis rows.
	cw, err := s.ListCrosswalk(entityType, "dice")
	if err != nil {
		return nil, 0, fmt.Errorf("listing crosswalk for %s: %w", entityType, err)
	}
	canonNameByID := map[string]string{}
	for _, r := range cw {
		if r.Method != methodManual {
			continue
		}
		// Key the cache by the stripped+canonicalized source value — the same
		// form the rule engine matches against at classify time.
		canonNameByID[r.CanonicalID] = effectiveCanon(r.SourceValue, stripRE)
	}
	manualCrosswalkCount := len(canonNameByID)
	if len(canonNameByID) == 0 {
		return map[string]map[string]string{}, manualCrosswalkCount, nil
	}

	cache := map[string]map[string]string{}
	ensure := func(name string) map[string]string {
		if cache[name] == nil {
			cache[name] = map[string]string{}
		}
		return cache[name]
	}

	if entityType == "ticket_type" {
		tiers, err := s.ListTierAttributes(entityType)
		if err != nil {
			return nil, 0, fmt.Errorf("listing tier attributes for %s: %w", entityType, err)
		}
		for _, t := range tiers {
			if t.Method != methodManual {
				continue
			}
			name, ok := canonNameByID[t.CanonicalID]
			if !ok {
				continue
			}
			ax := ensure(name)
			setIfNonEmpty(ax, axisAccessClass, t.AccessClass)
			setIfNonEmpty(ax, axisSalesStage, t.SalesStage)
			setIfNonEmpty(ax, axisEntryWindowType, t.EntryWindowType)
			setIfNonEmpty(ax, axisEntryWindowTime, t.EntryWindowTime)
			if t.GroupSize > 0 {
				ax[axisGroupSize] = fmt.Sprintf("%d", t.GroupSize)
			}
			if t.CompFlag {
				ax[axisCompFlag] = "true"
			}
		}
		return cache, manualCrosswalkCount, nil
	}

	// Generic entity: axes live in entity_attributes (one row per key).
	attrs, err := s.ListEntityAttributes(entityType)
	if err != nil {
		return nil, 0, fmt.Errorf("listing entity attributes for %s: %w", entityType, err)
	}
	for _, a := range attrs {
		if a.Method != methodManual {
			continue
		}
		name, ok := canonNameByID[a.CanonicalID]
		if !ok {
			continue
		}
		setIfNonEmpty(ensure(name), a.AttrKey, a.AttrValue)
	}
	return cache, manualCrosswalkCount, nil
}

// setIfNonEmpty assigns m[k]=v only when v is non-empty, so empty axes do not
// pollute the cache (and so a candidate rule is never proposed for an empty
// value, which would be a no-op assignment).
func setIfNonEmpty(m map[string]string, k, v string) {
	if v != "" {
		m[k] = v
	}
}

// proposeCandidateRules derives candidate rules from the manual axis cache. For
// each (axis, value) pair, it finds tokens that, across every cached name
// CONTAINING the token, co-occur with exactly that one value for that axis (no
// conflicting assignment). Each surviving token becomes a candidate rule
// matching a word-boundary occurrence of the token and setting the axis value.
// Single-character and purely-numeric tokens are skipped as too generic. The
// result is deterministically ordered (by axis, value, token).
func proposeCandidateRules(cache map[string]map[string]string, minSupport int) []normalizecfg.Rule {
	if minSupport < 1 {
		minSupport = 1
	}
	// For each axis, for each token, collect the set of values it co-occurs with.
	// token → axis → set(values).
	type axisVals map[string]map[string]bool
	tokenAxisVals := map[string]axisVals{}
	tokenSupport := map[string]int{}

	for name, axes := range cache {
		seen := map[string]bool{}
		for _, tok := range tokenWordRE.FindAllString(name, -1) {
			if len(tok) < 2 || isAllDigits(tok) {
				continue
			}
			if seen[tok] {
				continue
			}
			seen[tok] = true
			tokenSupport[tok]++
			for axis, val := range axes {
				if val == "" {
					continue
				}
				if tokenAxisVals[tok] == nil {
					tokenAxisVals[tok] = axisVals{}
				}
				if tokenAxisVals[tok][axis] == nil {
					tokenAxisVals[tok][axis] = map[string]bool{}
				}
				tokenAxisVals[tok][axis][val] = true
			}
		}
	}

	var tokens []promoteToken
	for tok, av := range tokenAxisVals {
		if tokenSupport[tok] < minSupport {
			continue
		}
		for axis, vals := range av {
			// A token is a clean predictor only when it co-occurs with exactly
			// one value for the axis. Multiple values → ambiguous → drop.
			if len(vals) != 1 {
				continue
			}
			for v := range vals {
				tokens = append(tokens, promoteToken{Token: tok, Axis: axis, Value: v})
			}
		}
	}

	// Deterministic ordering so the emitted config (and tests) are stable.
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].Axis != tokens[j].Axis {
			return tokens[i].Axis < tokens[j].Axis
		}
		if tokens[i].Value != tokens[j].Value {
			return tokens[i].Value < tokens[j].Value
		}
		return tokens[i].Token < tokens[j].Token
	})

	rules := make([]normalizecfg.Rule, 0, len(tokens))
	for _, t := range tokens {
		rules = append(rules, normalizecfg.Rule{
			Match: `\b` + regexp.QuoteMeta(t.Token) + `\b`,
			Set:   map[string]string{t.Axis: t.Value},
		})
	}
	return rules
}

// isAllDigits reports whether s consists only of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// promoteRulesForEntity is the testable core of `normalize promote-rules`: it
// builds the manual axis cache, proposes candidate rules, and returns only those
// that pass validation against the same cache (promoteRules rejects any rule with
// a false positive). The returned rules are deterministically ordered.
func promoteRulesForEntity(s *store.Store, entityType string, minSupport int) ([]normalizecfg.Rule, int, int, error) {
	cache, manualCrosswalkCount, err := buildManualAxisCache(s, entityType)
	if err != nil {
		return nil, 0, 0, err
	}
	candidates := proposeCandidateRules(cache, minSupport)
	validated := promoteRules(candidates, cache)
	return validated, len(cache), manualCrosswalkCount, nil
}

// newNormalizePromoteRulesCmd returns the `normalize promote-rules` subcommand.
// Default behavior is read-only: it prints a normalize-config fragment of the
// promoted rules to stdout for the operator to review and paste into their
// config. `--write` merges the promoted rules into the entity's existing config
// (the operator config at the default path), which is the only mutating path and
// is CLI-only (skipped under verify, like `normalize recommend`'s default write).
func newNormalizePromoteRulesCmd(flags *rootFlags) *cobra.Command {
	var (
		entity     string
		write      bool
		minSupport int
	)
	cmd := &cobra.Command{
		Use:   "promote-rules",
		Short: "Propose deterministic rules from the method=manual corpus (LLM-tail learn step)",
		Long: "Reads the method=manual classifications for an entity (operator/agent " +
			"decisions imported via `normalize --import`), proposes regex rules whose " +
			"token consistently predicts an axis value, validates each candidate against " +
			"the same corpus (false positives are rejected), and emits the survivors as a " +
			"normalize-config fragment. This graduates recurring manual classifications " +
			"into reusable rules so subsequent `normalize` runs resolve them deterministically.\n\n" +
			"Default is read-only (prints the fragment to stdout). Use --write to merge the " +
			"promoted rules into the operator config at the default path. The entity source_value " +
			"should be an event/ticket descriptor, not free text containing personal data.",
		Example: "  dice-fm-pp-cli normalize promote-rules --entity ticket_type\n" +
			"  dice-fm-pp-cli normalize promote-rules --entity ticket_type --write",
		// No mcp:read-only: the optional --write path writes the operator
		// config file to disk, so the tool must not advertise readOnlyHint and
		// have hosts auto-approve a filesystem write.
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			if s == nil {
				// No sync/import yet — emit an empty fragment rather than error.
				return writePromotedRules(cmd, entity, nil, write)
			}
			defer s.Close()

			rules, corpusSize, manualCrosswalkCount, err := promoteRulesForEntity(s, entity, minSupport)
			if err != nil {
				return err
			}
			if corpusSize == 0 {
				if manualCrosswalkCount == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"note: no method=manual rows for %q — import classified mappings with `normalize --import` first\n", entity)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"note: found method=manual crosswalk rows for %q but no axis attributes to learn from — promote-rules needs imported axis values (tier/entity attributes)\n", entity)
				}
			}
			return writePromotedRules(cmd, entity, rules, write)
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "ticket_type", "Entity whose manual corpus to learn rules from (default ticket_type)")
	cmd.Flags().BoolVar(&write, "write", false, "Merge the promoted rules into the operator config at the default path (default: print to stdout only)")
	cmd.Flags().IntVar(&minSupport, "min-support", 2, "Minimum distinct manual source values a token must appear in before it can be promoted")
	return cmd
}

// writePromotedRules renders the promoted rules as a single-entity config
// fragment. When write is false (default) it prints the fragment to stdout. When
// write is true it merges the rules into the operator config at the default path
// (skipped under verify, which prints instead). Merge semantics mirror the
// whole-entity replace used elsewhere: the entity's rules are replaced by the
// promoted set; other fields and other entities are preserved.
func writePromotedRules(cmd *cobra.Command, entity string, rules []normalizecfg.Rule, write bool) error {
	fragment := &normalizecfg.Config{
		Version: 1,
		Entities: map[string]normalizecfg.Entity{
			entity: {
				// Carry the entity's declared source/shape from the active config
				// so the emitted fragment is a valid standalone config the
				// operator can merge or write directly.
				Rules: rules,
			},
		},
	}
	// Backfill source/shape from the active config so the fragment validates and
	// is directly usable.
	if cfg, err := loadNormalizeConfig(); err == nil {
		if ent, ok := cfg.Entities[entity]; ok {
			merged := fragment.Entities[entity]
			merged.Source = ent.Source
			merged.Shape = ent.Shape
			merged.Attributes = ent.Attributes
			merged.Vocab = ent.Vocab
			merged.StripPattern = ent.StripPattern
			fragment.Entities[entity] = merged
		}
	}

	out, err := yaml.Marshal(fragment)
	if err != nil {
		return fmt.Errorf("marshaling promoted rules: %w", err)
	}

	if !write || cliutil.IsVerifyEnv() {
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}

	// --write: merge the promoted rules into the operator config on disk.
	cfgPath := defaultConfigPath(diceCLIName)
	if err := mergePromotedRulesToConfig(cfgPath, entity, rules); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "merged %d promoted rule(s) for %q into %s\n", len(rules), entity, cfgPath)
	return nil
}

// mergePromotedRulesToConfig loads the operator config at cfgPath (or starts
// from an empty operator config when no file exists yet), replaces/adds the
// named entity's rules with the promoted set, and writes the result back. The
// whole entity is preserved except its rules, matching the whole-entity replace
// semantics of the config loader. Starter entities the operator never
// customized are not persisted.
func mergePromotedRulesToConfig(cfgPath, entity string, rules []normalizecfg.Rule) error {
	cfg := &normalizecfg.Config{Version: 1, Entities: map[string]normalizecfg.Entity{}}
	opData, err := os.ReadFile(cfgPath)
	if err == nil {
		cfg, err = normalizecfg.Parse(opData)
		if err != nil {
			return fmt.Errorf("parsing operator config %q: %w", cfgPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading operator config %q: %w", cfgPath, err)
	}

	ent, ok := cfg.Entities[entity]
	if !ok {
		active, err := loadNormalizeConfig()
		if err != nil {
			return fmt.Errorf("loading active config: %w", err)
		}
		ent, ok = active.Entities[entity]
		if !ok {
			return fmt.Errorf("entity %q is not declared in the normalize config", entity)
		}
	}
	ent.Rules = rules
	cfg.Entities[entity] = ent

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling merged config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := atomicWriteFile(cfgPath, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".normalize-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
