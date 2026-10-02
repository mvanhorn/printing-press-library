// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
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
