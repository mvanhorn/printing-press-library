package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// sabrinaMatches mirrors the live /search/movie ordering for the query that
// motivated this fix. Note what the numbers say: the 1954 original leads the
// 1995 remake on BOTH vote_count (1373 vs 703) and popularity (4.24 vs 3.59),
// and TMDb still returns it second — the ordering is a relevance ranking we
// cannot read, not either exposed field.
func sabrinaMatches() []tmdbSearchResult {
	return []tmdbSearchResult{
		{ID: 11860, Title: "Sabrina", ReleaseDate: "1995-12-15", VoteCount: 703, Popularity: 3.59},
		{ID: 6620, Title: "Sabrina", ReleaseDate: "1954-09-22", VoteCount: 1373, Popularity: 4.24},
		{ID: 503902, Title: "Sabrina", ReleaseDate: "2018-01-01", VoteCount: 194, Popularity: 1.83},
		{ID: 708143, Title: "Sabrina", ReleaseDate: "2011-01-01", VoteCount: 0},
	}
}

func TestNoteAmbiguityRecordsForJSON(t *testing.T) {
	flags := &rootFlags{}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "--year")

	if len(flags.ambiguities) != 1 {
		t.Fatalf("recorded %d ambiguities, want 1", len(flags.ambiguities))
	}
	rec := flags.ambiguities[0]
	if rec.Query != "Sabrina" || rec.Kind != "titles" {
		t.Errorf("query/kind = %q/%q, want Sabrina/titles", rec.Query, rec.Kind)
	}
	if rec.Signal != signalBetterRated {
		t.Errorf("signal = %q, want %q — the chosen entry has fewer ratings than an alternative", rec.Signal, signalBetterRated)
	}
	if rec.MatchCount != 3 {
		t.Errorf("match_count = %d, want 3 (chosen + 2 notable alternatives)", rec.MatchCount)
	}
	if rec.Chosen.TMDBID != 11860 || rec.Chosen.Year != "1995" || rec.Chosen.VoteCount != 703 {
		t.Errorf("chosen = %+v, want the 1995 entry with its vote count", rec.Chosen)
	}
	if len(rec.Alternatives) != 2 || rec.Alternatives[0].TMDBID != 6620 {
		t.Fatalf("alternatives = %+v, want 6620 first", rec.Alternatives)
	}
	if !strings.Contains(rec.Hint, "--year") {
		t.Errorf("hint = %q, want it to name the --year flag the command exposes", rec.Hint)
	}
}

func TestNoteAmbiguityBetterRatedAlternativeNotFirst(t *testing.T) {
	// Relevance order can bury the best-rated alternative behind a weaker
	// one. The signal must compare the chosen entry against the strongest
	// alternative, not whichever TMDb happened to rank first — otherwise
	// this case reports multiple_exact_matches and the stderr notice skips
	// the better-rated warning entirely.
	flags := &rootFlags{}
	matches := []tmdbSearchResult{
		{ID: 1, Title: "Solaris", ReleaseDate: "2002-11-27", VoteCount: 1000},
		{ID: 2, Title: "Solaris", ReleaseDate: "1968-10-08", VoteCount: 500},
		{ID: 3, Title: "Solaris", ReleaseDate: "1972-03-20", VoteCount: 9000},
	}
	noteAmbiguity(flags, "titles", "Solaris", matches, notableVoteFloor, "--year")

	if len(flags.ambiguities) != 1 {
		t.Fatalf("recorded %d ambiguities, want 1", len(flags.ambiguities))
	}
	rec := flags.ambiguities[0]
	if rec.Signal != signalBetterRated {
		t.Errorf("signal = %q, want %q — the 1972 entry outranks the chosen one on votes even though it is listed last", rec.Signal, signalBetterRated)
	}
	if len(rec.Alternatives) != 2 || rec.Alternatives[0].TMDBID != 3 {
		t.Fatalf("alternatives = %+v, want the best-rated entry (3) moved to the front", rec.Alternatives)
	}
}

