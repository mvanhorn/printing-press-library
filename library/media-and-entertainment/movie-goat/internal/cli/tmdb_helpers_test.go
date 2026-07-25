package cli

import "testing"

func TestSplitInlineYear(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantYear  string
	}{
		{"Sabrina (1954)", "Sabrina", "1954"},
		{"Sabrina(1995)", "Sabrina", "1995"},
		{"  Ocean's Eleven (2001)  ", "Ocean's Eleven", "2001"},
		{"Sabrina", "Sabrina", ""},
		{"Blade Runner 2049", "Blade Runner 2049", ""},
		// Not a year: must be left intact so the literal title still searches.
		{"Brazil (Director's Cut)", "Brazil (Director's Cut)", ""},
		{"Se7en (123)", "Se7en (123)", ""},
		{"Thing (17760)", "Thing (17760)", ""},
		// A bare qualifier with no title is not a split.
		{"(1954)", "(1954)", ""},
	}
	for _, tc := range cases {
		gotTitle, gotYear := splitInlineYear(tc.in)
		if gotTitle != tc.wantTitle || gotYear != tc.wantYear {
			t.Errorf("splitInlineYear(%q) = (%q, %q), want (%q, %q)",
				tc.in, gotTitle, gotYear, tc.wantTitle, tc.wantYear)
		}
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := []struct{ a, b string }{
		{"Sabrina", "sabrina"},
		{"Ocean's Eleven", "Oceans   Eleven"},
		{"Wall-E", "WALL E"},
		{"Spider-Man", "Spider Man"},
		{"Amélie", "amélie"},
	}
	for _, tc := range cases {
		if normalizeTitle(tc.a) != normalizeTitle(tc.b) {
			t.Errorf("normalizeTitle(%q)=%q != normalizeTitle(%q)=%q",
				tc.a, normalizeTitle(tc.a), tc.b, normalizeTitle(tc.b))
		}
	}
	if normalizeTitle("Sabrina") == normalizeTitle("Sabrina Goes to Rome") {
		t.Error("normalizeTitle must not collapse a title into a longer one")
	}
	if got := normalizeTitle("   "); got != "" {
		t.Errorf("normalizeTitle(whitespace) = %q, want empty", got)
	}
}

func TestExactTitleMatches(t *testing.T) {
	results := []tmdbSearchResult{
		{ID: 11527, Title: "Sabrina", ReleaseDate: "1995-12-15"},
		{ID: 6620, Title: "Sabrina", ReleaseDate: "1954-09-22"},
		{ID: 999, Title: "Sabrina Goes to Rome", ReleaseDate: "1998-01-01"},
		{ID: 111, Title: "Localized Title", OriginalTitle: "Sabrina", ReleaseDate: "1960-01-01"},
	}
	got := exactTitleMatches(results, "sabrina")
	if len(got) != 3 {
		t.Fatalf("exactTitleMatches returned %d results, want 3: %+v", len(got), got)
	}
	if got[0].ID != 11527 {
		t.Errorf("first match = %d, want 11527 (TMDb's own ordering must be preserved)", got[0].ID)
	}
	if n := len(exactTitleMatches(results, "Inception")); n != 0 {
		t.Errorf("exactTitleMatches for a non-matching query returned %d, want 0", n)
	}
	if n := len(exactTitleMatches(results, "  ")); n != 0 {
		t.Errorf("exactTitleMatches for a blank query returned %d, want 0", n)
	}
}

