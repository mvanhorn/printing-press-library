// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestUpsertInboxMessagesReturnsNumericHighest locks in the int64 highest
// return from upsertInboxMessages. The inbox-sync --delete-through path
// sends `message` to Pushover's update_highest_message.json endpoint, which
// is type-strict and rejects string-encoded message IDs. Regressing the
// signature back to (int, string, error) would silently re-introduce the
// bug fixed in PR #511.
func TestUpsertInboxMessagesReturnsNumericHighest(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"id":100,"title":"a"}`),
		json.RawMessage(`{"id":789,"title":"b"}`),
		json.RawMessage(`{"id":42,"title":"c"}`),
	}
	dbPath := filepath.Join(t.TempDir(), "data.db")
	stored, highest, highestNum, err := upsertInboxMessages(context.Background(), dbPath, items)
	if err != nil {
		t.Fatalf("upsertInboxMessages: %v", err)
	}
	if stored != 3 {
		t.Fatalf("stored = %d, want 3", stored)
	}
	if highest != "789" {
		t.Fatalf("highest = %q, want %q", highest, "789")
	}
	if highestNum != 789 {
		t.Fatalf("highestNum = %d, want 789", highestNum)
	}
}

// TestUpsertInboxMessagesNonNumericIDs covers the fallback path where every
// message ID is non-numeric (sha256 prefix from the missing-id branch).
// In that case the delete-through caller must NOT POST to update_highest_message.json,
// so highestNum stays at -1 to signal "no numeric id seen".
func TestUpsertInboxMessagesNonNumericIDs(t *testing.T) {
	// Items without "id" trigger the sha256 fallback inside inboxRecordFromRaw.
	items := []json.RawMessage{
		json.RawMessage(`{"title":"no-id-one"}`),
		json.RawMessage(`{"title":"no-id-two"}`),
	}
	dbPath := filepath.Join(t.TempDir(), "data.db")
	_, highest, highestNum, err := upsertInboxMessages(context.Background(), dbPath, items)
	if err != nil {
		t.Fatalf("upsertInboxMessages: %v", err)
	}
	if highest == "" {
		t.Fatal("highest should fall back to first hashed ID, got empty")
	}
	if highestNum != -1 {
		t.Fatalf("highestNum = %d, want -1 (gate for delete-through)", highestNum)
	}
}
