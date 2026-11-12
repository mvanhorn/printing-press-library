// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): tests for the rankings page
// parser. The fixture-backed test reads
// testdata/rankings-tech-fixture.html.

package diggparse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadRankingsFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("fixture %q not found; tried: %v", name, candidates)
	return nil
}

// TestDecodeCompanyEntries_AllInvalidRowsSurfaceAsDrift simulates the
// schema-rename catastrophe: every row decodes structurally but lacks
// the username field we need. The schema-drift detector requires that
// such cases register as 100% Skipped — otherwise the CLI silently
// emits nothing and downstream consumers see "no deals today" instead
// of "your scraper broke".
func TestDecodeCompanyEntries_AllInvalidRowsSurfaceAsDrift(t *testing.T) {
	raw := json.RawMessage(`[
		{"rank":1,"target_x_id":"a","followed_by_count":1,"followers_count":1,"score":0},
		{"rank":2,"target_x_id":"b","followed_by_count":1,"followers_count":1,"score":0},
		{"rank":3,"target_x_id":"c","followed_by_count":1,"followers_count":1,"score":0}
	]`)
	entries, stats := decodeCompanyEntries(raw)
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0 (every row missing username)", len(entries))
	}
	if stats.Attempted != 3 || stats.Skipped != 3 || stats.Decoded != 0 {
		t.Errorf("stats = %+v, want Attempted=3 Skipped=3 Decoded=0", stats)
	}
	if stats.SkipRatio() != 1.0 {
		t.Errorf("SkipRatio = %v, want 1.0", stats.SkipRatio())
	}
	if err := stats.Threshold(0.10); err == nil {
		t.Error("Threshold(0.10) on 100%% drift = nil, want ThresholdError")
	}
}

// TestDecodeCompanyEntries_InterleavedReferencesAndObjects mirrors the
// real RSC stream shape where the entries array mixes inline company
// objects with reference strings to entries rendered elsewhere.
func TestDecodeCompanyEntries_InterleavedReferencesAndObjects(t *testing.T) {
	raw := json.RawMessage(`[
		{"rank":1,"target_x_id":"x","username":"u1","followed_by_count":1,"followers_count":1,"score":0},
		"$3a:props:left:entries:1",
		{"rank":2,"target_x_id":"y","username":"u2","followed_by_count":1,"followers_count":1,"score":0},
		"$3a:props:right:entries:0"
	]`)
	entries, stats := decodeCompanyEntries(raw)
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2 (references skipped, objects kept)", len(entries))
	}
	// Refs don't count toward Attempted; only the two object decode attempts.
	if stats.Attempted != 2 || stats.Decoded != 2 || stats.Skipped != 0 {
		t.Errorf("stats = %+v, want Attempted=2 Decoded=2 Skipped=0", stats)
	}
}

// TestDecodeCompanyEntries_MalformedInnerJSONCountedAsSkipped covers
// the per-entry parse-failure path (as distinct from the schema-rename
// missing-field path above).
func TestDecodeCompanyEntries_MalformedInnerJSONCountedAsSkipped(t *testing.T) {
	raw := json.RawMessage(`[
		{"rank":1,"target_x_id":"x","username":"u","followed_by_count":1,"followers_count":1,"score":0},
		{"rank":"BROKEN","target_x_id":"y","username":"u2"}
	]`)
	entries, stats := decodeCompanyEntries(raw)
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
	if stats.Attempted != 2 || stats.Decoded != 1 || stats.Skipped != 1 {
		t.Errorf("stats = %+v, want Attempted=2 Decoded=1 Skipped=1", stats)
	}
	if len(stats.Errors) != 1 {
		t.Errorf("stats.Errors len = %d, want 1 (rank type mismatch)", len(stats.Errors))
	}
}

