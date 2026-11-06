// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestReprintMigratesAndUpdatesExistingTranscriptCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prior.db")
	prior, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE transcripts (video_id TEXT NOT NULL, clip_id TEXT NOT NULL, kind TEXT NOT NULL, text TEXT, word_timings_json TEXT, fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(video_id,clip_id,kind))`,
		`CREATE VIRTUAL TABLE transcripts_fts USING fts5(video_id UNINDEXED, clip_id UNINDEXED, kind UNINDEXED, text, tokenize='porter unicode61')`,
		`INSERT INTO transcripts(video_id,clip_id,kind,text,word_timings_json) VALUES ('video','clip','cut','prior caption','[]')`,
	} {
		if _, err := prior.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := prior.Exec(`INSERT INTO transcripts_fts(rowid,video_id,clip_id,kind,text) VALUES (?,'video','clip','cut','prior caption')`, transcriptFTSRowID("video/clip/cut")); err != nil {
		t.Fatal(err)
	}
	if err := prior.Close(); err != nil {
		t.Fatal(err)
	}
	current, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	hits, err := current.SearchTranscripts("prior", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("prior transcript search=%v error=%v", hits, err)
	}
	if err := current.UpsertCachedTranscript("video", "clip", "cut", "updated caption", "[]"); err != nil {
		t.Fatal(err)
	}
	hits, err = current.SearchTranscripts("updated", 10)
	if err != nil || len(hits) != 1 || hits[0].VideoID != "video" {
		t.Fatalf("updated transcript search=%v error=%v", hits, err)
	}
	hits, err = current.SearchTranscripts("prior", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("stale FTS row remained=%v error=%v", hits, err)
	}
	if count, err := current.CountTranscripts(); err != nil || count != 1 {
		t.Fatalf("transcript count=%d error=%v", count, err)
	}
}
