// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package graphql

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// These are LIVE differential tests: they hit Medium's real internal GraphQL
// endpoint and diff the result against the v1 oracle fixtures captured at build
// time. They are gated behind MEDIUM_LIVE=1 and t.Skip otherwise, so the default
// `go test ./...` stays fully offline-green. Run explicitly with:
//
//	MEDIUM_LIVE=1 go test ./internal/source/graphql/ -run Live -v
//
// The contract is "what v2 fetches must be a subset of (or cover) what the v1
// oracle returned" — Medium's ranking shifts over time, so we assert containment
// against the oracle, not equality, exactly as the spec specifies.

// requireLive skips unless MEDIUM_LIVE=1. Keeping it a helper makes the gate
// obvious and uniform across the live tests.
func requireLive(t *testing.T) {
	t.Helper()
	if os.Getenv("MEDIUM_LIVE") != "1" {
		t.Skip("live differential test: set MEDIUM_LIVE=1 to run (default suite stays offline)")
	}
}

// oracleResults mirrors the v1 oracle fixture envelope ({meta, results}). Only
// the id arrays the differential tests compare are decoded.
type oracleResults struct {
	Results struct {
		Articles           []string `json:"articles"`            // g4 search
		AssociatedArticles []string `json:"associated_articles"` // g5 author archive
	} `json:"results"`
}

func loadOracleIDs(t *testing.T, fixture string, pick func(oracleResults) []string) map[string]bool {
	t.Helper()
	b := readFixture(t, "oracle", fixture)
	var o oracleResults
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatalf("decoding oracle %s: %v", fixture, err)
	}
	ids := pick(o)
	if len(ids) == 0 {
		t.Fatalf("oracle %s yielded no ids", fixture)
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// TestLiveSearchSubsetOfOracle: live Search "product builder" first pages must
// be a (near-)100% subset of the oracle search ids. Each fetched id should
// appear in the oracle's results.articles set.
func TestLiveSearchSubsetOfOracle(t *testing.T) {
	requireLive(t)
	oracle := loadOracleIDs(t, "g4-search-product-builder.json", func(o oracleResults) []string {
		return o.Results.Articles
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := New(source.NewHTTPClient(60 * time.Second))
	// Two pages' worth (20 results) is enough to assert subset behavior without a
	// long crawl; the oracle holds the full ranked set so any fetched id should
	// be present in it.
	got, err := s.Search(ctx, "product builder", 20)
	if err != nil {
		t.Fatalf("live Search: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("live Search returned no results")
	}

	missing := 0
	for _, p := range got {
		if !oracle[p.ID] {
			missing++
			t.Logf("fetched id %s (%q) not in oracle set", p.ID, p.Title)
		}
	}
	// Require a strict subset: every fetched id must be in the oracle. The spec's
	// criterion is ~100% overlap on fetched pages.
	if missing > 0 {
		t.Errorf("live search: %d/%d fetched ids not in oracle (want 0)", missing, len(got))
	} else {
		t.Logf("live search: all %d fetched ids are in the oracle set", len(got))
	}
}

// TestLiveAuthorArchiveCoversOracle: live AuthorArchive for user bcab753a4d4e
// must cover 200/200 of the oracle's associated_articles. We fetch enough pages
// to span the oracle (it captured 200 ids) and assert every oracle id is present
// in the live result.
func TestLiveAuthorArchiveCoversOracle(t *testing.T) {
	requireLive(t)
	oracle := loadOracleIDs(t, "g5-nickbabich-articles.json", func(o oracleResults) []string {
		return o.Results.AssociatedArticles
	})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	s := New(source.NewHTTPClient(60 * time.Second))
	// max=0 means "all pages" (bounded by the source's hard cap). The oracle has
	// 200 ids; the full archive should cover all of them.
	got, err := s.AuthorArchive(ctx, "bcab753a4d4e", 0)
	if err != nil {
		t.Fatalf("live AuthorArchive: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("live AuthorArchive returned no results")
	}

	live := make(map[string]bool, len(got))
	for _, p := range got {
		live[p.ID] = true
	}

	covered := 0
	for id := range oracle {
		if live[id] {
			covered++
		} else {
			t.Logf("oracle id %s not covered by live archive", id)
		}
	}
	if covered != len(oracle) {
		t.Errorf("live author-archive covered %d/%d oracle ids (want %d/%d)", covered, len(oracle), len(oracle), len(oracle))
	} else {
		t.Logf("live author-archive covered all %d oracle ids (fetched %d total)", len(oracle), len(got))
	}
}