func TestExtractMainRanking_DisambiguatesFromEmergingMoversWrappers(t *testing.T) {
	// Embed both a section wrapper (with `direction`) and a main
	// wrapper (without `direction`) in the same decoded stream. The
	// main extractor must pick the direction-less one even though its
	// entries array has fewer than the section's.
	decoded := `
		prefix
		{"direction":"emerging","entries":[
			{"rank":938,"target_x_id":"x1","username":"u1","followed_by_count":1,"followers_count":1,"score":0},
			{"rank":942,"target_x_id":"x2","username":"u2","followed_by_count":1,"followers_count":1,"score":0}
		]}
		middle
		{"entries":[
			{"rank":1,"target_x_id":"y1","username":"top","followed_by_count":1,"followers_count":1,"score":0}
		]}
		suffix
	`
	entries, stats, err := ExtractMainRanking(decoded)
	if err != nil {
		t.Fatalf("ExtractMainRanking: %v", err)
	}
	if len(entries) != 1 || entries[0].Username != "top" {
		t.Errorf("got entries %+v, want exactly [top]", entries)
	}
	if stats.Skipped != 0 || stats.Decoded != 1 {
		t.Errorf("stats = %+v, want Decoded=1 Skipped=0", stats)
	}
}

func TestExtractMainRanking_SkipsLookalikeDirectionlessSections(t *testing.T) {
	// A pagination/navigation section that has `"entries":[...]` but
	// without target_x_id should NOT be picked as main even if it's
	// larger than the real ranking.
	decoded := `
		{"entries":[
			{"page":1,"label":"first"},
			{"page":2,"label":"second"},
			{"page":3,"label":"third"},
			{"page":4,"label":"fourth"}
		]}
		later
		{"entries":[
			{"rank":1,"target_x_id":"x","username":"OpenAI","followed_by_count":1,"followers_count":1,"score":0},
			{"rank":2,"target_x_id":"y","username":"Anthropic","followed_by_count":1,"followers_count":1,"score":0}
		]}
	`
	entries, _, err := ExtractMainRanking(decoded)
	if err != nil {
		t.Fatalf("ExtractMainRanking: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (lookalike section skipped)", len(entries))
	}
	if entries[0].Username != "OpenAI" {
		t.Errorf("got first %q, want OpenAI", entries[0].Username)
	}
}

func TestIsRSCReference_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"object_not_ref", `{"rank":1}`, false},
		{"plain_string_not_ref", `"hello"`, false},
		{"normal_ref", `"$3a:props:left:entries:1"`, true},
		{"ref_with_leading_whitespace", "  \n\t\"$L1\"", true},
		{"empty_string_value", `""`, false},
		{"too_short", `"`, false},
		{"empty_bytes", ``, false},
		{"number", `42`, false},
		{"null", `null`, false},
		{"string_starts_with_dollar_dollar", `"$$money"`, true}, // accepted false positive — see docstring
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRSCReference(json.RawMessage(tc.in))
			if got != tc.want {
				t.Errorf("isRSCReference(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// The test below runs against testdata/rankings-tech-fixture.html, a
// trimmed capture of https://digg.com/tech/x/rankings (2026-10-21). The
// page ships only the full people-and-companies ranking.

func TestCurrentTechRankingsPage_MainRankingParses(t *testing.T) {
	decoded := DecodeRSC(loadRankingsFixture(t, "rankings-tech-fixture.html"))
	if decoded == "" {
		t.Fatal("DecodeRSC returned empty for the current-page fixture")
	}
	entries, stats, err := ExtractMainRanking(decoded)
	if err != nil {
		t.Fatalf("ExtractMainRanking: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one ranking entry from the current page")
	}
	if entries[0].Rank != 1 || entries[0].Username != "karpathy" {
		t.Errorf("entries[0] = rank %d @%s, want rank 1 @karpathy", entries[0].Rank, entries[0].Username)
	}
	if stats.Skipped != 0 {
		t.Errorf("stats = %+v, want no skipped entries", stats)
	}
}
