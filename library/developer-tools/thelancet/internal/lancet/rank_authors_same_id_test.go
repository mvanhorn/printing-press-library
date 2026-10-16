package lancet

import (
	"context"
	"testing"
)

// One OpenAlex author ID printed under two names must be ONE row.
func TestRankAuthorsSameIDOneRow(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()
	works := []decodedWork{
		{ID: "W91", DOI: "10.9/x1", Title: "T1", Year: 2020, Date: "2020-01-01", Cited: 100, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "A9", Name: "Walter Churchill Willett"}}},
		{ID: "W92", DOI: "10.9/x2", Title: "T2", Year: 2021, Date: "2021-01-01", Cited: 10, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "A9", Name: "Walter C. Willett"}}},
		{ID: "W93", DOI: "10.9/x3", Title: "T3", Year: 2022, Date: "2022-01-01", Cited: 5, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "A9", Name: "Walter C. Willett"}}},
	}
	if _, err := StoreWorks(ctx, db, works, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	rows, err := RankAuthors(ctx, db, "", "", 100)
	if err != nil {
		t.Fatalf("RankAuthors: %v", err)
	}
	var found []AuthorRank
	for _, r := range rows {
		if r.AuthorID == "A9" {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("author A9 has %d rows, want 1: %+v", len(found), found)
	}
	if found[0].Works != 3 || found[0].TotalCitations != 115 {
		t.Errorf("A9 = %+v, want works 3 and 115 citations", found[0])
	}
	if found[0].AuthorName != "Walter C. Willett" {
		t.Errorf("A9 name = %q, want the most frequent form \"Walter C. Willett\"", found[0].AuthorName)
	}
}

// Two different IDs with the same name are two people and stay two rows.
func TestRankAuthorsDifferentIDsStaySeparate(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()
	works := []decodedWork{
		{ID: "W94", DOI: "10.9/y1", Title: "U1", Year: 2020, Date: "2020-01-01", Cited: 50, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "B1", Name: "Frank B. Hu"}}},
		{ID: "W95", DOI: "10.9/y2", Title: "U2", Year: 2021, Date: "2021-01-01", Cited: 40, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "B2", Name: "Frank B. Hu"}}},
	}
	if _, err := StoreWorks(ctx, db, works, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	rows, err := RankAuthors(ctx, db, "", "", 100)
	if err != nil {
		t.Fatalf("RankAuthors: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.AuthorName == "Frank B. Hu" {
			seen[r.AuthorID] = true
		}
	}
	if !seen["B1"] || !seen["B2"] {
		t.Errorf("Frank B. Hu rows = %v, want both B1 and B2", seen)
	}
}

// rankedNameFor stores works and returns the single display name RankAuthors gives authorID.
func rankedNameFor(t *testing.T, works []decodedWork, authorID string) string {
	t.Helper()
	db := newTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := StoreWorks(ctx, db, works, "0140-6736", "The Lancet"); err != nil {
		t.Fatalf("store: %v", err)
	}
	rows, err := RankAuthors(ctx, db, "", "", 100)
	if err != nil {
		t.Fatalf("RankAuthors: %v", err)
	}
	var names []string
	for _, r := range rows {
		if r.AuthorID == authorID {
			names = append(names, r.AuthorName)
		}
	}
	if len(names) != 1 {
		t.Fatalf("author %s has %d rows, want 1: %v", authorID, len(names), names)
	}
	return names[0]
}

// Equal frequency: the longer spelling wins, even when it sorts later alphabetically.
func TestRankAuthorsTieLongestNameWins(t *testing.T) {
	works := []decodedWork{
		{ID: "W96", DOI: "10.9/z1", Title: "V1", Year: 2020, Date: "2020-01-01", Cited: 30, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "C1", Name: "Ann Lee"}}},
		{ID: "W97", DOI: "10.9/z2", Title: "V2", Year: 2021, Date: "2021-01-01", Cited: 20, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "C1", Name: "Ann M. Lee"}}},
	}
	if got := rankedNameFor(t, works, "C1"); got != "Ann M. Lee" {
		t.Errorf("C1 name = %q, want the longer form \"Ann M. Lee\" on a frequency tie", got)
	}
}

// Equal frequency and equal length: the alphabetically first spelling wins.
func TestRankAuthorsTieAlphabeticalName(t *testing.T) {
	works := []decodedWork{
		{ID: "W98", DOI: "10.9/z3", Title: "V3", Year: 2020, Date: "2020-01-01", Cited: 30, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "D1", Name: "Bob Fox"}}},
		{ID: "W99", DOI: "10.9/z4", Title: "V4", Year: 2021, Date: "2021-01-01", Cited: 20, Topic: "Topic",
			Authors: []decodedAuthor{{ID: "D1", Name: "Abe Fox"}}},
	}
	if got := rankedNameFor(t, works, "D1"); got != "Abe Fox" {
		t.Errorf("D1 name = %q, want the alphabetically first \"Abe Fox\" on a frequency and length tie", got)
	}
}