func TestNotableAlternatives(t *testing.T) {
	// Real TMDb vote counts, 2026-07: the remake outranks the better-rated
	// original on popularity, and a long tail of unrated entries shares the name.
	sabrina := []tmdbSearchResult{
		{ID: 11860, Title: "Sabrina", ReleaseDate: "1995-12-15", VoteCount: 703},
		{ID: 6620, Title: "Sabrina", ReleaseDate: "1954-09-22", VoteCount: 1373},
		{ID: 503902, Title: "Sabrina", ReleaseDate: "2018-01-01", VoteCount: 194},
		{ID: 780623, Title: "Sabrina", ReleaseDate: "2019-01-01", VoteCount: 3},
		{ID: 708143, Title: "Sabrina", ReleaseDate: "2011-01-01", VoteCount: 0},
	}
	got := notableAlternatives(sabrina, notableVoteFloor)
	if len(got) != 2 {
		t.Fatalf("Sabrina alternatives = %d, want 2 (1954 + 2018): %+v", len(got), got)
	}
	if got[0].ID != 6620 {
		t.Errorf("first Sabrina alternative = %d, want 6620", got[0].ID)
	}

	// A famous title whose only namesake is an unrated obscurity must stay silent.
	inception := []tmdbSearchResult{
		{ID: 27205, Title: "Inception", ReleaseDate: "2010-07-15", VoteCount: 39638},
		{ID: 1359046, Title: "Inception", ReleaseDate: "1980-01-01", VoteCount: 0},
	}
	if n := len(notableAlternatives(inception, notableVoteFloor)); n != 0 {
		t.Errorf("Inception alternatives = %d, want 0 (unrated namesake is noise)", n)
	}

	// When the chosen entry is itself obscure, a better-rated namesake still counts.
	obscure := []tmdbSearchResult{
		{ID: 1, Title: "Foo", VoteCount: 2},
		{ID: 2, Title: "Foo", VoteCount: 9},
	}
	if n := len(notableAlternatives(obscure, notableVoteFloor)); n != 1 {
		t.Errorf("obscure-chosen alternatives = %d, want 1", n)
	}

	// People have no vote counts; the relative-popularity gate applies instead.
	people := []tmdbSearchResult{
		{ID: 16828, Name: "Chris Evans", Popularity: 6.18},
		{ID: 1212362, Name: "Chris Evans", Popularity: 0.76},
		{ID: 2219504, Name: "Chris Evans", Popularity: 0.21},
	}
	if n := len(notableAlternatives(people, 0)); n != 0 {
		t.Errorf("person alternatives = %d, want 0 (namesakes far below the top match)", n)
	}
	rivals := []tmdbSearchResult{
		{ID: 1, Name: "Alex Smith", Popularity: 4.0},
		{ID: 2, Name: "Alex Smith", Popularity: 3.0},
	}
	if n := len(notableAlternatives(rivals, 0)); n != 1 {
		t.Errorf("comparable-namesake alternatives = %d, want 1", n)
	}

	if n := len(notableAlternatives([]tmdbSearchResult{{ID: 1}}, notableVoteFloor)); n != 0 {
		t.Errorf("single match produced %d alternatives, want 0", n)
	}
}

func TestDescribeResult(t *testing.T) {
	cases := []struct {
		in   tmdbSearchResult
		want string
	}{
		{tmdbSearchResult{Title: "Sabrina", ReleaseDate: "1954-09-22"}, "Sabrina (1954)"},
		{tmdbSearchResult{Name: "Fargo", FirstAirDate: "2014-04-15"}, "Fargo (2014)"},
		{tmdbSearchResult{Name: "Chris Evans", KnownFor: "Acting"}, "Chris Evans (Acting)"},
		{tmdbSearchResult{Title: "Untitled"}, "Untitled"},
	}
	for _, tc := range cases {
		if got := describeResult(tc.in); got != tc.want {
			t.Errorf("describeResult(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateYearFlag(t *testing.T) {
	good := []string{"", "1954", " 2001 "}
	want := []string{"", "1954", "2001"}
	for i, in := range good {
		got, err := validateYearFlag(in)
		if err != nil {
			t.Errorf("validateYearFlag(%q) returned error %v", in, err)
		}
		if got != want[i] {
			t.Errorf("validateYearFlag(%q) = %q, want %q", in, got, want[i])
		}
	}
	for _, in := range []string{"54", "19540", "nineteen", "1954-09", "1700"} {
		if _, err := validateYearFlag(in); err == nil {
			t.Errorf("validateYearFlag(%q) accepted an invalid year", in)
		}
	}
}
