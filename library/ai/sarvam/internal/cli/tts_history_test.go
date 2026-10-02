// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/store"
)

func TestTTSHistoryKeepsRequestMetadataWithoutAudioPayload(t *testing.T) {
	response := json.RawMessage(`{"request_id":"tts-1","audios":["large-base64-audio"]}`)
	record, err := newTTSHistoryRecord(response, map[string]any{
		"text":          "नमस्ते",
		"language_code": "hi-IN",
		"speaker":       "shubh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.RequestID != "tts-1" {
		t.Fatalf("record = %#v", record)
	}
	if string(record.Request) == "" || string(record.Request) == "null" {
		t.Fatalf("request metadata was not retained: %s", record.Request)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || json.Valid(encoded) == false {
		t.Fatalf("invalid encoded record: %s", encoded)
	}
	if strings.Contains(string(encoded), "large-base64-audio") {
		t.Fatalf("audio payload unexpectedly retained: %s", encoded)
	}
	if string(encoded) == string(response) {
		t.Fatalf("history duplicated raw audio response: %s", encoded)
	}
}

func TestTTSHistoryRequiresRequestID(t *testing.T) {
	if _, err := newTTSHistoryRecord(json.RawMessage(`{"audios":[]}`), map[string]any{}); err == nil {
		t.Fatal("newTTSHistoryRecord() accepted response without request_id")
	}
}

func TestTTSHistoryPersistsRequestWithoutAudio(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	response := json.RawMessage(`{"request_id":"tts-local","audios":["base64-audio-fixture"]}`)
	if err := persistTTSHistory(context.Background(), response, map[string]any{"text": "fixture text"}); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenReadOnly(defaultDBPath("sarvam-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw, err := db.Get("text-to-speech", "tts-local")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "fixture text") || strings.Contains(string(raw), "base64-audio-fixture") {
		t.Fatalf("TTS history lost request text or saved audio: %s", raw)
	}
}