func TestNoteAmbiguitySilentWhenNoNotableAlternatives(t *testing.T) {
	flags := &rootFlags{}
	inception := []tmdbSearchResult{
		{ID: 27205, Title: "Inception", ReleaseDate: "2010-07-15", VoteCount: 39638},
		{ID: 1359046, Title: "Inception", ReleaseDate: "1980-01-01", VoteCount: 0},
	}
	noteAmbiguity(flags, "titles", "Inception", inception, notableVoteFloor, "--year")
	if len(flags.ambiguities) != 0 {
		t.Errorf("recorded %d ambiguities for an unambiguous title, want 0 — the JSON field must not appear when the stderr notice doesn't fire", len(flags.ambiguities))
	}
}

func TestNoteAmbiguityQuietStillRecords(t *testing.T) {
	// --quiet is about terminal chatter; the JSON record is a fact about how
	// the result was resolved and has a different audience.
	flags := &rootFlags{quiet: true}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "--year")
	if len(flags.ambiguities) != 1 {
		t.Fatalf("--quiet suppressed the JSON record (%d recorded), want 1", len(flags.ambiguities))
	}
}

func TestNoteAmbiguityHintWithoutYearFlag(t *testing.T) {
	flags := &rootFlags{}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "")
	hint := flags.ambiguities[0].Hint
	if strings.Contains(hint, "--year") {
		t.Errorf("hint = %q, must not offer --year to a command that has no such flag", hint)
	}
	if !strings.Contains(hint, "(YYYY)") {
		t.Errorf("hint = %q, want the inline-year form offered instead", hint)
	}
}

func TestNoteAmbiguityPersonSignal(t *testing.T) {
	flags := &rootFlags{}
	people := []tmdbSearchResult{
		{ID: 1405209, Name: "David Jones", Popularity: 0.36, KnownFor: "Visual Effects"},
		{ID: 52784, Name: "David Jones", Popularity: 0.25, KnownFor: "Acting"},
	}
	noteAmbiguity(flags, "people", "David Jones", people, 0, "")
	if len(flags.ambiguities) != 1 {
		t.Fatalf("recorded %d ambiguities for namesakes, want 1", len(flags.ambiguities))
	}
	rec := flags.ambiguities[0]
	if rec.Signal != signalMultipleMatches {
		t.Errorf("signal = %q, want %q — people carry no vote counts to compare", rec.Signal, signalMultipleMatches)
	}
	if rec.Chosen.KnownFor != "Visual Effects" || rec.Alternatives[0].KnownFor != "Acting" {
		t.Errorf("known_for missing from the person record: %+v / %+v", rec.Chosen, rec.Alternatives[0])
	}
}

func TestPrintAmbiguityNotice(t *testing.T) {
	matches := sabrinaMatches()
	alts := notableAlternatives(matches, notableVoteFloor)
	var buf bytes.Buffer
	printAmbiguityNotice(&buf, "titles", "Sabrina", matches[0], alts, signalBetterRated, "Disambiguate with --year <YYYY>.")
	out := buf.String()
	for _, want := range []string{"matches 3 titles", "using id 11860", "search relevance put it first", "has more ratings (1373 vs 703)", "6620", "--year"} {
		if !strings.Contains(out, want) {
			t.Errorf("notice missing %q:\n%s", want, out)
		}
	}
	// The notice must not blame the popularity field: in this very case the
	// chosen entry is the LESS popular one, so the claim would be false.
	if strings.Contains(out, "by popularity") {
		t.Errorf("notice attributes the ordering to the popularity field, which the data contradicts:\n%s", out)
	}
}

