// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"strconv"
)

// TestNovelConnectionsHelpWires smoke-tests that the connections command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelConnectionsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"connections", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("connections --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "connections"} {
		if !strings.Contains(help, want) {
			t.Fatalf("connections --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelExtractTagsAndTerms(t *testing.T) {
	tags := novelExtractTags(`{"tags":{"machine-learning":{},"AI":{}}}`)
	if len(tags) != 2 || tags[0] != "ai" || tags[1] != "machine-learning" {
		t.Fatalf("tags = %#v, want sorted normalized keys", tags)
	}
	terms := novelTokens("The Practical Guide to Retrieval", "Retrieval systems and ranking")
	if len(terms) == 0 || terms[0] != "guide" {
		t.Fatalf("terms = %#v, want stopwords removed and sorted", terms)
	}
}

func TestNovelConnectionMatchesByOverlap(t *testing.T) {
	seed := novelConnectionDoc{ID: "seed", Title: "Retrieval Ranking", Author: "Ada", Tags: []string{"ai"}}
	docs := []novelConnectionDoc{
		seed,
		{ID: "a", Title: "Retrieval Notes", Author: "Ada", Tags: []string{"ai"}},
		{ID: "b", Title: "Cooking", Author: "Other", Tags: []string{"food"}},
	}
	matches := novelConnectionMatches(seed, docs, map[string]bool{"tags": true, "author": true, "terms": true}, 3)
	if len(matches) != 1 || matches[0].ID != "a" || matches[0].Score < 3 {
		t.Fatalf("matches = %+v, want doc a with overlap", matches)
	}
}

func novelLibraryWideFixture() []novelConnectionDoc {
	summaries := []string{
		"cartography compass", "fermentation starter", "typography baseline", "security sandbox",
		"garden seedlings", "budget runway", "latency metrics", "recipe garnish",
		"storage queues", "facilitation agenda", "navigation waypoint", "archive catalog",
	}
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Vector Orchard", Summary: "calibration", Author: "Ada"},
		{ID: "b", Title: "Vector Harbor", Summary: "planning", Author: "Ada"},
	}
	for i, summary := range summaries {
		docs = append(docs, novelConnectionDoc{
			ID:      "filler-" + strconv.Itoa(i),
			Title:   "Filler " + strconv.Itoa(i),
			Summary: summary,
			Author:  "Author " + strconv.Itoa(i),
		})
	}
	return docs
}

func TestNovelConnectionsLibraryWidePairs(t *testing.T) {
	pairs, notes, hidden := novelConnectionsLibraryWide(novelLibraryWideFixture(), map[string]bool{"author": true, "terms": true}, 3, 20)
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if hidden != 0 {
		t.Fatalf("hidden = %d, want none", hidden)
	}
	if len(pairs) == 0 {
		t.Fatal("pairs empty, want shared low-DF term/author pair")
	}
	got := pairs[0]
	if got.A.ID != "a" || got.B.ID != "b" || got.Score != 3 || got.Overlap != 3 {
		t.Fatalf("top pair = %+v, want a/b score 3", got)
	}
	if !got.Shared.Author || len(got.Shared.Terms) != 1 || got.Shared.Terms[0] != "vector" {
		t.Fatalf("shared = %+v, want author and vector term", got.Shared)
	}
}

func TestNovelConnectionsLibraryWideStopwordAndDFOnlyNoPair(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "They are going through their day", Tags: []string{"common"}},
		{ID: "b", Title: "They are going through their day", Tags: []string{"common"}},
		{ID: "c", Title: "Cartography Compass", Tags: []string{"common"}},
		{ID: "d", Title: "Fermentation Starter", Tags: []string{"common"}},
	}
	for i := 0; i < 17; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Unique " + strconv.Itoa(i)})
	}
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true, "terms": true}, 1, 20)
	if len(pairs) != 0 {
		t.Fatalf("pairs = %+v, want no stopword/common-tag pair", pairs)
	}
}

