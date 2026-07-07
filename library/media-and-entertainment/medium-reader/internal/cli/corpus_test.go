// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// TestCorpus_HitsCarryAuthorAndDate guards the corpus side of the same store
// contract: before the fix, corpus read author/published_at while author-archive
// wrote author_name/first_published_at, so every hit had a blank author and date.
// A title match must now return a hit with both populated.
func TestCorpus_HitsCarryAuthorAndDate(t *testing.T) {
	dbPath := seedArticle(t, source.PostSummary{
		ID:          "def456",
		Title:       "Designing Calm Software",
		URL:         "https://medium.com/p/def456",
		Author:      "Quincy Larson",
		Username:    "quincylarson",
		PublishedAt: time.Now().Add(-48 * time.Hour),
	}, &source.Article{Markdown: "calm software body", Subtitle: "on restraint"})

	flags := &rootFlags{asJSON: true}
	cmd := newNovelCorpusCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"calm", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("corpus execute: %v (out=%s)", err, out.String())
	}

	var hits []corpusHit
	if err := json.Unmarshal(out.Bytes(), &hits); err != nil {
		t.Fatalf("parse corpus JSON: %v (raw=%s)", err, out.String())
	}
	if len(hits) != 1 {
		t.Fatalf("corpus returned %d hits, want 1 (raw=%s)", len(hits), out.String())
	}
	if hits[0].Author == "" {
		t.Error("corpus hit has blank author — store key mismatch regressed")
	}
	if hits[0].PublishedAt == "" {
		t.Error("corpus hit has blank published_at — store key mismatch regressed")
	}
}
