// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package approval

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEquivalentOutcomeUnknownRequiresReconciliation(t *testing.T) {
	state := NewStore(t.TempDir(), time.Minute)
	mutation := testMutation()
	record, err := state.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatal(err)
	}
	_, permit, err := state.Consume(record.ID, record.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Complete(record.ID, StatusOutcomeUnknown, "outcome_unknown"); err != nil {
		t.Fatal(err)
	}
	permit.Release()

	if _, err := state.Create(mutation, ReviewSummary{}); err == nil {
		t.Fatal("equivalent outcome-unknown operation was not blocked")
	} else {
		var equivalent *EquivalentOperationError
		if !errors.As(err, &equivalent) || equivalent.ID != record.ID || equivalent.Status != StatusOutcomeUnknown {
			t.Fatalf("equivalent error=%T %v", err, err)
		}
	}

	reason := sha256.Sum256([]byte("carrier confirmed that no shipment was created"))
	if err := state.Reconcile(record.ID, "not_executed", hex.EncodeToString(reason[:])); err != nil {
		t.Fatal(err)
	}
	resolved, err := state.Get(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != StatusReconciledNotExecuted || resolved.ErrorClass == "" {
		t.Fatalf("resolved record=%+v", resolved)
	}
	if _, err := state.Create(mutation, ReviewSummary{}); err != nil {
		t.Fatalf("new approval after reconciliation: %v", err)
	}
}

func TestEquivalentSucceededOperationRemainsBlocked(t *testing.T) {
	state := NewStore(t.TempDir(), time.Minute)
	mutation := testMutation()
	record, err := state.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatal(err)
	}
	_, permit, err := state.Consume(record.ID, record.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Complete(record.ID, StatusSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	permit.Release()
	if _, err := state.Create(mutation, ReviewSummary{}); err == nil {
		t.Fatal("equivalent successful operation was not blocked")
	}
}

func TestExecutingOperationCannotBeReconciledBeforeApprovalExpiry(t *testing.T) {
	base := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	state := NewStore(t.TempDir(), time.Minute)
	state.now = func() time.Time { return base }
	mutation := testMutation()
	record, err := state.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatal(err)
	}
	_, permit, err := state.Consume(record.ID, record.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatal(err)
	}
	reason := sha256.Sum256([]byte("operator verified transport process exited before send"))
	reasonHash := hex.EncodeToString(reason[:])
	if err := state.Reconcile(record.ID, "not_executed", reasonHash); !errors.Is(err, ErrReconciliationNotAllowed) {
		t.Fatalf("fresh executing reconciliation error=%v", err)
	}
	state.now = func() time.Time { return record.ExpiresAt.Add(time.Second) }
	if err := state.Reconcile(record.ID, "not_executed", reasonHash); !errors.Is(err, ErrReconciliationNotAllowed) {
		t.Fatalf("expired but leased executing reconciliation error=%v", err)
	}
	permit.Release()
	if err := state.Reconcile(record.ID, "not_executed", reasonHash); err != nil {
		t.Fatalf("expired executing reconciliation: %v", err)
	}
}

func TestReconcileHookFailureLeavesOperationBlocking(t *testing.T) {
	state := NewStore(t.TempDir(), 10*time.Minute)
	mutation := testMutation()
	record, err := state.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, permit, err := state.Consume(record.ID, record.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := state.Complete(record.ID, StatusOutcomeUnknown, "transport"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	permit.Release()
	reasonHash := sha256.Sum256([]byte("ledger unavailable"))
	err = state.ReconcileWithHook(record.ID, "not_executed", hex.EncodeToString(reasonHash[:]), func(*Record) error {
		return errors.New("database write failed")
	})
	if err == nil {
		t.Fatal("ReconcileWithHook unexpectedly succeeded")
	}
	got, err := state.Get(record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusOutcomeUnknown {
		t.Fatalf("status=%q, want %q", got.Status, StatusOutcomeUnknown)
	}
	_, err = state.Create(mutation, ReviewSummary{})
	var equivalent *EquivalentOperationError
	if !errors.As(err, &equivalent) {
		t.Fatalf("Create after failed reconciliation error=%v, want EquivalentOperationError", err)
	}
}

func TestLegacyExecutingOperationRequiresManualRecovery(t *testing.T) {
	state := NewStore(t.TempDir(), time.Minute)
	mutation := testMutation()
	record, err := state.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatal(err)
	}
	record.OperationHash = ""
	record.Status = StatusExecuting
	record.ExpiresAt = time.Now().Add(-time.Minute)
	if err := state.writeRecord(record); err != nil {
		t.Fatal(err)
	}
	reason := sha256.Sum256([]byte("legacy process is believed stopped"))
	err = state.Reconcile(record.ID, "not_executed", hex.EncodeToString(reason[:]))
	if !errors.Is(err, ErrReconciliationNotAllowed) {
		t.Fatalf("legacy executing reconciliation error=%v, want ErrReconciliationNotAllowed", err)
	}
}

func TestReconciliationRejectsOperationHashChangedAfterSnapshot(t *testing.T) {
	record := &Record{RequestHash: strings.Repeat("a", sha256.Size*2), Status: StatusOutcomeUnknown}
	lockedHash, err := reconciliationLockHash(record)
	if err != nil {
		t.Fatal(err)
	}
	record.OperationHash = strings.Repeat("b", sha256.Size*2)
	if err := validateReconciliationLockHash(record, lockedHash); !errors.Is(err, ErrReconciliationNotAllowed) {
		t.Fatalf("changed operation hash validation error=%v, want ErrReconciliationNotAllowed", err)
	}
}