func TestNovelConnectionsLibraryWideTagAuthorDFCutoff(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Alpha Root", Author: "Common", Tags: []string{"junk"}},
		{ID: "b", Title: "Beta Branch", Author: "Common", Tags: []string{"junk"}},
		{ID: "c", Title: "Gamma Leaf", Author: "Common", Tags: []string{"junk"}},
		{ID: "d", Title: "Delta Stone", Author: "Common", Tags: []string{"junk"}},
	}
	for i := 0; i < 17; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Filler " + strconv.Itoa(i), Author: "Author " + strconv.Itoa(i)})
	}
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true, "author": true}, 2, 20)
	if len(pairs) != 0 {
		t.Fatalf("pairs = %+v, want common tag/author dropped by DF cutoff", pairs)
	}
}

func TestNovelConnectionsLibraryWideTagDFCutoffBoundarySurvives(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Alpha Root", Tags: []string{"boundary"}},
		{ID: "b", Title: "Beta Branch", Tags: []string{"boundary"}},
		{ID: "c", Title: "Gamma Leaf", Tags: []string{"boundary"}},
	}
	for i := 0; i < 17; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Filler " + strconv.Itoa(i)})
	}
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true}, 2, 20)
	if len(pairs) == 0 {
		t.Fatal("pairs empty, want tag at df/total == 0.15 to survive")
	}
}

func TestNovelConnectionsLibraryWideInvertedIndexCandidates(t *testing.T) {
	docs := append(novelLibraryWideFixture(), novelConnectionDoc{ID: "z", Title: "Completely Different", Summary: "solitude"})
	scored := map[string]bool{}
	oldHook := novelLibraryWideScoreCandidateHook
	novelLibraryWideScoreCandidateHook = func(a, b novelConnectionDoc) {
		scored[a.ID+"\x00"+b.ID] = true
	}
	defer func() { novelLibraryWideScoreCandidateHook = oldHook }()
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"terms": true}, 1, 20)
	if len(pairs) == 0 || pairs[0].A.ID != "a" || pairs[0].B.ID != "b" {
		t.Fatalf("pairs = %+v, want rare-term a/b pair", pairs)
	}
	if scored["a\x00z"] || scored["b\x00z"] {
		t.Fatalf("scored = %#v, want non-overlapping z never scored", scored)
	}
}

func TestNovelConnectionsLibraryWideSkipSignalNote(t *testing.T) {
	oldCap := novelLibraryWideSignalPostingsCap
	novelLibraryWideSignalPostingsCap = 1
	defer func() { novelLibraryWideSignalPostingsCap = oldCap }()
	pairs, notes, hidden := novelConnectionsLibraryWide(novelLibraryWideFixture(), map[string]bool{"terms": true}, 1, 20)
	if len(pairs) != 0 {
		t.Fatalf("pairs = %+v, want skipped signal to remove candidate", pairs)
	}
	if hidden != 0 {
		t.Fatalf("hidden = %d, want none", hidden)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "term:vector") {
		t.Fatalf("notes = %#v, want skipped vector signal", notes)
	}
	note := novelLibraryWideNote(len(novelLibraryWideFixture()), len(novelLibraryWideFixture()), false, notes, 0)
	if !strings.Contains(note, "skipped 1 over-common signal") || strings.Contains(note, "term:") {
		t.Fatalf("note = %q, want humanized skipped signal count", note)
	}
}

func TestNovelConnectionsLibraryWideDeterminism(t *testing.T) {
	docs := novelLibraryWideFixture()
	first, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"author": true, "terms": true}, 1, 20)
	second, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"author": true, "terms": true}, 1, 20)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("first = %s\nsecond = %s\nwant identical ordering", a, b)
	}
}

