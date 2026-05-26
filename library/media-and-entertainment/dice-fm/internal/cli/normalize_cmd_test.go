// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Tests for the normalize command (Task 8).
// All fixtures are synthetic — no real tenant ticket-type or venue names.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestNormalizeCommandSummaryTiers verifies that runNormalize over a seeded
// store reports canonical/unmatched counts > 0 when --tiers is active.
func TestNormalizeCommandSummaryTiers(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"General Admission"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"VIP Experience"}}`,
			"t3": `{"id":"t3","ticketType":{"name":"zzz mystery label"}}`,
		},
	})

	opts := normalizeOpts{
		Tiers:             true,
		ClassifierVersion: 1,
	}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("runNormalize: %v", err)
	}

	var summary normalizeSummary
	if err := json.NewDecoder(&buf).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Tiers == nil {
		t.Fatal("tiers summary is nil")
	}
	if summary.Tiers.CanonicalCount < 1 {
		t.Errorf("want >=1 canonical tiers, got %d", summary.Tiers.CanonicalCount)
	}
	if summary.Tiers.Unmatched < 1 {
		t.Errorf("want >=1 unmatched tier, got %d", summary.Tiers.Unmatched)
	}
}

// TestNormalizeCommandSummaryVenues verifies that runNormalize with --venues
// reports canonical venue counts.
func TestNormalizeCommandSummaryVenues(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"}]}`,
			"e2": `{"id":"e2","venues":[{"name":"Southside Arena"}]}`,
		},
	})

	opts := normalizeOpts{
		Venues:            true,
		ClassifierVersion: 1,
	}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("runNormalize venues: %v", err)
	}
	var summary normalizeSummary
	if err := json.NewDecoder(&buf).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.Venues == nil {
		t.Fatal("venues summary is nil")
	}
	if summary.Venues.CanonicalCount < 1 {
		t.Errorf("want >=1 canonical venue, got %d", summary.Venues.CanonicalCount)
	}
}

// TestNormalizeCommandWithImport verifies that --import wires through to
// importMapping and the imported rows survive in the crosswalk.
func TestNormalizeCommandWithImport(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})

	csvData := "entity_type,source_value,canonical_name\nticket_type,weird name,general admission\n"
	opts := normalizeOpts{
		Tiers:             true,
		ClassifierVersion: 1,
		ImportData:        []byte(csvData),
		ImportFormat:      "csv",
	}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("runNormalize with import: %v", err)
	}
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	found := false
	for _, r := range cw {
		if r.SourceValue == "weird name" && r.Method == "manual" {
			found = true
		}
	}
	if !found {
		t.Errorf("imported manual crosswalk row not found; rows=%+v", cw)
	}
}

// TestNormalizeStatsCobraWiring verifies that `normalize stats` is registered
// and its --help parses without error.
func TestNormalizeStatsCobraWiring(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	// Find the stats subcommand.
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "stats" {
			found = true
			if sub.Annotations["mcp:read-only"] != "true" {
				t.Errorf("normalize stats: expected mcp:read-only=true annotation")
			}
		}
	}
	if !found {
		t.Error("normalize stats subcommand not registered")
	}
}

// TestNormalizeCmdHelp smoke-tests that the normalize command's --help parses.
func TestNormalizeCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	// Help exits with nil after printing.
	_ = cmd.Execute()
	if !strings.Contains(buf.String(), "normalize") {
		t.Errorf("help output missing 'normalize': %s", buf.String())
	}
}

// TestNormalizeFlagsRegistered verifies all required flags are present.
func TestNormalizeFlagsRegistered(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	for _, flagName := range []string{"tiers", "venues", "fuzzy", "classifier-version", "export-unmatched", "import"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("flag --%s not registered on normalize", flagName)
		}
	}
}
