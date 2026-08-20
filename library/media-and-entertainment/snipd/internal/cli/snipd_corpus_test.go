// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/store"
)

// fixtureStore builds a small hermetic corpus in a temp SQLite mirror, matching
// how `pull` writes: typed snip/episode JSON into the generic resources table.
func fixtureStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snipd.sqlite")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ep := snipd.Episode{EpisodeID: "e1", Title: "Ep One", Show: "Alpha Show", PublishDate: "2026-01-15"}
	raw, _ := json.Marshal(ep)
	if err := st.Upsert("episodes", ep.EpisodeID, raw); err != nil {
		t.Fatalf("upsert episode: %v", err)
	}

	snips := []snipd.Snip{
		{SnipID: "s1", EpisodeID: "e1", Show: "Alpha Show", EpisodeTitle: "Ep One",
			Favorite: true, Title: "Thinking partner", Note: "AI as a junior partner",
			Quote: "junior thought partner", Speaker: "Ada", Transcript: "long transcript one",
			Start: "01:00", Tags: []string{"ai", "ux"}, URL: "https://share.snipd.com/snip/s1"},
		{SnipID: "s2", EpisodeID: "e1", Show: "Alpha Show", EpisodeTitle: "Ep One",
			Favorite: false, Title: "Orchestration", Note: "stitching services together",
			Quote: "", Transcript: "long transcript two",
			Start: "02:00", Tags: []string{"ux"}, URL: "https://share.snipd.com/snip/s2"},
		{SnipID: "s3", EpisodeID: "e2", Show: "Beta Show", EpisodeTitle: "Ep Two",
			Favorite: false, Title: "Personas", Note: "archetypes vs personas",
			Quote: "personas are lies", Speaker: "Grace", Transcript: "long transcript three",
			Start: "03:00", Tags: []string{"research"}, URL: "https://share.snipd.com/snip/s3"},
	}
	for _, s := range snips {
		raw, _ := json.Marshal(s)
		if err := st.Upsert("snips", s.SnipID, raw); err != nil {
			t.Fatalf("upsert snip %s: %v", s.SnipID, err)
		}
	}
	return st, dbPath
}

func TestQuerySnipsShowFilter(t *testing.T) {
	st, _ := fixtureStore(t)
	defer st.Close()

	got, err := querySnips(context.Background(), st, "json_extract(data,'$.show') = ?", "", []any{"Alpha Show"}, 0)
	if err != nil {
		t.Fatalf("querySnips: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Alpha Show snips = %d, want 2", len(got))
	}

	fav, err := querySnips(context.Background(), st, "json_extract(data,'$.favorite') = 1", "", nil, 0)
	if err != nil {
		t.Fatalf("querySnips favorite: %v", err)
	}
	if len(fav) != 1 || fav[0].SnipID != "s1" {
		t.Fatalf("favorite snips = %+v, want just s1", fav)
	}
}

func TestFTSSearchRanksAndScopes(t *testing.T) {
	st, _ := fixtureStore(t)
	defer st.Close()

	raws, err := st.Search("partner", 10, "snips")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	snips := unmarshalSnips(raws)
	if len(snips) == 0 || snips[0].SnipID != "s1" {
		t.Fatalf("FTS 'partner' top hit = %+v, want s1 first", snips)
	}
	// porter stemming: "orchestration" should still match the stemmed root.
	raws, _ = st.Search("orchestration", 10, "snips")
	if len(unmarshalSnips(raws)) == 0 {
		t.Errorf("FTS 'orchestration' returned nothing")
	}
}

func TestQuoteFilterDropsQuotelessSnips(t *testing.T) {
	st, _ := fixtureStore(t)
	defer st.Close()

	// s2 has no quote; a broad search must not surface it as a quote row.
	raws, _ := st.Search("services", 10, "snips") // matches s2's note
	snips := unmarshalSnips(raws)
	withQuote := 0
	for _, s := range snips {
		if strings.TrimSpace(s.Quote) != "" {
			withQuote++
		}
	}
	if withQuote != 0 {
		t.Errorf("expected the quoteless match to be droppable; got %d quoted", withQuote)
	}
}

func TestAggregateBySQL(t *testing.T) {
	st, _ := fixtureStore(t)
	defer st.Close()

	rows, err := st.DB().Query(aggDimensions["by-show"])
	if err != nil {
		t.Fatalf("by-show: %v", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		counts[k] = n
	}
	if counts["Alpha Show"] != 2 || counts["Beta Show"] != 1 {
		t.Fatalf("by-show counts = %v, want Alpha 2 / Beta 1", counts)
	}

	// by-tag expands the JSON tags array via json_each.
	trows, err := st.DB().Query(aggDimensions["by-tag"])
	if err != nil {
		t.Fatalf("by-tag: %v", err)
	}
	defer trows.Close()
	tagCounts := map[string]int{}
	for trows.Next() {
		var k string
		var n int
		if err := trows.Scan(&k, &n); err != nil {
			t.Fatalf("scan tag: %v", err)
		}
		tagCounts[k] = n
	}
	if tagCounts["ux"] != 2 || tagCounts["ai"] != 1 || tagCounts["research"] != 1 {
		t.Fatalf("by-tag counts = %v, want ux 2 / ai 1 / research 1", tagCounts)
	}
}