func TestNovelConnectionsLibraryWidePairUniquenessAcrossSignals(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Vector Systems", Author: "Ada"},
		{ID: "b", Title: "Vector Practice", Author: "Ada"},
	}
	for i := 0; i < 12; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Filler " + strconv.Itoa(i), Author: "Author " + strconv.Itoa(i)})
	}
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"author": true, "terms": true}, 1, 20)
	counts := map[string]int{}
	for _, pair := range pairs {
		counts[pair.A.ID+"-"+pair.B.ID]++
	}
	if counts["a-b"] != 1 {
		t.Fatalf("counts = %#v, want a-b exactly once despite shared term and author", counts)
	}
}

func TestNovelConnectionsLibraryWideRankedOrdering(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Alpha Root", Author: "Ada", Tags: []string{"red", "blue"}},
		{ID: "b", Title: "Beta Branch", Author: "Ada", Tags: []string{"red", "blue"}},
		{ID: "c", Title: "Gamma Leaf", Author: "Grace", Tags: []string{"green"}},
		{ID: "d", Title: "Delta Stone", Author: "Grace", Tags: []string{"green"}},
		{ID: "e", Title: "Epsilon Field", Tags: []string{"gold", "silver"}},
		{ID: "f", Title: "Zeta Harbor", Tags: []string{"gold", "silver"}},
		{ID: "g", Title: "Eta Signal", Tags: []string{"teal"}},
		{ID: "h", Title: "Theta Archive", Tags: []string{"teal"}},
	}
	for i := 0; i < 12; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Filler " + strconv.Itoa(i), Author: "Author " + strconv.Itoa(i)})
	}
	pairs, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true, "author": true}, 2, 20)
	got := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		got = append(got, pair.A.ID+"-"+pair.B.ID)
	}
	want := []string{"a-b", "c-d", "e-f", "g-h"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered pairs = %#v, want %#v", got, want)
	}
	if pairs[1].Score != pairs[2].Score || pairs[0].Score <= pairs[1].Score || pairs[3].Score >= pairs[1].Score {
		t.Fatalf("scores = %+v, want descending scores with c-d/e-f tie", pairs)
	}
}

func TestNovelConnectionsLibraryWideHidesNearDuplicates(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "a", Title: "Same Saved Article", Author: "Ada", URL: "https://example.com/story?utm_source=reader", Tags: []string{"ai"}},
		{ID: "b", Title: "Same Saved Article", Author: "Ada", URL: "http://www.example.com/story/", Tags: []string{"ai"}},
		{ID: "c", Title: "Fresh Connection", Tags: []string{"search"}},
		{ID: "d", Title: "Distant Connection", Tags: []string{"search"}},
	}
	for i := 0; i < 12; i++ {
		docs = append(docs, novelConnectionDoc{ID: "filler-" + strconv.Itoa(i), Title: "Filler " + strconv.Itoa(i)})
	}
	pairs, _, hidden := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true}, 2, 20)
	for _, pair := range pairs {
		if pair.A.ID == "a" && pair.B.ID == "b" {
			t.Fatalf("pairs = %+v, want near-duplicate a/b hidden", pairs)
		}
	}
	if hidden != 1 {
		t.Fatalf("hidden = %d, want 1", hidden)
	}
	note := novelLibraryWideNote(len(docs), len(docs), false, nil, hidden)
	if !strings.Contains(note, "hidden 1 near-duplicate pair (see `dedupe`)") {
		t.Fatalf("note = %q, want near-duplicate count", note)
	}
}

func TestNovelConnectionMatchesWeightedBoundary(t *testing.T) {
	seed := novelConnectionDoc{ID: "seed", Title: "Seed", Author: "Ada", Tags: []string{"ai"}}
	docs := []novelConnectionDoc{
		seed,
		{ID: "at-threshold", Title: "Threshold", Author: "Ada", Tags: []string{"ai"}},
		{ID: "below-threshold", Title: "Below", Author: "Other", Tags: []string{"ai"}},
	}
	matches := novelConnectionMatches(seed, docs, map[string]bool{"tags": true, "author": true}, 3)
	if len(matches) != 1 || matches[0].ID != "at-threshold" || matches[0].Score != 4 {
		t.Fatalf("matches = %+v, want only score-4 threshold match", matches)
	}
}

