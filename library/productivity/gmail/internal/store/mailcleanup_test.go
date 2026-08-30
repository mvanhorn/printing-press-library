// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for the cleanup engine's durability tables: the atomic
// nonce-burn + authorization transaction (grill R3-C2), chunk intent
// lifecycle, and idempotent ledger inserts.

package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openCleanupTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAuthorizeMailApply_AtomicNonceLifecycle(t *testing.T) {
	t.Parallel()
	s := openCleanupTestStore(t)
	now := time.Now()
	sha := "aa11"
	if err := s.CreateMailNonce("n1", sha, "acct", now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Unknown token.
	if _, err := s.AuthorizeMailApply("nope", sha, "acct", "trash", now); !errors.Is(err, ErrNonceUnknown) {
		t.Fatalf("unknown token err = %v, want ErrNonceUnknown", err)
	}
	// Wrong plan binding.
	if _, err := s.AuthorizeMailApply("n1", "other-sha", "acct", "trash", now); !errors.Is(err, ErrNonceMismatch) {
		t.Fatalf("wrong sha err = %v, want ErrNonceMismatch", err)
	}
	// Wrong account binding.
	if _, err := s.AuthorizeMailApply("n1", sha, "other-acct", "trash", now); !errors.Is(err, ErrNonceMismatch) {
		t.Fatalf("wrong account err = %v, want ErrNonceMismatch", err)
	}
	// Refusals above must NOT have burned the nonce.
	applyID, err := s.AuthorizeMailApply("n1", sha, "acct", "trash", now)
	if err != nil {
		t.Fatalf("valid authorize err = %v", err)
	}
	if applyID <= 0 {
		t.Fatalf("applyID = %d, want > 0", applyID)
	}
	a, err := s.GetMailApply(applyID)
	if err != nil || a.State != MailApplyStateAuthorized {
		t.Fatalf("apply row = %+v err = %v, want authorized", a, err)
	}
	// Single use: the same tx that recorded the apply burned the nonce.
	if _, err := s.AuthorizeMailApply("n1", sha, "acct", "trash", now); !errors.Is(err, ErrNonceUsed) {
		t.Fatalf("reuse err = %v, want ErrNonceUsed", err)
	}

	// Expired token.
	if err := s.CreateMailNonce("n2", sha, "acct", now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthorizeMailApply("n2", sha, "acct", "trash", now); !errors.Is(err, ErrNonceExpired) {
		t.Fatalf("expired err = %v, want ErrNonceExpired", err)
	}
}

func TestMailApplyChunksAndLedgerLifecycle(t *testing.T) {
	t.Parallel()
	s := openCleanupTestStore(t)
	now := time.Now()
	if err := s.CreateMailNonce("n1", "sha1", "acct", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	applyID, err := s.AuthorizeMailApply("n1", "sha1", "acct", "trash", now)
	if err != nil {
		t.Fatal(err)
	}

	chunks := []MailApplyChunk{
		{ApplyID: applyID, ChunkNo: 0, Kind: "trash", IDs: []string{"m1", "m2"}},
		{ApplyID: applyID, ChunkNo: 1, Kind: "label", IDs: []string{"m3"}, Add: []string{"L1"}, Remove: []string{"UNREAD"}},
	}
	if err := s.InsertMailApplyChunks(chunks); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListMailApplyChunks(applyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].State != MailChunkStatePending || got[1].Add[0] != "L1" {
		t.Fatalf("chunks round-trip mismatch: %+v", got)
	}
	if err := s.SetMailApplyChunkState(applyID, 0, MailChunkStateDone); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListMailApplyChunks(applyID)
	if got[0].State != MailChunkStateDone {
		t.Fatalf("chunk 0 state = %s, want done", got[0].State)
	}

	// Leftovers view tracks non-terminal applies only.
	leftovers, err := s.ListMailApplyLeftovers()
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 1 || leftovers[0].ID != applyID {
		t.Fatalf("leftovers = %+v, want the authorized apply", leftovers)
	}
	if err := s.SetMailApplyState(applyID, MailApplyStateDone); err != nil {
		t.Fatal(err)
	}
	leftovers, _ = s.ListMailApplyLeftovers()
	if len(leftovers) != 0 {
		t.Fatalf("leftovers after done = %+v, want none", leftovers)
	}

	// Ledger: INSERT OR IGNORE makes recovery re-inserts idempotent.
	if err := s.CreateMailLedger(MailLedger{LedgerID: "L1", Account: "acct", PlanSha: "sha1", ApplyID: applyID, Action: "trash"}); err != nil {
		t.Fatal(err)
	}
	entries := []MailLedgerEntry{
		{LedgerID: "L1", ID: "m1", Kind: "trash", DeltaAdd: []string{"TRASH"}, PrePlacement: []string{"INBOX"}},
		{LedgerID: "L1", ID: "m2", Kind: "trash", DeltaAdd: []string{"TRASH"}},
	}
	n, err := s.InsertMailLedgerEntries(entries)
	if err != nil || n != 2 {
		t.Fatalf("first insert n=%d err=%v, want 2", n, err)
	}
	n, err = s.InsertMailLedgerEntries(entries)
	if err != nil || n != 0 {
		t.Fatalf("idempotent re-insert n=%d err=%v, want 0", n, err)
	}
	roundTrip, err := s.ListMailLedgerEntries("L1")
	if err != nil || len(roundTrip) != 2 {
		t.Fatalf("ledger entries = %d err=%v, want 2", len(roundTrip), err)
	}
	if !contains(roundTrip[0].PrePlacement, "INBOX") && !contains(roundTrip[1].PrePlacement, "INBOX") {
		t.Fatalf("pre_placement lost in round-trip: %+v", roundTrip)
	}
	if err := s.SetMailLedgerEntryUndone("L1", "m1", "undone"); err != nil {
		t.Fatal(err)
	}
	roundTrip, _ = s.ListMailLedgerEntries("L1")
	for _, e := range roundTrip {
		if e.ID == "m1" && e.Undone != "undone" {
			t.Fatalf("m1 undone = %q, want undone", e.Undone)
		}
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
