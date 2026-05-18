// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

package granola

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncState_ReadMissing(t *testing.T) {
	t.Setenv("GRANOLA_SYNC_STATE_PATH", filepath.Join(t.TempDir(), "absent.json"))
	_, err := ReadSyncState()
	if !IsSyncStateMissing(err) {
		t.Fatalf("expected missing-file signal, got %v", err)
	}
}

func TestSyncState_RoundTrip(t *testing.T) {
	t.Setenv("GRANOLA_SYNC_STATE_PATH", filepath.Join(t.TempDir(), "sync_state.json"))
	now := time.Now().Truncate(time.Second)
	in := SyncState{
		LastSyncAt:           now,
		LastDecryptStatus:    DecryptStatusOK,
		LastTokenSource:      "TokenSourceEncryptedSupabase",
		LastDocumentsFetched: 42,
	}
	if err := WriteSyncState(in); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := ReadSyncState()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !out.LastSyncAt.Equal(in.LastSyncAt) {
		t.Errorf("LastSyncAt: got %v want %v", out.LastSyncAt, in.LastSyncAt)
	}
	if out.LastDecryptStatus != in.LastDecryptStatus {
		t.Errorf("LastDecryptStatus: got %q want %q", out.LastDecryptStatus, in.LastDecryptStatus)
	}
	if out.LastDocumentsFetched != in.LastDocumentsFetched {
		t.Errorf("LastDocumentsFetched: got %d want %d", out.LastDocumentsFetched, in.LastDocumentsFetched)
	}
}

func TestSyncState_MalformedTreatedAsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sync_state.json")
	t.Setenv("GRANOLA_SYNC_STATE_PATH", path)
	if err := os.WriteFile(path, []byte("{this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSyncState()
	if !IsSyncStateMissing(err) {
		t.Fatalf("malformed file should be treated as missing, got %v", err)
	}
}

func TestSyncState_AtomicWrite(t *testing.T) {
	// WriteSyncState must use a tmp+rename so a concurrent reader never
	// catches a half-written file. We verify the tmp file is cleaned up.
	dir := t.TempDir()
	path := filepath.Join(dir, "sync_state.json")
	t.Setenv("GRANOLA_SYNC_STATE_PATH", path)
	if err := WriteSyncState(SyncState{LastDecryptStatus: DecryptStatusOK}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected tmp file to be gone after rename, got: %v", err)
	}
}
