// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// testCatalogues mirrors the rows that matter for the find surface, taken
// verbatim from a 2026-05-18 snapshot of Vinny's local Numista cache
// (3,106 rows total). The fixture is deliberately small — just the rows
// the test cases assert on plus a handful of decoys that exercise tier
// boundaries (e.g. another author named Yeoman, another Krause-published
// catalogue). Adding more rows is fine; removing one will tighten the
// assertions because the tiebreak order matters when scores collide.
var testCatalogues = []catalogueRecord{
	// Exact-code test target: 'find KM' must lock to id=3.
	{ID: 3, Code: "KM", Title: "Standard Catalog of World Coins",
		Author:    "Chester Lee Krause, Clifford Mishler, Colin R. Bruce",
		Publisher: "Krause Publications"},
	// Code-prefix tiebreaker decoys: same publisher, different code shapes.
	{ID: 3144, Code: "Curto", Title: "Military Tokens of the United States",
		Author: "James J. Curto", Publisher: "Krause Publications"},
	{ID: 1844, Code: "Bruce", Title: "The Standard Guide to South Asian Coins and Paper Money Since 1556 AD",
		Author: "Colin R. Bruce", Publisher: "Krause Publications"},
	// Title-prefix and author-substring target: 'find yeoman' must lock
	// to id=9 (author match on a canonical reference) before id=925
	// (Yeoman as a co-author on a longer-titled work).
	{ID: 9, Code: "Y", Title: "R. S. Yeoman's Modern & Current World Coins",
		Author:    "Richard Sperry Yeoman, Neil Shafer, Holland Wallace",
		Publisher: "Whitman Publishing"},
	{ID: 925, Code: "Raymond", Title: "The Silver Dollars of North and South America",
		Author:    "Wayte Raymond, Imre Molnar, Richard Sperry Yeoman",
		Publisher: "Whitman Publishing"},
	{ID: 1556, Code: "Y US", Title: "A Guide Book of United States Coins",
		Author:    "Richard Sperry Yeoman, Kenneth Edward Bressett",
		Publisher: "Whitman Publishing"},
	// Diacritic-bearing target: 'find Schön' (or 'find schon' after
	// normalization) must lock to id=24. Title is in German; the author
	// surname carries the diacritic and the query is the canonical lookup.
	{ID: 24, Code: "Schön", Title: "Weltmünzkatalog",
		Author:    "Gerhard Schön, Sebastian Krämer",
		Publisher: "Battenberg Gietl Verlag"},
	// Exact-code target for 'find pcgs' — the cross-walk anchor that
	// closes pp-numista/AGENTS.md Priority 4.
	{ID: 1856, Code: "PCGS", Title: "PCGS CoinFacts",
		Author: "", Publisher: "Professional Coin Grading Services"},
	// Decoy that contains "krause" in publisher to confirm the publisher-
	// substring tier doesn't dethrone the canonical KM (id=3) match.
	{ID: 2375, Code: "Baker", Title: "Medallic Portraits of Washington",
		Author: "William Spohn Baker", Publisher: "Krause Publications"},
}

// TestFuzzyFindCatalogues asserts the top hit for each canonical query
// matches AGENTS.md's expected mapping. The full ranking matters less than
// the top result — agents pipe the first row into 'types search --catalogue
// <id>' for the definitive cross-walk.
func TestFuzzyFindCatalogues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		query     string
		limit     int
		wantTopID int
		wantEmpty bool
	}{
		{"pcgs locks to id=1856 (exact code)", "pcgs", 5, 1856, false},
		{"PCGS uppercase locks to id=1856 (exact code, case-insensitive)", "PCGS", 5, 1856, false},
		{"krause locks to id=3 KM (publisher substring + alpha tiebreak)", "krause", 5, 3, false},
		{"yeoman locks to id=9 Y (author substring, shortest title wins)", "yeoman", 5, 9, false},
		{"KM exact code locks to id=3", "KM", 5, 3, false},
		{"Schön locks to id=24 (exact code match)", "Schön", 5, 24, false},
		{"schon normalized still locks to id=24 (exact code after diacritic fold)", "schon", 5, 24, false},
		{"empty query returns nil", "", 5, 0, true},
		{"limit zero falls back to default", "krause", 0, 3, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fuzzyFindCatalogues(testCatalogues, tc.query, tc.limit)
			if tc.wantEmpty {
				if len(got) != 0 {
					t.Fatalf("expected empty result, got %d matches: %+v", len(got), got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("expected at least one match for %q, got none", tc.query)
			}
			if got[0].ID != tc.wantTopID {
				t.Errorf("top match for %q: got id=%d (code=%s, title=%s), want id=%d",
					tc.query, got[0].ID, got[0].Code, got[0].Title, tc.wantTopID)
			}
		})
	}
}