func TestNovelConnectionMatchesIgnoresCorpusStopwordTerms(t *testing.T) {
	seed := novelConnectionDoc{ID: "seed", Title: "Platform Signals"}
	docs := []novelConnectionDoc{
		seed,
		{ID: "candidate", Title: "Platform Notes"},
		{ID: "noise-1", Title: "Platform Alpha"},
		{ID: "noise-2", Title: "Platform Beta"},
		{ID: "noise-3", Title: "Platform Gamma"},
		{ID: "noise-4", Title: "Platform Delta"},
		{ID: "noise-5", Title: "Platform Epsilon"},
		{ID: "noise-6", Title: "Platform Zeta"},
	}
	matches := novelConnectionMatches(seed, docs, map[string]bool{"terms": true}, 1)
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, want corpus-common term ignored", matches)
	}
}

func TestNovelTokensDropsNumericTokens(t *testing.T) {
	terms := novelTokens("20260615 retrieval 1234 ranking")
	for _, term := range terms {
		if term == "20260615" || term == "1234" {
			t.Fatalf("terms = %#v, want pure numeric tokens dropped", terms)
		}
	}
}

func TestNovelTokensFiltersStopwords(t *testing.T) {
	terms := novelTokens("they are going through their day", "this is instead of that")
	blocked := map[string]bool{
		"are": true, "going": true, "instead": true, "that": true, "their": true, "they": true, "this": true, "through": true,
	}
	for _, term := range terms {
		if blocked[term] {
			t.Fatalf("terms = %#v, want stopword %q filtered", terms, term)
		}
	}
}

func TestNovelTokensKeepsRealTerms(t *testing.T) {
	terms := novelTokens("leadership negotiation vulnerability homebrew")
	got := map[string]bool{}
	for _, term := range terms {
		got[term] = true
	}
	for _, want := range []string{"leadership", "negotiation", "vulnerability", "homebrew"} {
		if !got[want] {
			t.Fatalf("terms = %#v, want %q kept", terms, want)
		}
	}
}

func TestNovelConnectionsNoStopwordOverlap(t *testing.T) {
	docs := []novelConnectionDoc{
		{ID: "seed", Title: "They are going through their day", Summary: "leadership"},
		{ID: "candidate", Title: "They are going through their day", Summary: "negotiation"},
		{ID: "leadership-peer", Title: "Leadership pattern", Summary: "coaching"},
		{ID: "filler-1", Title: "Cartography archive", Summary: "harbor compass atlas"},
		{ID: "filler-2", Title: "Fermentation notebook", Summary: "starter culture loaf"},
		{ID: "filler-3", Title: "Observability report", Summary: "metrics tracing latency"},
		{ID: "filler-4", Title: "Architecture memo", Summary: "queues storage indexing"},
		{ID: "filler-5", Title: "Gardening calendar", Summary: "mulch compost seedlings"},
		{ID: "filler-6", Title: "Navigation brief", Summary: "waypoint bearing signal"},
		{ID: "filler-7", Title: "Typography notes", Summary: "kerning baseline glyphs"},
		{ID: "filler-8", Title: "Security review", Summary: "threat model sandbox"},
		{ID: "filler-9", Title: "Budget planning", Summary: "forecast margin runway"},
		{ID: "filler-10", Title: "Recipe testing", Summary: "simmer braise garnish"},
		{ID: "filler-11", Title: "Workshop agenda", Summary: "facilitation outcomes"},
	}
	df := novelConnectionTermDF(docs)
	seedTerms := novelSignificantConnectionTerms(docs[0], df, len(docs))
	docTerms := novelSignificantConnectionTerms(docs[1], df, len(docs))
	if shared := novelSharedStrings(seedTerms, docTerms); len(shared) != 0 {
		t.Fatalf("shared terms = %#v, want stopword-only overlap empty", shared)
	}
	peerTerms := novelSignificantConnectionTerms(docs[2], df, len(docs))
	if shared := novelSharedStrings(seedTerms, peerTerms); len(shared) == 0 || shared[0] != "leadership" {
		t.Fatalf("shared terms = %#v, want real low-DF content term connection", shared)
	}
}

