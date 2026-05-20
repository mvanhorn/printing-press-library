// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): tests for the rankings/companies
// parser. Backed by three checked-in fixtures:
//   testdata/rankings-companies-fixture.html        (pristine, trimmed)
//   testdata/rankings-companies-dirty-fixture.html  (hand-mutated rows)
//   testdata/rankings-companies-empty-fixture.html  (no RSC pushes)

package diggparse

import (
	"encoding/json"
	"errors"
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

func TestParseRankingsCompanies_PristineFixture(t *testing.T) {
	html := loadRankingsFixture(t, "rankings-companies-fixture.html")
	r, err := ParseRankingsCompanies(html)
	if err != nil {
		t.Fatalf("ParseRankingsCompanies: %v", err)
	}

	// Section sizes from the captured snapshot: 10 emerging, 10 + 10 movers, 30 main.
	if len(r.Emerging) < 5 || len(r.Emerging) > 15 {
		t.Errorf("emerging count = %d, want roughly 10", len(r.Emerging))
	}
	if len(r.MoversUp) < 1 || len(r.MoversUp) > 15 {
		t.Errorf("moversUp count = %d, want roughly 10", len(r.MoversUp))
	}
	if len(r.MoversDown) < 1 || len(r.MoversDown) > 15 {
		t.Errorf("moversDown count = %d, want roughly 10", len(r.MoversDown))
	}
	if len(r.Main) < 20 {
		t.Errorf("main count = %d, want >= 20", len(r.Main))
	}

	// Direction stamping
	for _, e := range r.MoversUp {
		if e.Direction != "up" {
			t.Errorf("moversUp entry @%s direction = %q, want \"up\"", e.Username, e.Direction)
		}
	}
	for _, e := range r.MoversDown {
		if e.Direction != "down" {
			t.Errorf("moversDown entry @%s direction = %q, want \"down\"", e.Username, e.Direction)
		}
	}
	for _, e := range r.Emerging {
		if e.Direction != "" {
			t.Errorf("emerging entry @%s direction = %q, want empty", e.Username, e.Direction)
		}
	}
	for _, e := range r.Main {
		if e.Direction != "" {
			t.Errorf("main entry @%s direction = %q, want empty", e.Username, e.Direction)
		}
	}

	// Every row has a non-empty username and positive rank
	for _, sl := range [][]CompanyEntry{r.Emerging, r.MoversUp, r.MoversDown, r.Main} {
		for _, e := range sl {
			if e.Username == "" {
				t.Errorf("rank %d has empty username", e.Rank)
			}
			if e.Rank <= 0 {
				t.Errorf("@%s has non-positive rank %d", e.Username, e.Rank)
			}
		}
	}

	// At least one Emerging entry must carry the curated-flag (proves
	// the IsEmergingStartup boolean is decoded).
	found := false
	for _, e := range r.Emerging {
		if e.IsEmergingStartup {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one Emerging entry with IsEmergingStartup=true")
	}

	// Spot-check: main ranking should include "OpenAI" at rank 1 (from
	// the live snapshot at fixture capture time).
	var openai *CompanyEntry
	for i := range r.Main {
		if r.Main[i].Username == "OpenAI" {
			openai = &r.Main[i]
			break
		}
	}
	if openai == nil {
		t.Fatal("expected fixture's main ranking to contain OpenAI")
	}
	if openai.Rank != 1 {
		t.Errorf("OpenAI rank = %d, want 1", openai.Rank)
	}

	// Pristine fixture should parse cleanly across all sections.
	for label, s := range map[string]ParseStats{
		"emerging": r.EmergingStats,
		"movers":   r.MoversStats,
		"main":     r.MainStats,
	} {
		if s.Skipped != 0 {
			t.Errorf("%s: pristine fixture had %d skipped entries (errors: %v)",
				label, s.Skipped, s.Errors)
		}
	}
}

func TestParseRankingsCompanies_DirtyFixturePartialSucceeds(t *testing.T) {
	html := loadRankingsFixture(t, "rankings-companies-dirty-fixture.html")
	r, err := ParseRankingsCompanies(html)
	if err != nil {
		t.Fatalf("ParseRankingsCompanies(dirty) returned hard error: %v", err)
	}
	// One Emerging entry has rank:"oops" (string instead of int) —
	// extractor should record one Skipped attempt and still return
	// the rest of the section.
	if r.EmergingStats.Skipped != 1 {
		t.Errorf("emerging.Skipped = %d, want 1 (rank-type-mismatch entry); errors: %v",
			r.EmergingStats.Skipped, r.EmergingStats.Errors)
	}
	if len(r.Emerging) < 1 {
		t.Errorf("emerging slice = %d, expected partial result with at least 1 row",
			len(r.Emerging))
	}
	// One Movers-up entry has its username removed. With the schema-
	// drift detector active, the missing-username entry counts as a
	// Skipped attempt (NOT a silent drop) so a hypothetical "upstream
	// renamed `username` to `user_handle`" rename would surface as
	// every entry being Skipped, not as a silent empty result.
	if r.MoversStats.Skipped < 1 {
		t.Errorf("movers.Skipped = %d, want >= 1 (missing-username defensive drop); errors: %v",
			r.MoversStats.Skipped, r.MoversStats.Errors)
	}
	upUsernames := make(map[string]bool)
	for _, e := range r.MoversUp {
		upUsernames[e.Username] = true
	}
	if _, ok := upUsernames[""]; ok {
		t.Error("moversUp output contains an entry with empty username (defensive drop failed)")
	}
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
	entries, stats := decodeCompanyEntries(raw, "")
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
	entries, stats := decodeCompanyEntries(raw, "")
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
	entries, stats := decodeCompanyEntries(raw, "")
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

func TestParseRankingsCompanies_EmptyFixtureReturnsTypedError(t *testing.T) {
	html := loadRankingsFixture(t, "rankings-companies-empty-fixture.html")
	r, err := ParseRankingsCompanies(html)
	if err == nil {
		t.Fatalf("expected error for empty fixture, got result: %+v", r)
	}
	// Error message should mention "RSC pushes" or "page shape" so
	// operators reading stderr know what changed.
	msg := err.Error()
	if !contains(msg, "RSC") && !contains(msg, "page shape") {
		t.Errorf("error message did not mention RSC/page-shape: %q", msg)
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

func TestExtractEmerging_MissingSectionReturnsErrSectionNotFound(t *testing.T) {
	decoded := `{"entries":[{"rank":1,"username":"x","followed_by_count":0,"followers_count":0,"score":0}]}`
	entries, stats, err := ExtractEmerging(decoded)
	if !errors.Is(err, ErrSectionNotFound) {
		t.Errorf("err = %v, want errors.Is ErrSectionNotFound", err)
	}
	if entries != nil {
		t.Errorf("entries = %+v, want nil", entries)
	}
	if stats.Attempted != 0 {
		t.Errorf("stats.Attempted = %d, want 0 (section never reached)", stats.Attempted)
	}
}

func TestExtractMovers_PartialSucceedsWithOneSide(t *testing.T) {
	// Only the up side present — should return the up entries with
	// down as nil, NOT fail outright. A future page variant that
	// hides one side shouldn't break the whole command.
	decoded := `{"left":{"direction":"up","entries":[{"rank":1,"target_x_id":"x","username":"u","followed_by_count":0,"followers_count":0,"score":0}]}}`
	up, down, stats, err := ExtractMovers(decoded)
	if err != nil {
		t.Fatalf("ExtractMovers(left-only): %v, want nil", err)
	}
	if len(up) != 1 || up[0].Username != "u" {
		t.Errorf("up = %+v, want one entry @u", up)
	}
	if down != nil {
		t.Errorf("down = %+v, want nil (right side absent)", down)
	}
	if stats.Decoded != 1 || stats.Skipped != 0 {
		t.Errorf("stats = %+v, want Decoded=1 Skipped=0", stats)
	}
}

func TestExtractMovers_BothSidesMissingReturnsErrSectionNotFound(t *testing.T) {
	// The `"left":{"direction":"up"` needle isn't present at all
	// (page doesn't have a Movers component) — should be the
	// section-not-found case.
	decoded := `{"entries":[{"rank":1,"target_x_id":"x","username":"u","followed_by_count":0,"followers_count":0,"score":0}]}`
	_, _, _, err := ExtractMovers(decoded)
	if !errors.Is(err, ErrSectionNotFound) {
		t.Errorf("err = %v, want errors.Is ErrSectionNotFound", err)
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

// contains is a tiny stdlib-free substring check used by the typed-error
// message assertions above. Avoids pulling in strings just for the test.
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
