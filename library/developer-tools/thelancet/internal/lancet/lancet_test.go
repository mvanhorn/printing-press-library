package lancet

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

// stubFetcher returns a fixed OpenAlex payload for CurateLive / Refresh tests.
type stubFetcher struct {
	payload json.RawMessage
}

func (s stubFetcher) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return s.payload, nil
}

func TestLookup(t *testing.T) {
	cases := []struct {
		in      string
		wantLen int
		wantOK  bool
	}{
		{"", 1, true},          // empty defaults to flagship
		{"lancet", 1, true},    // flagship
		{"lancet-oncology", 1, true},
		{"0140-6736", 1, true}, // by ISSN
		{"all", len(journals), true},
		{"family", len(journals), true},
		{"not-a-journal", 0, false},
	}
	for _, c := range cases {
		got, ok := Lookup(c.in)
		if ok != c.wantOK {
			t.Errorf("Lookup(%q) ok = %v, want %v", c.in, ok, c.wantOK)
		}
		if len(got) != c.wantLen {
			t.Errorf("Lookup(%q) len = %d, want %d", c.in, len(got), c.wantLen)
		}
	}
}

func TestNameForISSN(t *testing.T) {
	if got := NameForISSN("0140-6736"); got != "The Lancet" {
		t.Errorf("NameForISSN flagship = %q, want The Lancet", got)
	}
	if got := NameForISSN("9999-9999"); got != "9999-9999" {
		t.Errorf("NameForISSN unknown = %q, want passthrough", got)
	}
}

func TestShortID(t *testing.T) {
	cases := map[string]string{
		"https://openalex.org/W123": "W123",
		"https://openalex.org/A456": "A456",
		"W789":                      "W789",
		"":                          "",
	}
	for in, want := range cases {
		if got := shortID(in); got != want {
			t.Errorf("shortID(%q) = %q, want %q", in, got, want)
		}
	}
}

// newTestDB builds an in-memory store seeded with a couple of works so the
// query helpers have data to operate on.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	works := []decodedWork{
		{
			ID: "W1", DOI: "10.1/a", Title: "Cancer trial", Year: 2024, Date: "2024-03-01",
			Cited: 100, IsOA: true, Topic: "Oncology",
			Authors: []decodedAuthor{
				{ID: "A1", Name: "Alice", Institutions: []decodedInstitution{{ID: "I1", Name: "Oxford"}}},
				{ID: "A2", Name: "Bob", Institutions: []decodedInstitution{{ID: "I1", Name: "Oxford"}}},
			},
		},
		{
			ID: "W2", DOI: "10.1/b", Title: "Neuro study", Year: 2019, Date: "2019-06-01",
			Cited: 10, IsOA: false, Topic: "Neurology",
			Authors: []decodedAuthor{
				{ID: "A1", Name: "Alice", Institutions: []decodedInstitution{{ID: "I1", Name: "Oxford"}}},
			},
		},
	}
	if _, err := StoreWorks(ctx, db, works, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	return db
}

func TestRankAuthors(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	rows, err := RankAuthors(context.Background(), db, "", "", 10)
	if err != nil {
		t.Fatalf("RankAuthors: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d authors, want 2", len(rows))
	}
	// Alice has two works (100 + 10 = 110), should rank first.
	if rows[0].AuthorName != "Alice" || rows[0].TotalCitations != 110 {
		t.Errorf("top author = %+v, want Alice with 110 citations", rows[0])
	}
}

func TestCoAuthorMesh(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	edges, err := CoAuthorMesh(context.Background(), db, "Oxford", 10)
	if err != nil {
		t.Fatalf("CoAuthorMesh: %v", err)
	}
	if len(edges) != 1 || edges[0].SharedWorks != 1 {
		t.Fatalf("got %+v, want one Alice-Bob pair sharing 1 work", edges)
	}
}

func TestCurate(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	rows, err := Curate(context.Background(), db, "cancer", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Cancer trial" {
		t.Fatalf("got %+v, want the Cancer trial work", rows)
	}
}

