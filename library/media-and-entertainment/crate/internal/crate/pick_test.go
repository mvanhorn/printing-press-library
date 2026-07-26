// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package crate

import (
	"testing"
	"time"
)

func shelf() []Record {
	return []Record{
		{ReleaseID: 1, Title: "Blue Train", Artists: []string{"John Coltrane"}, Year: 1957,
			Labels: []string{"Blue Note"}, Genres: []string{"Jazz"}, Styles: []string{"Hard Bop"},
			Formats: []string{"Vinyl", "LP"}, Rating: 5},
		{ReleaseID: 2, Title: "Selected Ambient Works", Artists: []string{"Aphex Twin"}, Year: 1992,
			Labels: []string{"Apollo"}, Genres: []string{"Electronic"}, Styles: []string{"Ambient", "Techno"},
			Formats: []string{"Vinyl"}, Rating: 0},
		{ReleaseID: 3, Title: "Maiden Voyage", Artists: []string{"Herbie Hancock"}, Year: 1965,
			Labels: []string{"Blue Note"}, Genres: []string{"Jazz"}, Styles: []string{"Post Bop"},
			Formats: []string{"Vinyl"}, Rating: 0},
		{ReleaseID: 4, Title: "Unknown Year", Artists: []string{"Various"}, Year: 0,
			Labels: []string{"White Label"}, Genres: []string{"Rock"}, Formats: []string{"Vinyl"}, Rating: 3},
	}
}

func TestDecadeIgnoresMissingYear(t *testing.T) {
	// Discogs uses 0 for "year not recorded". Bucketing that as the 0s would
	// invent a decade that does not exist.
	if got := (Record{Year: 0}).Decade(); got != "" {
		t.Errorf("missing year should have no decade, got %q", got)
	}
	if got := (Record{Year: 1957}).Decade(); got != "1950s" {
		t.Errorf("1957 -> %q, want 1950s", got)
	}
	if got := (Record{Year: 1990}).Decade(); got != "1990s" {
		t.Errorf("1990 -> %q, want 1990s", got)
	}
}

func TestFilterMatches(t *testing.T) {
	recs := shelf()
	cases := []struct {
		name string
		f    Filter
		want int
	}{
		{"no filter", Filter{}, 4},
		{"genre", Filter{Genre: "Jazz"}, 2},
		{"genre case-insensitive", Filter{Genre: "jazz"}, 2},
		{"label", Filter{Label: "Blue Note"}, 2},
		{"style substring", Filter{Style: "Bop"}, 2},
		{"artist", Filter{Artist: "Coltrane"}, 1},
		{"format", Filter{Format: "LP"}, 1},
		{"decade bare", Filter{Decade: "1960"}, 1},
		{"decade with suffix", Filter{Decade: "1960s"}, 1},
		{"year range", Filter{YearFrom: 1950, YearTo: 1970}, 2},
		{"unrated only", Filter{Unrated: true}, 2},
		{"combined", Filter{Genre: "Jazz", Unrated: true}, 1},
		{"no match", Filter{Genre: "Reggae"}, 0},
		{"nonsense decade", Filter{Decade: "banana"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(tc.f.Apply(recs)); got != tc.want {
				t.Errorf("%+v matched %d records, want %d", tc.f, got, tc.want)
			}
		})
	}
}

// A record with an unknown year must not be swept into a decade filter.
func TestDecadeFilterExcludesUnknownYear(t *testing.T) {
	recs := shelf()
	for _, d := range []string{"1950s", "1960s", "1990s", "2000s"} {
		for _, r := range (Filter{Decade: d}).Apply(recs) {
			if r.Year == 0 {
				t.Errorf("decade filter %q admitted a record with no year: %q", d, r.Title)
			}
		}
	}
}

func TestPickPrefersUnrated(t *testing.T) {
	recs := shelf()
	for seed := int64(0); seed < 12; seed++ {
		got, reason, pool, ok := Pick(recs, Filter{}, seed, true)
		if !ok {
			t.Fatalf("seed %d: expected a pick", seed)
		}
		if got.Rating != 0 {
			t.Errorf("seed %d picked a rated record %q; unrated ones exist", seed, got.Title)
		}
		if pool != 4 {
			t.Errorf("seed %d reported pool %d, want 4", seed, pool)
		}
		if reason == "" {
			t.Errorf("seed %d gave no reason", seed)
		}
	}
}

func TestPickFallsBackWhenAllRated(t *testing.T) {
	recs := shelf()
	for i := range recs {
		recs[i].Rating = 4
	}
	got, reason, _, ok := Pick(recs, Filter{}, 1, true)
	if !ok {
		t.Fatal("expected a pick when every record is rated")
	}
	if got.ReleaseID == 0 {
		t.Error("expected a real record")
	}
	if reason != "from the whole shelf" {
		t.Errorf("reason = %q, want the whole-shelf fallback", reason)
	}
}

func TestPickIsDeterministicForASeed(t *testing.T) {
	recs := shelf()
	a, _, _, _ := Pick(recs, Filter{}, 7, true)
	b, _, _, _ := Pick(recs, Filter{}, 7, true)
	if a.ReleaseID != b.ReleaseID {
		t.Errorf("same seed gave different records: %d then %d", a.ReleaseID, b.ReleaseID)
	}

	// Input order must not change the result, or a re-sync would silently
	// change what a given seed plays.
	reversed := make([]Record, len(recs))
	for i := range recs {
		reversed[i] = recs[len(recs)-1-i]
	}
	c, _, _, _ := Pick(reversed, Filter{}, 7, true)
	if c.ReleaseID != a.ReleaseID {
		t.Errorf("row order changed the pick: %d vs %d", c.ReleaseID, a.ReleaseID)
	}
}

