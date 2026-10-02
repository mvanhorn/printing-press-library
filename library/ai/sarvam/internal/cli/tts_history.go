// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Library patch: persist searchable TTS request metadata without duplicating base64 audio in SQLite.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/store"
)

type ttsHistoryRecord struct {
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
}

func newTTSHistoryRecord(response json.RawMessage, request any) (ttsHistoryRecord, error) {
	var envelope struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return ttsHistoryRecord{}, fmt.Errorf("parsing TTS response: %w", err)
	}
	if envelope.RequestID == "" {
		return ttsHistoryRecord{}, fmt.Errorf("TTS response has no request id")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return ttsHistoryRecord{}, fmt.Errorf("encoding TTS request history: %w", err)
	}
	return ttsHistoryRecord{
		RequestID: envelope.RequestID,
		Request:   requestJSON,
	}, nil
}

func persistTTSHistory(ctx context.Context, response json.RawMessage, request any) error {
	record, err := newTTSHistoryRecord(response, request)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding TTS history: %w", err)
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("sarvam-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local TTS history: %w", err)
	}
	defer db.Close()
	if err := db.Upsert("text-to-speech", record.RequestID, raw); err != nil {
		return fmt.Errorf("saving local TTS history: %w", err)
	}
	return nil
}
