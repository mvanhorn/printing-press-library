// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package hackernews

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSearchFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "algolia_search.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	signals, err := parseSearch(body)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(signals) == 0 {
		t.Fatal("expected at least one signal from fixture")
	}
	for i, s := range signals {
		if s.Source != "hackernews" {
			t.Errorf("signal %d: source = %q, want hackernews", i, s.Source)
		}
		if s.ID == "" {
			t.Errorf("signal %d: empty ID", i)
		}
		if s.Title == "" {
			t.Errorf("signal %d: empty Title", i)
		}
		if s.URL == "" {
			t.Errorf("signal %d: empty URL (should fall back to HN permalink)", i)
		}
		if s.RawJSON == "" {
			t.Errorf("signal %d: empty RawJSON (raw evidence must be preserved)", i)
		}
		if s.PublishedAt.IsZero() {
			t.Errorf("signal %d: zero PublishedAt", i)
		}
	}
}

func TestParseSearchPermalinkFallback(t *testing.T) {
	// A text post (Ask HN) has an empty url; URL must fall back to the HN
	// item permalink, not stay empty.
	body := []byte(`{"hits":[{"objectID":"42","title":"Ask HN: anything?","url":"","author":"pg","points":10,"num_comments":3,"created_at_i":1700000000,"story_text":"body"}]}`)
	signals, err := parseSearch(body)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	want := itemPermalink + "42"
	if signals[0].URL != want {
		t.Errorf("URL = %q, want %q", signals[0].URL, want)
	}
	if signals[0].Comments != 3 || signals[0].Points != 10 {
		t.Errorf("points/comments mismapped: got %d/%d, want 10/3", signals[0].Points, signals[0].Comments)
	}
}