func TestPickEmptyPool(t *testing.T) {
	if _, _, _, ok := Pick(shelf(), Filter{Genre: "Reggae"}, 1, true); ok {
		t.Error("expected no pick when nothing matches")
	}
	if _, _, _, ok := Pick(nil, Filter{}, 1, true); ok {
		t.Error("expected no pick from an empty shelf")
	}
}

func TestPickHandlesNegativeSeed(t *testing.T) {
	if _, _, _, ok := Pick(shelf(), Filter{}, -5, true); !ok {
		t.Error("a negative seed should still pick, not panic or miss")
	}
}

func TestBreakdown(t *testing.T) {
	recs := shelf()

	genres, err := Breakdown(recs, ByGenre)
	if err != nil {
		t.Fatalf("genre breakdown: %v", err)
	}
	if genres[0].Key != "Jazz" || genres[0].Count != 2 {
		t.Errorf("top genre = %+v, want Jazz x2", genres[0])
	}

	labels, _ := Breakdown(recs, ByLabel)
	if labels[0].Key != "Blue Note" || labels[0].Count != 2 {
		t.Errorf("top label = %+v, want Blue Note x2", labels[0])
	}

	// The record with no year must not create a bucket.
	decades, _ := Breakdown(recs, ByDecade)
	for _, d := range decades {
		if d.Key == "" || d.Key == "0s" {
			t.Errorf("decade breakdown invented a bucket for a missing year: %+v", d)
		}
	}
	var total int
	for _, d := range decades {
		total += d.Count
	}
	if total != 3 {
		t.Errorf("decade counts total %d, want 3 (one record has no year)", total)
	}
}

func TestBreakdownShareIsAgainstRecordCount(t *testing.T) {
	recs := shelf()
	styles, _ := Breakdown(recs, ByStyle)
	for _, s := range styles {
		if s.Share <= 0 || s.Share > 1 {
			t.Errorf("share %.3f out of range for %+v", s.Share, s)
		}
	}
	// Styles are multi-valued, so counts may exceed the record count; that is
	// expected and is why share divides by records, not by the count sum.
	var sum int
	for _, s := range styles {
		sum += s.Count
	}
	if sum <= len(recs) {
		t.Logf("style counts (%d) did not exceed records (%d); fixture may be too small", sum, len(recs))
	}
	if !IsMultiValued(ByStyle) {
		t.Error("style should be reported as multi-valued")
	}
	if IsMultiValued(ByDecade) {
		t.Error("decade should not be reported as multi-valued")
	}
}

func TestBreakdownStableForEqualCounts(t *testing.T) {
	recs := shelf()
	first, _ := Breakdown(recs, ByFormat)
	for i := 0; i < 5; i++ {
		again, _ := Breakdown(recs, ByFormat)
		for j := range first {
			if first[j].Key != again[j].Key {
				t.Fatalf("breakdown order is unstable at %d: %q vs %q", j, first[j].Key, again[j].Key)
			}
		}
	}
}

func TestBreakdownUnknownDimension(t *testing.T) {
	if _, err := Breakdown(shelf(), Dimension("colour")); err == nil {
		t.Error("expected an error for an unknown dimension")
	}
}

func TestBreakdownEmptyCollection(t *testing.T) {
	got, err := Breakdown(nil, ByGenre)
	if err != nil {
		t.Fatalf("empty breakdown errored: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no rows, got %d", len(got))
	}
}

func TestArtistLine(t *testing.T) {
	if got := (Record{}).ArtistLine(); got != "Unknown Artist" {
		t.Errorf("no artists -> %q", got)
	}
	if got := (Record{Artists: []string{"A", "B"}}).ArtistLine(); got != "A, B" {
		t.Errorf("two artists -> %q", got)
	}
}

func TestRecordRoundTripFields(t *testing.T) {
	r := Record{ReleaseID: 9, DateAdded: time.Now().UTC().Truncate(time.Second)}
	if r.ReleaseID != 9 {
		t.Error("sanity")
	}
}

// --any must actually widen the pool. It previously printed "drawn from the
// whole matching shelf" while still only ever returning unrated records.
func TestPickAnyDrawsFromRatedRecordsToo(t *testing.T) {
	recs := shelf() // 2 unrated (ids 2,3), 2 rated (ids 1,4)

	var sawRated bool
	for seed := int64(0); seed < 40; seed++ {
		got, _, pool, ok := Pick(recs, Filter{}, seed, false)
		if !ok {
			t.Fatalf("seed %d: expected a pick", seed)
		}
		if pool != 4 {
			t.Errorf("seed %d: pool = %d, want the whole shelf (4)", seed, pool)
		}
		if got.Rating != 0 {
			sawRated = true
		}
	}
	if !sawRated {
		t.Error("preferUnrated=false never returned a rated record; --any is a no-op")
	}
}

func TestPickPreferUnratedStillPrefers(t *testing.T) {
	recs := shelf()
	for seed := int64(0); seed < 20; seed++ {
		got, _, _, _ := Pick(recs, Filter{}, seed, true)
		if got.Rating != 0 {
			t.Fatalf("seed %d returned a rated record while unrated ones exist", seed)
		}
	}
}

func TestBreakdownUnknownDimensionOnEmptyCollection(t *testing.T) {
	// The validation used to live inside the range loop, so an unknown
	// dimension over zero records returned (nil, nil).
	if _, err := Breakdown(nil, Dimension("bogus")); err == nil {
		t.Error("an unknown dimension must error even when there are no records")
	}
}
