// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the normalize command (Task 8).
// All fixtures are synthetic — no real tenant ticket-type or venue names.
package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
	"github.com/spf13/cobra"
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
	tiers, ok := summary["tiers"]
	if !ok {
		t.Fatal("tiers summary is missing")
	}
	if tiers.CanonicalCount < 1 {
		t.Errorf("want >=1 canonical tiers, got %d", tiers.CanonicalCount)
	}
	if tiers.Unmatched < 1 {
		t.Errorf("want >=1 unmatched tier, got %d", tiers.Unmatched)
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
	venues, ok := summary["venues"]
	if !ok {
		t.Fatal("venues summary is missing")
	}
	if venues.CanonicalCount < 1 {
		t.Errorf("want >=1 canonical venue, got %d", venues.CanonicalCount)
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

func TestNormalizePromoteRulesCobraWiring(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNormalizeCmd(flags)
	var promote *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "promote-rules" {
			promote = sub
			break
		}
	}
	if promote == nil {
		t.Fatal("normalize promote-rules subcommand not registered")
	}
	for _, flagName := range []string{"entity", "write", "min-support"} {
		if promote.Flags().Lookup(flagName) == nil {
			t.Errorf("normalize promote-rules: flag --%s not registered", flagName)
		}
	}
	if promote.Annotations["mcp:read-only"] == "true" {
		t.Error("normalize promote-rules must not advertise mcp:read-only because --write mutates the operator config")
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
	for _, flagName := range []string{"tiers", "venues", "all", "entity", "fuzzy", "classifier-version", "export-unmatched", "import"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("flag --%s not registered on normalize", flagName)
		}
	}
}

// TestExportUnmatchedCSVRoundTrip verifies that exportUnmatched writes valid
// CSV that can be re-imported, including source values containing commas and
// double quotes — characters that raw fmt.Fprintf would misformat.
func TestExportUnmatchedCSVRoundTrip(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"weird, name with comma"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"name with \"quote\""}}`,
		},
	})

	// Classify so unmatched rows are written to the crosswalk.
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}

	// Export unmatched rows (names format keeps the old CSV-only behaviour).
	outPath := filepath.Join(t.TempDir(), "unmatched.csv")
	if err := exportUnmatchedWithFormat(context.Background(), s, "ticket_type", outPath, "names"); err != nil {
		t.Fatalf("exportUnmatchedWithFormat: %v", err)
	}

	// Re-read with encoding/csv — must round-trip without parse errors and
	// preserve the original values including embedded commas and quotes.
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open exported csv: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("csv parse exported file: %v (file not valid CSV)", err)
	}
	// First row is the header.
	if len(records) < 1 {
		t.Fatal("exported CSV is empty")
	}
	// Build a set of the exported source values. The "names" format writes a
	// single source_value column, so rec[0] is the value.
	exported := map[string]bool{}
	for _, rec := range records[1:] {
		if len(rec) < 1 {
			t.Fatalf("short record: %v", rec)
		}
		exported[rec[0]] = true
	}
	for _, want := range []string{`weird, name with comma`, `name with "quote"`} {
		if !exported[want] {
			t.Errorf("source value %q not found in exported CSV; got: %v", want, exported)
		}
	}
}

// TestIdempotentRerunClearsStaleRows verifies that a second classifyTiers call
// does NOT leave stale crosswalk rows when a ticket's source value was removed
// from the store between runs, and that a method='manual' row survives.
func TestIdempotentRerunClearsStaleRows(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"General Admission"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"stale ticket type"}}`,
		},
	})

	// Pre-seed a manual crosswalk row that must survive.
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: "manual override", CanonicalID: "ticket_type:manual-999",
		Method: "manual", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("pre-seed manual: %v", err)
	}

	// First classify run.
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("first classifyTiers: %v", err)
	}

	// Verify "stale ticket type" was classified.
	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	hasStale := false
	for _, r := range cw {
		if r.SourceValue == "stale ticket type" {
			hasStale = true
		}
	}
	if !hasStale {
		t.Fatal("stale ticket type row missing after first run")
	}

	// Remove the stale ticket from the store.
	if _, err := s.DB().Exec(`DELETE FROM resources WHERE resource_type='tickets' AND id='t2'`); err != nil {
		t.Fatalf("delete stale ticket: %v", err)
	}

	// Second classify run — classifyTiers calls ClearNormalization first, so
	// stale derived rows must be gone.
	if _, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1}); err != nil {
		t.Fatalf("second classifyTiers: %v", err)
	}

	cw2, _ := s.ListCrosswalk("ticket_type", "dice")
	for _, r := range cw2 {
		if r.SourceValue == "stale ticket type" {
			t.Errorf("stale crosswalk row survived second run: %+v", r)
		}
	}
	// Manual row must survive.
	manualFound := false
	for _, r := range cw2 {
		if r.SourceValue == "manual override" && r.Method == "manual" {
			manualFound = true
		}
	}
	if !manualFound {
		t.Errorf("manual crosswalk row did not survive second run; rows=%+v", cw2)
	}
}