func TestNovelConnectionsMissingSeedReturnsEmptyResult(t *testing.T) {
	seedNovelCommandStore(t)

	out := runNovelCommand(t, "connections", "--seed-doc", "missing-seed")
	var view novelConnectionsView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("connections output is not JSON: %v\n%s", err, out)
	}
	if view.Seed["id"] != "missing-seed" || view.Seed["title"] != "" {
		t.Fatalf("seed = %+v, want missing id with empty title", view.Seed)
	}
	if len(view.Matches) != 0 {
		t.Fatalf("matches = %+v, want empty matches", view.Matches)
	}
	if !strings.Contains(view.Note, "seed document missing-seed not found in local store") {
		t.Fatalf("note = %q, want missing-seed local-store note", view.Note)
	}
}

func TestNovelRenderConnectionsMarkdown(t *testing.T) {
	view := novelConnectionsLibraryView{
		Note: "scanned 2 engaged docs",
		Pairs: []novelConnectionPair{
			{
				A:     novelConnectionPairDoc{ID: "a", Title: "Alpha"},
				B:     novelConnectionPairDoc{ID: "b", Title: "Beta"},
				Score: 4,
				Shared: novelConnectionShared{
					Tags: []string{"ai"},
				},
			},
			{
				A:     novelConnectionPairDoc{ID: "c", Title: "Gamma"},
				B:     novelConnectionPairDoc{ID: "d", Title: "Delta"},
				Score: 2,
				Shared: novelConnectionShared{
					Author: true,
				},
			},
		},
	}
	var out bytes.Buffer
	if err := novelRenderConnectionsMarkdown(&out, view); err != nil {
		t.Fatalf("novelRenderConnectionsMarkdown: %v", err)
	}
	rendered := out.String()
	for _, want := range []string{
		"Alpha (a) <-> Beta (b) - score 4",
		"Gamma (c) <-> Delta (d) - score 2",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered markdown = %q, want line %q", rendered, want)
		}
	}
}

func TestNovelLoadEngagedDocsFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := defaultDBPath("readwise-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer db.Close()
	docs := []json.RawMessage{
		json.RawMessage(`{"id":"tagged","title":"Tagged","reading_progress":0,"tags":{"ai":{}}}`),
		json.RawMessage(`{"id":"opened","title":"Opened","reading_progress":0,"last_opened_at":"2026-06-19T12:00:00Z"}`),
		json.RawMessage(`{"id":"progress","title":"Progress","reading_progress":0.2}`),
		json.RawMessage(`{"id":"idle","title":"Idle","reading_progress":0}`),
	}
	if stored, failed, err := db.UpsertBatch("documents", docs); err != nil {
		t.Fatalf("UpsertBatch documents: %v", err)
	} else if stored != len(docs) || failed != 0 {
		t.Fatalf("UpsertBatch stored=%d failed=%d", stored, failed)
	}
	engaged, total, err := novelLoadEngagedDocs(db, false)
	if err != nil {
		t.Fatalf("novelLoadEngagedDocs default: %v", err)
	}
	if total != 4 || len(engaged) != 3 {
		t.Fatalf("default total=%d engaged=%d docs=%+v, want total 4 engaged 3", total, len(engaged), engaged)
	}
	all, total, err := novelLoadEngagedDocs(db, true)
	if err != nil {
		t.Fatalf("novelLoadEngagedDocs all: %v", err)
	}
	if total != 4 || len(all) != 4 {
		t.Fatalf("all total=%d docs=%d, want 4/4", total, len(all))
	}
}

func TestNovelConnectionsDispatchUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "connections"},
		{"--json", "connections", "--seed-doc", "doc-1", "--library-wide"},
		{"--json", "connections", "--seed-doc", "doc-1", "--all"},
	} {
		var flags rootFlags
		cmd := newRootCmd(&flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("%v succeeded, want usage error", args)
		}
		if !strings.Contains(err.Error(), "connections needs") && !strings.Contains(err.Error(), "choose one") && !strings.Contains(err.Error(), "--all only applies with --library-wide") {
			t.Fatalf("%v error = %v, want connections usage error", args, err)
		}
	}
}

func TestNovelConnectionsBareUsageErrorOmitsUsageBlob(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"connections"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("bare connections succeeded, want usage error")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Fatalf("output = %q, want no Usage blob", out.String())
	}
	if !strings.Contains(err.Error(), "connections needs --seed-doc") {
		t.Fatalf("err = %v, want actionable hint", err)
	}
}

func TestNovelConnectionsLibraryWideOutputShapeAndSeedUnchanged(t *testing.T) {
	seedNovelCommandStore(t)
	seedOut := runNovelCommand(t, "connections", "--seed-doc", "doc-1")
	var seeded map[string]any
	if err := json.Unmarshal([]byte(seedOut), &seeded); err != nil {
		t.Fatalf("seeded output JSON: %v\n%s", err, seedOut)
	}
	if _, ok := seeded["seed"]; !ok {
		t.Fatalf("seeded output = %+v, want seed key", seeded)
	}
	if _, ok := seeded["matches"]; !ok {
		t.Fatalf("seeded output = %+v, want matches key", seeded)
	}
	if _, ok := seeded["pairs"]; ok {
		t.Fatalf("seeded output = %+v, want no pairs key", seeded)
	}

	libraryOut := runNovelCommand(t, "connections", "--library-wide", "--min-overlap", "1")
	var library novelConnectionsLibraryView
	if err := json.Unmarshal([]byte(libraryOut), &library); err != nil {
		t.Fatalf("library output JSON: %v\n%s", err, libraryOut)
	}
	if library.Pairs == nil {
		t.Fatalf("library output = %+v, want pairs key", library)
	}
	if library.Scanned == 0 || !strings.Contains(library.Note, "scanned") {
		t.Fatalf("library output = %+v, want scanned note", library)
	}
}

func TestNovelConnectionsLibraryWideAllScansTotalWithoutRunAllNote(t *testing.T) {
	seedNovelCommandStore(t)
	out := runNovelCommand(t, "connections", "--library-wide", "--all", "--min-overlap", "1")
	var view novelConnectionsLibraryView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("library-wide --all output JSON: %v\n%s", err, out)
	}
	if view.Scanned != 4 {
		t.Fatalf("scanned = %d, want all 4 root documents", view.Scanned)
	}
	if strings.Contains(view.Note, "run --all") {
		t.Fatalf("note = %q, want no run --all clause", view.Note)
	}
}

