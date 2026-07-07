// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for `normalize promote-rules` (the LLM-tail learn step). All fixtures
// are synthetic — no real tenant ticket-type names.
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// TestBuildManualAxisCacheTicketType verifies the manual corpus is read into the
// validateRule-shaped cache keyed by canonicalized source value, with axes drawn
// from the manual tier_attributes rows only.
func TestBuildManualAxisCacheTicketType(t *testing.T) {
	s := openSeededStoreForImport(t)
	csvData := "source_value,access_class,sales_stage\n" +
		"VIP Lounge,vip,early_bird\n" +
		"VIP Experience,vip,final_release\n" +
		"General Admission,ga,early_bird\n"
	if _, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv"); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	cache, manualCrosswalks, err := buildManualAxisCache(s, "ticket_type")
	if err != nil {
		t.Fatalf("buildManualAxisCache: %v", err)
	}
	if manualCrosswalks != 3 {
		t.Fatalf("want 3 manual crosswalks, got %d", manualCrosswalks)
	}
	if len(cache) != 3 {
		t.Fatalf("want 3 cached names, got %d: %+v", len(cache), cache)
	}
	// Keys are canonicalized (lowercased).
	if cache["vip lounge"]["access_class"] != "vip" {
		t.Errorf("vip lounge access_class = %q, want vip", cache["vip lounge"]["access_class"])
	}
	if cache["general admission"]["access_class"] != "ga" {
		t.Errorf("general admission access_class = %q, want ga", cache["general admission"]["access_class"])
	}
}

// TestPromoteRulesForEntity verifies the full learn step: a token that cleanly
// predicts an axis value across the manual corpus is promoted, while a token
// that co-occurs with conflicting values for the same axis is NOT promoted.
func TestPromoteRulesForEntity(t *testing.T) {
	s := openSeededStoreForImport(t)
	// "vip" cleanly predicts access_class=vip; "early" co-occurs with both
	// early_bird (good) — but here we make sales_stage ambiguous for the shared
	// token "admission" by giving the two GA rows different sales stages, so no
	// sales_stage rule should be promoted from "admission".
	csvData := "source_value,access_class,sales_stage\n" +
		"VIP Lounge,vip,early_bird\n" +
		"VIP Balcony,vip,early_bird\n" +
		"General Admission,ga,early_bird\n" +
		"Standard Admission,ga,final_release\n"
	if _, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv"); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	rules, corpus, manualCrosswalks, err := promoteRulesForEntity(s, "ticket_type", 2)
	if err != nil {
		t.Fatalf("promoteRulesForEntity: %v", err)
	}
	if corpus != 4 {
		t.Errorf("corpus size = %d, want 4", corpus)
	}
	if manualCrosswalks != 4 {
		t.Errorf("manual crosswalk count = %d, want 4", manualCrosswalks)
	}

	// Collect (axis,value) pairs that were promoted.
	byAxis := map[string]map[string]bool{}
	for _, r := range rules {
		for k, v := range r.Set {
			if byAxis[k] == nil {
				byAxis[k] = map[string]bool{}
			}
			byAxis[k][v] = true
		}
	}

	// "vip" → access_class=vip must be promoted.
	if !byAxis["access_class"]["vip"] {
		t.Errorf("expected an access_class=vip rule promoted; rules=%+v", rules)
	}
	// "admission" predicts access_class=ga cleanly (both admission rows are ga),
	// so an access_class=ga rule SHOULD be promoted.
	if !byAxis["access_class"]["ga"] {
		t.Errorf("expected an access_class=ga rule promoted; rules=%+v", rules)
	}
	// No rule should set access_class to both vip and ga from the same token —
	// validation (promoteRules) guarantees each promoted rule is a clean
	// predictor. Verify every promoted rule, applied to its own corpus, has no
	// false positive by re-running validation.
	cache, _, err := buildManualAxisCache(s, "ticket_type")
	if err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	for _, r := range rules {
		if !validateRule(r, cache) {
			t.Errorf("promoted rule failed re-validation (should be impossible): %+v", r)
		}
	}
}

// TestPromoteRulesForEntityEmptyCorpus verifies an entity with no manual rows
// yields zero promoted rules and a zero corpus size (no panic, no error).
func TestPromoteRulesForEntityEmptyCorpus(t *testing.T) {
	s := openSeededStoreForImport(t)
	rules, corpus, manualCrosswalks, err := promoteRulesForEntity(s, "ticket_type", 2)
	if err != nil {
		t.Fatalf("promoteRulesForEntity on empty store: %v", err)
	}
	if corpus != 0 || manualCrosswalks != 0 || len(rules) != 0 {
		t.Errorf("empty corpus: got %d rules / corpus %d / manual crosswalks %d, want 0/0/0", len(rules), corpus, manualCrosswalks)
	}
}

func TestPromoteRulesWarnsWhenManualCrosswalksHaveNoAxisAttributes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := defaultDBPath(diceCLIName)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	csvData := "source_value,canonical_name\n" +
		"VIP Lounge,VIP Lounge\n"
	if _, err := importMapping(s, "dice", "ticket_type", []byte(csvData), "csv"); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close seeded store: %v", err)
	}

	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"promote-rules", "--entity", "ticket_type"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote-rules: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "found method=manual crosswalk rows for \"ticket_type\" but no axis attributes to learn from") {
		t.Fatalf("missing distinct no-axis warning; stderr=%q", stderr)
	}
	if strings.Contains(stderr, "no method=manual rows") {
		t.Fatalf("used no-manual-rows warning for manual rows without axes; stderr=%q", stderr)
	}
}