// TestNormalizeCommandWithImportSurvivesRerun extends TestNormalizeCommandWithImport
// by verifying the imported manual row still has method="manual" after a second
// runNormalize call (ClearNormalization preserves manual rows).
func TestNormalizeCommandWithImportSurvivesRerun(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})

	csvData := "entity_type,source_value,canonical_name\nticket_type,weird name,general admission\n"
	opts := normalizeOpts{
		Tiers:             true,
		ClassifierVersion: 1,
		ImportData:        []byte(csvData),
		ImportFormat:      "csv",
	}

	// First run (import + classify).
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("first runNormalize: %v", err)
	}

	// Second run without import data (re-classify only).
	opts2 := normalizeOpts{Tiers: true, ClassifierVersion: 1}
	var buf2 bytes.Buffer
	if err := runNormalize(context.Background(), s, opts2, &buf2); err != nil {
		t.Fatalf("second runNormalize: %v", err)
	}

	cw, _ := s.ListCrosswalk("ticket_type", "dice")
	found := false
	for _, r := range cw {
		if r.SourceValue == "weird name" && r.Method == "manual" {
			found = true
		}
	}
	if !found {
		t.Errorf("manual crosswalk row did not survive second runNormalize; rows=%+v", cw)
	}
}

// TestNormalizeAllClassifiesDeclaredEntities verifies that --all drives the
// generic classifyEntity for every declared entity, not just tiers/venues. The
// alias entities (artist, ticket_pool) resolve every value, so their summaries
// report Matched>=1; the vocab genre entity with an empty starter vocab reports
// all-unmatched but is still present. The two original spines stay keyed
// "tiers"/"venues" for back-compat.
func TestNormalizeAllClassifiesDeclaredEntities(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"}],"artists":[{"name":"DJ Synthetic"}],"genres":["house"],"ticketPools":[{"name":"Pool Alpha","allocation":100}]}`,
			"e2": `{"id":"e2","venues":[{"name":"Southside Arena"}],"artists":[{"name":"The Test Band"}],"genres":["techno"],"ticketPools":[{"name":"Pool Beta","allocation":50}]}`,
		},
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"General Admission"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"VIP Experience"}}`,
		},
	})

	opts := normalizeOpts{All: true, ClassifierVersion: 1}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("runNormalize --all: %v", err)
	}

	var summary normalizeSummary
	if err := json.NewDecoder(&buf).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	// Back-compat keys for the two original spines.
	if _, ok := summary["tiers"]; !ok {
		t.Errorf("--all summary missing back-compat key %q; got keys %v", "tiers", keysOf(summary))
	}
	if _, ok := summary["venues"]; !ok {
		t.Errorf("--all summary missing back-compat key %q; got keys %v", "venues", keysOf(summary))
	}

	// Alias entities resolve every value -> Matched>=1.
	for _, key := range []string{"artist", "ticket_pool"} {
		r, ok := summary[key]
		if !ok {
			t.Errorf("--all summary missing entity key %q; got keys %v", key, keysOf(summary))
			continue
		}
		if r.Matched < 1 {
			t.Errorf("entity %q: want Matched>=1 (alias resolves every value), got %+v", key, r)
		}
	}

	// genre is present even though its empty starter vocab leaves all values unmatched.
	if _, ok := summary["genre"]; !ok {
		t.Errorf("--all summary missing entity key %q; got keys %v", "genre", keysOf(summary))
	}
}

// keysOf returns the sorted keys of a normalizeSummary for stable error output.
func keysOf(m normalizeSummary) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TestNormalizeEntityFlagUnknownErrors verifies that --entity with an entity
// name not declared in the loaded config returns a clear error.
func TestNormalizeEntityFlagUnknownErrors(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})

	opts := normalizeOpts{Entities: []string{"bogus"}, ClassifierVersion: 1}
	var buf bytes.Buffer
	err := runNormalize(context.Background(), s, opts, &buf)
	if err == nil {
		t.Fatal("want error for unknown --entity, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should name the unknown entity; got %v", err)
	}
}

// TestNormalizeDefaultStillTiers verifies that with no entity flags set the run
// classifies only the ticket_type spine (keyed "tiers"), preserving today's
// default behavior.
func TestNormalizeDefaultStillTiers(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"General Admission"}}`,
		},
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"}]}`,
		},
	})

	opts := normalizeOpts{ClassifierVersion: 1}
	var buf bytes.Buffer
	if err := runNormalize(context.Background(), s, opts, &buf); err != nil {
		t.Fatalf("runNormalize default: %v", err)
	}

	var summary normalizeSummary
	if err := json.NewDecoder(&buf).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if _, ok := summary["tiers"]; !ok {
		t.Errorf("default run should classify tiers; got keys %v", keysOf(summary))
	}
	if len(summary) != 1 {
		t.Errorf("default run should classify only tiers, got keys %v", keysOf(summary))
	}
}