func TestCurateDecodesHTMLEntities(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()
	raw := rawWork{
		ID:              "https://openalex.org/W-html",
		DOI:             "https://doi.org/10.1/html",
		Title:           "Bile Acid &amp; Tryptophan Metabolism",
		PublicationYear: 2025,
		PublicationDate: "2025-01-01",
		CitedByCount:    1,
	}
	raw.PrimaryTopic = &struct {
		DisplayName string `json:"display_name"`
	}{DisplayName: "Metabolism &amp; Diet"}
	stored := decodeWork(raw)
	if _, err := StoreWorks(ctx, db, []decodedWork{stored}, "0140-6736", "The Lancet &amp; Infectious Diseases"); err != nil {
		t.Fatalf("store: %v", err)
	}
	rows, err := Curate(ctx, db, "Bile Acid", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Title != "Bile Acid & Tryptophan Metabolism" {
		t.Errorf("Title = %q, want decoded ampersand", rows[0].Title)
	}
	if rows[0].Topic != "Metabolism & Diet" {
		t.Errorf("Topic = %q, want decoded ampersand", rows[0].Topic)
	}
	if rows[0].Journal != "The Lancet & Infectious Diseases" {
		t.Errorf("Journal = %q, want decoded ampersand", rows[0].Journal)
	}
}

func TestCurateMatchesDecodedAmpersandQuery(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()
	raw := rawWork{
		ID:              "https://openalex.org/W-amp-query",
		DOI:             "https://doi.org/10.1/amp-query",
		Title:           "Bile Acid &amp; Tryptophan Metabolism",
		PublicationYear: 2025,
		PublicationDate: "2025-01-01",
		CitedByCount:    1,
	}
	stored := decodeWork(raw)
	if stored.Title != "Bile Acid & Tryptophan Metabolism" {
		t.Fatalf("decodeWork Title = %q, want decoded-once ampersand", stored.Title)
	}
	if _, err := StoreWorks(ctx, db, []decodedWork{stored}, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	rows, err := Curate(ctx, db, "Bile Acid & Tryptophan", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (decoded '&' query should LIKE a title ingested from &amp;)", len(rows))
	}
	if rows[0].Title != "Bile Acid & Tryptophan Metabolism" {
		t.Errorf("Title = %q, want store-decoded title without a second CleanText", rows[0].Title)
	}
}

func TestCurateLiveDecodesHTMLEntities(t *testing.T) {
	ctx := context.Background()
	live, err := CurateLive(ctx, stubFetcher{payload: curateLivePayload(
		"Bile Acid &amp; Tryptophan Metabolism",
		"10.1/html-live",
		"Metabolism &amp; Diet",
		"The Lancet &amp; Infectious Diseases",
	)}, "Bile Acid", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("CurateLive: %v", err)
	}
	if len(live) != 1 {
		t.Fatalf("got %d rows, want 1", len(live))
	}
	if live[0].Title != "Bile Acid & Tryptophan Metabolism" {
		t.Errorf("Title = %q, want decoded ampersand", live[0].Title)
	}
	if live[0].Topic != "Metabolism & Diet" {
		t.Errorf("Topic = %q, want decoded ampersand", live[0].Topic)
	}
	if live[0].Journal != "The Lancet & Infectious Diseases" {
		t.Errorf("Journal = %q, want decoded ampersand", live[0].Journal)
	}
}

func TestCurateAndCurateLiveMatchOnNestedEntities(t *testing.T) {
	const nested = "A &amp;amp; B"
	const want = "A &amp; B"
	ctx := context.Background()

	raw := rawWork{
		ID:              "https://openalex.org/W-nested",
		DOI:             "https://doi.org/10.1/nested",
		Title:           nested,
		PublicationYear: 2025,
		PublicationDate: "2025-01-01",
		CitedByCount:    1,
	}
	raw.PrimaryTopic = &struct {
		DisplayName string `json:"display_name"`
	}{DisplayName: nested}
	stored := decodeWork(raw)
	if stored.Title != want || stored.Topic != want {
		t.Fatalf("decodeWork Title=%q Topic=%q, want one-pass %q", stored.Title, stored.Topic, want)
	}

	db := newTestDB(t)
	defer db.Close()
	if _, err := StoreWorks(ctx, db, []decodedWork{stored}, "0140-6736", nested); err != nil {
		t.Fatalf("store: %v", err)
	}
	storeRows, err := Curate(ctx, db, "A ", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if len(storeRows) != 1 {
		t.Fatalf("Curate got %d rows, want 1", len(storeRows))
	}

	liveRows, err := CurateLive(ctx, stubFetcher{payload: curateLivePayload(nested, "10.1/nested", nested, nested)}, "A", "", "citations", false, 10)
	if err != nil {
		t.Fatalf("CurateLive: %v", err)
	}
	if len(liveRows) != 1 {
		t.Fatalf("CurateLive got %d rows, want 1", len(liveRows))
	}

	if storeRows[0].Title != want {
		t.Errorf("store Title = %q, want one-pass %q", storeRows[0].Title, want)
	}
	if liveRows[0].Title != want {
		t.Errorf("live Title = %q, want one-pass %q", liveRows[0].Title, want)
	}
	if storeRows[0].Title != liveRows[0].Title {
		t.Errorf("Title diverged: store %q vs live %q", storeRows[0].Title, liveRows[0].Title)
	}
	if storeRows[0].Topic != liveRows[0].Topic {
		t.Errorf("Topic diverged: store %q vs live %q", storeRows[0].Topic, liveRows[0].Topic)
	}
	if storeRows[0].Journal != liveRows[0].Journal {
		t.Errorf("Journal diverged: store %q vs live %q", storeRows[0].Journal, liveRows[0].Journal)
	}
	if storeRows[0].Topic != want || storeRows[0].Journal != want {
		t.Errorf("store Topic/Journal = %q/%q, want one-pass %q", storeRows[0].Topic, storeRows[0].Journal, want)
	}
}

func curateLivePayload(title, doi, topic, journal string) json.RawMessage {
	type source struct {
		DisplayName string `json:"display_name"`
	}
	type location struct {
		Source *source `json:"source"`
	}
	type topicRow struct {
		DisplayName string `json:"display_name"`
	}
	type result struct {
		DOI             string    `json:"doi"`
		Title           string    `json:"title"`
		PublicationYear int       `json:"publication_year"`
		CitedByCount    int       `json:"cited_by_count"`
		PrimaryTopic    *topicRow `json:"primary_topic"`
		PrimaryLocation *location `json:"primary_location"`
	}
	body, err := json.Marshal(struct {
		Results []result `json:"results"`
	}{Results: []result{{
		DOI:             "https://doi.org/" + doi,
		Title:           title,
		PublicationYear: 2025,
		CitedByCount:    1,
		PrimaryTopic:    &topicRow{DisplayName: topic},
		PrimaryLocation: &location{Source: &source{DisplayName: journal}},
	}}})
	if err != nil {
		panic(err)
	}
	return body
}

func TestTopicDrift(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	rows, err := TopicDrift(context.Background(), db, "", 2018, 2020, 2023, 2025, 10)
	if err != nil {
		t.Fatalf("TopicDrift: %v", err)
	}
	// Neurology only in window1, Oncology only in window2.
	if len(rows) != 2 {
		t.Fatalf("got %d topic shifts, want 2", len(rows))
	}
}
