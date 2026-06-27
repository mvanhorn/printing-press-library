// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package techmeme

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

func loadFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "feed.xml"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return body
}

func TestParseFeedFixture(t *testing.T) {
	signals, err := parseFeed(loadFixture(t), source.SyncOptions{})
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected items from fixture")
	}
	for i, s := range signals {
		if s.Source != "techmeme" {
			t.Errorf("signal %d: source = %q, want techmeme", i, s.Source)
		}
		if s.ID == "" {
			t.Errorf("signal %d: empty ID", i)
		}
		if s.Title == "" {
			t.Errorf("signal %d: empty Title", i)
		}
		if s.URL == "" {
			t.Errorf("signal %d: empty URL", i)
		}
		if s.RawJSON == "" {
			t.Errorf("signal %d: empty RawJSON", i)
		}
	}
}

func TestParseFeedQueryFilter(t *testing.T) {
	body := loadFixture(t)
	// A query term that cannot appear filters everything out (no fabricated rows).
	signals, err := parseFeed(body, source.SyncOptions{Query: "zzzznomatchqqqq"})
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for non-matching query, got %d", len(signals))
	}
}

func TestParseFeedSinceFilter(t *testing.T) {
	body := loadFixture(t)
	// A future lower-bound excludes every dated item.
	future := time.Now().Add(100 * 365 * 24 * time.Hour)
	signals, err := parseFeed(body, source.SyncOptions{Since: future})
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	for _, s := range signals {
		if !s.PublishedAt.IsZero() {
			t.Errorf("item with dated PublishedAt %v should have been excluded by future Since", s.PublishedAt)
		}
	}
}