func TestSliceDConnectionsURLAndSuffixDuplicates(t *testing.T) {
	db := sliceDStore(t,
		"UPDATE documents SET title='Different Article', source_url='https://www.example.com/repeat/?edition=1#top', url='https://reader/one', saved_at='2026-01-01T00:00:00Z' WHERE id='doc-1'",
		"UPDATE documents SET title='Other Article', source_url='http://example.com/repeat?edition=1&utm_source=reader', url='https://reader/two', saved_at='2026-01-01T00:30:00Z' WHERE id='doc-3'",
	)
	docs, _, err := novelLoadEngagedDocs(db, true)
	if err != nil {
		t.Fatal(err)
	}
	var a, b novelConnectionDoc
	for _, doc := range docs {
		if doc.ID == "doc-1" {
			a = doc
		}
		if doc.ID == "doc-3" {
			b = doc
		}
	}
	if a.URL == "" || a.SavedAt.IsZero() || !novelNearDuplicateConnectionDocs(a, b) {
		t.Fatalf("%+v %+v", a, b)
	}
	// Same-title burst saves require a shared author or host.
	for _, suffix := range []string{" | Site", " - Site", " — Site"} {
		a.Title = "Useful Article" + suffix
		b.Title = "Useful Article"
		a.URL = ""
		b.URL = ""
		a.Author = "Shared Author"
		b.Author = "Shared Author"
		if !novelNearDuplicateConnectionDocs(a, b) {
			t.Fatalf("suffix %q", suffix)
		}
	}
	a.Title = "Different Article"
	b.Title = "Other Article"
	if novelNearDuplicateConnectionDocs(a, b) {
		t.Fatal("unrelated burst saves hidden")
	}
	// Pad the corpus so shared signals survive the existing 15% DF cutoff.
	for i := 0; i < 20; i++ {
		docs = append(docs, novelConnectionDoc{ID: fmt.Sprintf("padding-%d", i)})
	}
	pairs, _, hidden := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true, "author": true}, 1, 20)
	if hidden < 1 {
		t.Fatalf("hidden=%d pairs=%+v", hidden, pairs)
	}
	for _, pair := range pairs {
		if pair.A.ID == "doc-1" && pair.B.ID == "doc-3" {
			t.Fatalf("hidden duplicate returned: %+v", pair)
		}
	}
	if !strings.Contains(novelLibraryWideNote(len(docs), len(docs), true, nil, hidden), "hidden") {
		t.Fatal("missing hidden count")
	}
}

func TestE3ConnectionsRelativeCap(t *testing.T) {
	docs := make([]novelConnectionDoc, 100)
	for i := range docs {
		docs[i] = novelConnectionDoc{ID: fmt.Sprint(i), Title: fmt.Sprintf("Unique %d", i)}
		if i < 6 {
			docs[i].Tags = []string{"over_common"}
		}
	}
	pairs, skipped, _ := novelConnectionsLibraryWide(docs, map[string]bool{"tags": true}, 1, 10)
	if len(pairs) != 0 || len(skipped) != 1 {
		t.Fatalf("relative cap: pairs=%d skipped=%v", len(pairs), skipped)
	}
}

func TestE3ConnectionsBoundedHeap(t *testing.T) {
	h := novelPairHeap{}
	for i := 0; i < 1000; i++ {
		h.keep(novelConnectionPair{Score: i, A: novelConnectionPairDoc{ID: fmt.Sprint(i)}}, 3)
		if len(h) > 3 {
			t.Fatal("top K heap exceeded bound")
		}
	}
	if len(h) != 3 || h[0].Score != 997 {
		t.Fatalf("heap=%+v", h)
	}
	docs := novelLibraryWideFixture()
	all, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"author": true, "terms": true}, 1, 100)
	best, _, _ := novelConnectionsLibraryWide(docs, map[string]bool{"author": true, "terms": true}, 1, 1)
	if len(best) != 1 || len(all) == 0 || best[0].A != all[0].A || best[0].B != all[0].B {
		t.Fatalf("best=%v all=%v", best, all)
	}
}

func TestE3ConnectionsAllRequiresBound(t *testing.T) {
	sliceDStore(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"connections", "--library-wide", "--all", "--top", "0"})
	if err := cmd.Execute(); err == nil || ExitCode(err) != 2 {
		t.Fatalf("err=%v", err)
	}
}

func TestE3ConnectionsMeaningfulURLsAndBurst(t *testing.T) {
	if novelConnectionURL("https://youtube.com/watch?v=one&utm_source=x") == novelConnectionURL("https://youtube.com/watch?v=two") {
		t.Fatal("meaningful URL parameters lost")
	}
	a := novelConnectionDoc{Title: "Chapter One - Alpha", URL: "https://one.example/a", SavedAt: time.Now()}
	b := novelConnectionDoc{Title: "Chapter One - Beta", URL: "https://two.example/b", SavedAt: a.SavedAt}
	if novelNearDuplicateConnectionDocs(a, b) {
		t.Fatal("unrelated imports hidden")
	}
	b.URL = "https://one.example/b"
	if !novelNearDuplicateConnectionDocs(a, b) {
		t.Fatal("same-host burst not hidden")
	}
}