// TestFuzzyFindCataloguesLimit verifies the limit flag caps the returned
// slice. When the query has more matches than the limit, the function must
// return exactly limit rows, ranked best-first.
func TestFuzzyFindCataloguesLimit(t *testing.T) {
	t.Parallel()
	// "krause" matches id=3, id=3144, id=1844, id=2375 (publisher) — 4 rows.
	matches := fuzzyFindCatalogues(testCatalogues, "krause", 2)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches with limit=2, got %d", len(matches))
	}
	if matches[0].ID != 3 {
		t.Errorf("top match for krause limit=2: got id=%d, want id=3", matches[0].ID)
	}
}

// TestScoreCatalogueTiers covers the boundary between scoring tiers. Each
// tier must outrank the one below it, regardless of the magnitudes within
// a tier — the test fails if a future scoring tweak inverts a tier
// boundary (e.g. a long-title prefix beating an exact-code match).
func TestScoreCatalogueTiers(t *testing.T) {
	t.Parallel()
	exactCode := catalogueRecord{ID: 1, Code: "PCGS", Title: "PCGS CoinFacts"}
	exactTitle := catalogueRecord{ID: 2, Code: "X", Title: "pcgs"}
	codePrefix := catalogueRecord{ID: 3, Code: "PCGSExtra", Title: "Something"}
	titleWordPrefix := catalogueRecord{ID: 4, Code: "Z", Title: "PCGSExtended Reference"}
	substring := catalogueRecord{ID: 5, Code: "Q", Title: "Modern References including PCGS data"}
	noMatch := catalogueRecord{ID: 6, Code: "Foo", Title: "Bar"}

	q := normalizeCatalogueQuery("pcgs")
	scores := []struct {
		name string
		rec  catalogueRecord
	}{
		{"exact code", exactCode},
		{"exact title", exactTitle},
		{"code prefix", codePrefix},
		{"title word prefix", titleWordPrefix},
		{"substring", substring},
		{"no match", noMatch},
	}
	prev := 1 << 30
	for _, s := range scores {
		got := scoreCatalogue(s.rec, q)
		if got >= prev {
			t.Errorf("tier %q scored %d, expected strictly less than previous tier (%d)",
				s.name, got, prev)
		}
		prev = got
	}
	if prev != 0 {
		t.Errorf("no-match tier scored %d, expected 0", prev)
	}
}

// TestLoadCataloguesFromCacheMissing exercises the empty-cache contract:
// when the on-disk store can't be opened (no file, wrong permissions,
// schema mismatch), loadCataloguesFromCache returns (nil, nil) so the
// caller can render the warm-the-cache message rather than erroring.
// Setting a bogus HOME isolates the test from the developer's real
// ~/.local/share/numista-pp-cli/data.db.
func TestLoadCataloguesFromCacheMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	got, err := loadCataloguesFromCache()
	if err != nil {
		t.Fatalf("expected nil error on missing cache, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil slice on missing cache, got %d records", len(got))
	}
}

// TestIntFromAny covers the JSON-numeric coercion contract. encoding/json
// decodes numeric values as float64 through the generic map[string]any
// path — the function must coerce that back to int, plus handle the
// pre-existing int/int64/json.Number/string-of-digits edge cases the
// loader may encounter as the schema evolves.
func TestIntFromAny(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
		ok   bool
	}{
		{"float64 (json default)", float64(1856), 1856, true},
		{"int", 1856, 1856, true},
		{"int64", int64(1856), 1856, true},
		{"string of digits", "1856", 1856, true},
		{"empty string", "", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := intFromAny(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok: got %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("value: got %d, want %d", got, tc.want)
			}
		})
	}
}