func TestPrintAmbiguityNoticeTruncatesLongLists(t *testing.T) {
	matches := []tmdbSearchResult{{ID: 1, Title: "Foo", VoteCount: 100}}
	for i := 0; i < maxListedAlternatives+3; i++ {
		matches = append(matches, tmdbSearchResult{ID: 100 + i, Title: "Foo", VoteCount: 60})
	}
	alts := notableAlternatives(matches, notableVoteFloor)
	var buf bytes.Buffer
	printAmbiguityNotice(&buf, "titles", "Foo", matches[0], alts, signalMultipleMatches, "hint")
	if !strings.Contains(buf.String(), "and 3 more") {
		t.Errorf("long alternative list not truncated:\n%s", buf.String())
	}
}

func TestInjectAmbiguityMeta(t *testing.T) {
	doc := json.RawMessage(`{"title":"Sabrina","ratings":{"imdb":"6.3"}}`)

	// No recorded ambiguity — the document must come back byte-identical.
	if got := injectAmbiguityMeta(doc, &rootFlags{}); string(got) != string(doc) {
		t.Errorf("injected into a clean run: %s", got)
	}
	if got := injectAmbiguityMeta(doc, nil); string(got) != string(doc) {
		t.Errorf("nil flags mutated the document: %s", got)
	}

	flags := &rootFlags{}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "--year")

	var out map[string]json.RawMessage
	if err := json.Unmarshal(injectAmbiguityMeta(doc, flags), &out); err != nil {
		t.Fatalf("injected document is not valid JSON: %v", err)
	}
	// Additive: pre-existing keys survive untouched.
	if string(out["title"]) != `"Sabrina"` {
		t.Errorf("title was altered: %s", out["title"])
	}
	if string(out["ratings"]) != `{"imdb":"6.3"}` {
		t.Errorf("ratings was altered: %s", out["ratings"])
	}
	var meta struct {
		Ambiguous []ambiguityMeta `json:"ambiguous"`
	}
	if err := json.Unmarshal(out["meta"], &meta); err != nil {
		t.Fatalf("meta is not an object: %v", err)
	}
	if len(meta.Ambiguous) != 1 || meta.Ambiguous[0].Chosen.TMDBID != 11860 {
		t.Errorf("meta.ambiguous = %+v", meta.Ambiguous)
	}
}

func TestInjectAmbiguityMetaMergesExistingMeta(t *testing.T) {
	flags := &rootFlags{}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "--year")

	// The provenance envelope already owns meta.source — merge, don't clobber.
	doc := json.RawMessage(`{"results":[],"meta":{"source":"live"}}`)
	var out struct {
		Meta struct {
			Source    string          `json:"source"`
			Ambiguous []ambiguityMeta `json:"ambiguous"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(injectAmbiguityMeta(doc, flags), &out); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if out.Meta.Source != "live" {
		t.Errorf("meta.source was clobbered: %q", out.Meta.Source)
	}
	if len(out.Meta.Ambiguous) != 1 {
		t.Errorf("meta.ambiguous missing after merge: %+v", out.Meta)
	}
}

func TestInjectAmbiguityMetaLeavesNonObjectsAlone(t *testing.T) {
	flags := &rootFlags{}
	noteAmbiguity(flags, "titles", "Sabrina", sabrinaMatches(), notableVoteFloor, "--year")

	for _, doc := range []string{`[{"id":1}]`, `"plain text"`, `not json at all`} {
		if got := injectAmbiguityMeta(json.RawMessage(doc), flags); string(got) != doc {
			t.Errorf("injectAmbiguityMeta(%s) = %s, want unchanged — there is no object to add a key to", doc, got)
		}
	}

	// A meta field that isn't an object belongs to someone else; don't touch it.
	weird := `{"title":"x","meta":"already a string"}`
	if got := injectAmbiguityMeta(json.RawMessage(weird), flags); string(got) != weird {
		t.Errorf("clobbered a non-object meta: %s", got)
	}
}

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
	// Real TMDb vote counts, 2026-07: search returned the remake ahead of the
	// better-rated original, and a long tail of unrated entries shares the name.
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