// TestProposeCandidateRulesSkipsGenericTokens verifies single-character and
// purely-numeric tokens are not proposed as rules (too generic).
func TestProposeCandidateRulesSkipsGenericTokens(t *testing.T) {
	cache := map[string]map[string]string{
		"a 1 vip": {"access_class": "vip"},
		"b 2 vip": {"access_class": "vip"},
	}
	rules := proposeCandidateRules(cache, 2)
	for _, r := range rules {
		// The match wraps a token in \b...\b; the only acceptable token here is
		// "vip" (len>=2, not all-digits).
		if !strings.Contains(r.Match, "vip") {
			t.Errorf("unexpected candidate from a generic token: %q", r.Match)
		}
	}
	if len(rules) == 0 {
		t.Error("expected at least the vip rule to be proposed")
	}
}

func TestProposeCandidateRulesMinimumSupport(t *testing.T) {
	cache := map[string]map[string]string{
		"vip lounge":     {"access_class": "vip"},
		"vip experience": {"access_class": "vip"},
		"solo balcony":   {"access_class": "vip"},
	}

	defaultRules := proposeCandidateRules(cache, 2)
	if !hasRuleMatch(defaultRules, `\bvip\b`) {
		t.Fatalf("token with support 2 should be promoted: %+v", defaultRules)
	}
	if hasRuleMatch(defaultRules, `\bsolo\b`) {
		t.Fatalf("token with support 1 should not be promoted by default: %+v", defaultRules)
	}

	legacyRules := proposeCandidateRules(cache, 1)
	if !hasRuleMatch(legacyRules, `\bsolo\b`) {
		t.Fatalf("--min-support 1 should allow single-example tokens: %+v", legacyRules)
	}
}

func TestBuildManualAxisCacheAppliesStripPattern(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfgPath := defaultConfigPath(diceCLIName)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	operator := []byte(`version: 1
entities:
  price_tier:
    source: tickets.priceTier.name
    shape: attributes
    attributes: [price_band]
    strip_pattern: '^[^:]+:\s*'
`)
	if err := os.WriteFile(cfgPath, operator, 0o600); err != nil {
		t.Fatalf("write operator config: %v", err)
	}

	s := openSeededStoreForImport(t)
	csvData := "source_value,price_band\n" +
		"Currency: Premium Alpha,high\n" +
		"Currency: Premium Beta,high\n"
	if _, err := importMapping(s, "dice", "price_tier", []byte(csvData), "csv"); err != nil {
		t.Fatalf("seed generic import: %v", err)
	}

	cache, _, err := buildManualAxisCache(s, "price_tier")
	if err != nil {
		t.Fatalf("buildManualAxisCache: %v", err)
	}
	if _, ok := cache["premium alpha"]; !ok {
		t.Fatalf("stripped canonical key missing; cache=%+v", cache)
	}
	if _, ok := cache["currency premium alpha"]; ok {
		t.Fatalf("unstripped canonical key should not be present; cache=%+v", cache)
	}

	rules, _, _, err := promoteRulesForEntity(s, "price_tier", 2)
	if err != nil {
		t.Fatalf("promoteRulesForEntity: %v", err)
	}
	if !hasRuleMatch(rules, `\bpremium\b`) {
		t.Fatalf("expected stripped token to promote: %+v", rules)
	}
	if hasRuleMatch(rules, `\bcurrency\b`) {
		t.Fatalf("prefix token removed by strip_pattern must not promote: %+v", rules)
	}
}

func TestMergePromotedRulesToConfigDoesNotPersistStarterEntities(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfgPath := filepath.Join(t.TempDir(), "normalize.yaml")
	rules := []normalizecfg.Rule{{Match: `\bvip\b`, Set: map[string]string{"access_class": "vip"}}}

	if err := mergePromotedRulesToConfig(cfgPath, "ticket_type", rules); err != nil {
		t.Fatalf("mergePromotedRulesToConfig absent operator: %v", err)
	}
	cfg := readNormalizeConfigFile(t, cfgPath)
	if len(cfg.Entities) != 1 {
		t.Fatalf("absent operator write should contain only promoted entity, got %d: %+v", len(cfg.Entities), cfg.Entities)
	}
	if cfg.Entities["ticket_type"].Source != "tickets.ticketType.name" {
		t.Fatalf("promoted entity should be backfilled from active config: %+v", cfg.Entities["ticket_type"])
	}

	operator := []byte(`version: 1
entities:
  genre:
    source: events.genres
    shape: vocab
    vocab: [house]
`)
	if err := os.WriteFile(cfgPath, operator, 0o600); err != nil {
		t.Fatalf("replace operator config: %v", err)
	}
	if err := mergePromotedRulesToConfig(cfgPath, "ticket_type", rules); err != nil {
		t.Fatalf("mergePromotedRulesToConfig existing operator: %v", err)
	}
	cfg = readNormalizeConfigFile(t, cfgPath)
	if len(cfg.Entities) != 2 {
		t.Fatalf("existing operator write should preserve only operator entities plus promoted entity, got %d: %+v", len(cfg.Entities), cfg.Entities)
	}
	if _, ok := cfg.Entities["genre"]; !ok {
		t.Fatal("existing operator entity was not preserved")
	}
	if _, ok := cfg.Entities["venue"]; ok {
		t.Fatalf("starter-only venue should not be frozen into operator config: %+v", cfg.Entities)
	}
}

func hasRuleMatch(rules []normalizecfg.Rule, match string) bool {
	for _, r := range rules {
		if r.Match == match {
			return true
		}
	}
	return false
}

func readNormalizeConfigFile(t *testing.T, path string) *normalizecfg.Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg, err := normalizecfg.Parse(data)
	if err != nil {
		t.Fatalf("parse config: %v\n%s", err, data)
	}
	return cfg
}
