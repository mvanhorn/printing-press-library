// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/store"
)

// seedArticle writes one archived article into a fresh temp store using the
// exact projection author-archive produces, and returns the db path. This is
// the writer side of the store contract; the command under test is the reader.
func seedArticle(t *testing.T, s source.PostSummary, art *source.Article) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "store.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	raw, err := json.Marshal(buildArchiveRecord(s, art, s.Username))
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if err := db.Upsert("articles", s.ID, raw); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return dbPath
}

// TestDigest_ReadsCanonicalArchiveRecord is the end-to-end regression guard for
// the field-key mismatch: before the fix, digest read published_at/author while
// author-archive wrote first_published_at/author_name, so every row failed the
// IsZero date filter and digest returned empty. With the canonical keys, the
// archived article must surface with a populated author and date.
func TestDigest_ReadsCanonicalArchiveRecord(t *testing.T) {
	dbPath := seedArticle(t, source.PostSummary{
		ID:          "abc123",
		Title:       "Learn to Code",
		URL:         "https://medium.com/p/abc123",
		Author:      "Quincy Larson",
		Username:    "quincylarson",
		PublishedAt: time.Now().Add(-24 * time.Hour),
	}, nil)

	flags := &rootFlags{asJSON: true}
	cmd := newNovelDigestCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Wide window so a recently-published article is inside the digest cutoff.
	cmd.SetArgs([]string{"--db", dbPath, "--since", "3650d"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("digest execute: %v (out=%s)", err, out.String())
	}

	var entries []digestEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("parse digest JSON: %v (raw=%s)", err, out.String())
	}
	if len(entries) != 1 {
		t.Fatalf("digest returned %d entries, want 1 — field-key mismatch likely regressed (raw=%s)", len(entries), out.String())
	}
	if entries[0].Author == "" {
		t.Error("digest entry has blank author — store key mismatch regressed")
	}
	if entries[0].PublishedAt == "" {
		t.Error("digest entry has blank published_at — store key mismatch regressed")
	}
}
