// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSarvamResourceScopedIDFallbacks(t *testing.T) {
	for _, tc := range []struct {
		resource string
		item     map[string]any
		want     string
	}{
		{resource: "doc-ai", item: map[string]any{"job_id": "doc-job"}, want: "doc-job"},
		{resource: "doc-ai", item: map[string]any{"upload_id": "doc-upload"}, want: "doc-upload"},
		{resource: "speech-to-text", item: map[string]any{"request_id": "stt-request"}, want: "stt-request"},
		{resource: "speech-to-text", item: map[string]any{"job_id": "stt-job"}, want: "stt-job"},
		{resource: "text-lid", item: map[string]any{"request_id": "lid-request"}, want: "lid-request"},
		{resource: "text-to-speech", item: map[string]any{"request_id": "tts-request"}, want: "tts-request"},
		{resource: "text-to-speech", item: map[string]any{"dictionary_id": "tts-dictionary", "request_id": "tts-request"}, want: "tts-dictionary"},
		{resource: "text-to-speech", item: map[string]any{"dictionary_id": "tts-dictionary", "name": "display-name"}, want: "tts-dictionary"},
		{resource: "translate", item: map[string]any{"request_id": "translate-request"}, want: "translate-request"},
		{resource: "transliterate", item: map[string]any{"request_id": "transliterate-request"}, want: "transliterate-request"},
		{resource: "models", item: map[string]any{"request_id": "foreign-request"}, want: ""},
		{resource: "speech-to-text", item: map[string]any{"id": "stable-id", "request_id": "stt-request"}, want: "stable-id"},
		{resource: "text-to-speech", item: map[string]any{"name": "voice-name", "request_id": "tts-request"}, want: "voice-name"},
	} {
		if got := ExtractResourceID(tc.resource, tc.item); got != tc.want {
			t.Errorf("ExtractResourceID(%q, %#v) = %q, want %q", tc.resource, tc.item, got, tc.want)
		}
	}
}

func TestSarvamDictionarySyncMigratesLegacyNameKey(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := json.RawMessage(`{"dictionary_id":"dictionary-1","name":"legacy-name"}`)
	if err := db.Upsert("text-to-speech", "legacy-name", legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO text_to_speech (id, data, dictionary_id, name) VALUES (?, ?, ?, ?)`,
		"legacy-name", string(legacy), "dictionary-1", "legacy-name",
	); err != nil {
		t.Fatal(err)
	}
	// Generic-only rows can survive a failed typed projection and need cleanup too.
	if err := db.Upsert("text-to-speech", "older-name", json.RawMessage(`{"dictionary_id":"dictionary-1","name":"older-name"}`)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertBatch("text-to-speech", []json.RawMessage{json.RawMessage(`{"name":"voice-name"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertBatch("text-to-speech", []json.RawMessage{json.RawMessage(`{"dictionary_id":"dictionary-1","name":"current-name"}`)}); err != nil {
		t.Fatal(err)
	}
	for _, staleID := range []string{"legacy-name", "older-name"} {
		if _, err := db.Get("text-to-speech", staleID); err != sql.ErrNoRows {
			t.Fatalf("legacy row %q remains or lookup failed: %v", staleID, err)
		}
		var matches int
		if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources_fts WHERE rowid = ?`, ftsRowID("text-to-speech", staleID)).Scan(&matches); err != nil || matches != 0 {
			t.Fatalf("legacy search row %q count=%d err=%v", staleID, matches, err)
		}
	}
	for _, id := range []string{"dictionary-1", "voice-name"} {
		if _, err := db.Get("text-to-speech", id); err != nil {
			t.Fatalf("expected row %q is missing: %v", id, err)
		}
	}
	for _, table := range []string{"resources", "text_to_speech"} {
		var rows int
		query := `SELECT COUNT(*) FROM ` + table
		if table == "resources" {
			query += ` WHERE resource_type = 'text-to-speech'`
		}
		if err := db.DB().QueryRow(query).Scan(&rows); err != nil || rows != 2 {
			t.Fatalf("%s row count=%d err=%v, want dictionary and voice", table, rows, err)
		}
	}
}

func TestSarvamDictionaryCreateAndDetailUseOneLocalRow(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, item := range []json.RawMessage{
		json.RawMessage(`{"dictionary_id":"dictionary-1"}`),
		json.RawMessage(`{"dictionary_id":"dictionary-1","name":"display-name","pronunciations":[]}`),
	} {
		stored, skipped, err := db.UpsertBatch("text-to-speech", []json.RawMessage{item})
		if err != nil || stored != 1 || skipped != 0 {
			t.Fatalf("UpsertBatch stored=%d skipped=%d err=%v, want one stored row", stored, skipped, err)
		}
	}
	var rows int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, "text-to-speech").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("dictionary created and then fetched left %d rows, want one", rows)
	}
	got, err := db.Get("text-to-speech", "dictionary-1")
	if err != nil {
		t.Fatal(err)
	}
	var detail map[string]any
	if err := json.Unmarshal(got, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["name"] != "display-name" {
		t.Fatalf("dictionary detail did not replace create response: %#v", detail)
	}
}

func TestSarvamRequestIDResponsePersistsToLocalStore(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	item := json.RawMessage(`{"request_id":"stt-request","status":"completed"}`)
	stored, skipped, err := db.UpsertBatch("speech-to-text", []json.RawMessage{item})
	if err != nil || stored != 1 || skipped != 0 {
		t.Fatalf("UpsertBatch stored=%d skipped=%d err=%v, want one stored row", stored, skipped, err)
	}
	got, err := db.Get("speech-to-text", "stt-request")
	if err != nil || !json.Valid(got) {
		t.Fatalf("request-scoped row is missing from the local store: %v", err)
	}
}
