// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openScheduleTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestScheduledSendLifecycle(t *testing.T) {
	s := openScheduleTestStore(t)
	due := time.Now().Add(-time.Minute)
	id, err := s.CreateScheduledSend(ScheduledSend{
		To: "a@b.c", Subject: "hi", BodyText: "body", SendAt: due,
	})
	if err != nil {
		t.Fatalf("CreateScheduledSend: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	items, err := s.ListScheduledSends("pending", 10)
	if err != nil {
		t.Fatalf("ListScheduledSends: %v", err)
	}
	if len(items) != 1 || items[0].Subject != "hi" || items[0].Status != "pending" {
		t.Fatalf("pending list = %+v", items)
	}

	dueItems, err := s.DueScheduledSends(time.Now())
	if err != nil {
		t.Fatalf("DueScheduledSends: %v", err)
	}
	if len(dueItems) != 1 || dueItems[0].ID != id {
		t.Fatalf("due list = %+v", dueItems)
	}

	ok, err := s.ClaimScheduledSend(id)
	if err != nil || !ok {
		t.Fatalf("ClaimScheduledSend = %v, %v", ok, err)
	}
	// Second claim must fail: idempotency guarantee.
	ok, err = s.ClaimScheduledSend(id)
	if err != nil || ok {
		t.Fatalf("second ClaimScheduledSend = %v, want false", ok)
	}

	if err := s.FinishScheduledSend(id, "gm-123", nil); err != nil {
		t.Fatalf("FinishScheduledSend: %v", err)
	}
	sent, err := s.ListScheduledSends("sent", 10)
	if err != nil {
		t.Fatalf("ListScheduledSends(sent): %v", err)
	}
	if len(sent) != 1 || sent[0].GmailID != "gm-123" || sent[0].SentAtJSON == "" {
		t.Fatalf("sent list = %+v", sent)
	}

	// A sent item is no longer due.
	dueItems, err = s.DueScheduledSends(time.Now())
	if err != nil || len(dueItems) != 0 {
		t.Fatalf("due after send = %+v, %v", dueItems, err)
	}
}

func TestScheduledSendFailureRecordsError(t *testing.T) {
	s := openScheduleTestStore(t)
	id, err := s.CreateScheduledSend(ScheduledSend{To: "a@b.c", Subject: "x", BodyText: "y", SendAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateScheduledSend: %v", err)
	}
	if ok, err := s.ClaimScheduledSend(id); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if err := s.FinishScheduledSend(id, "", errors.New("api exploded")); err != nil {
		t.Fatalf("FinishScheduledSend(err): %v", err)
	}
	failed, err := s.ListScheduledSends("failed", 10)
	if err != nil || len(failed) != 1 || failed[0].LastError != "api exploded" {
		t.Fatalf("failed list = %+v, %v", failed, err)
	}
}

func TestScheduledSendCancelAndReschedule(t *testing.T) {
	s := openScheduleTestStore(t)
	future := time.Now().Add(2 * time.Hour)
	id, err := s.CreateScheduledSend(ScheduledSend{To: "a@b.c", Subject: "x", BodyText: "y", SendAt: future})
	if err != nil {
		t.Fatalf("CreateScheduledSend: %v", err)
	}

	newTime := time.Now().Add(4 * time.Hour)
	ok, err := s.UpdateScheduledSendTime(id, newTime)
	if err != nil || !ok {
		t.Fatalf("UpdateScheduledSendTime = %v, %v", ok, err)
	}
	items, _ := s.ListScheduledSends("pending", 10)
	if len(items) != 1 || items[0].SendAt.UTC().Sub(newTime.UTC()).Abs() > time.Second {
		t.Fatalf("rescheduled item = %+v", items)
	}

	ok, err = s.CancelScheduledSend(id)
	if err != nil || !ok {
		t.Fatalf("CancelScheduledSend = %v, %v", ok, err)
	}
	// Cancel of a non-pending item reports false.
	ok, err = s.CancelScheduledSend(id)
	if err != nil || ok {
		t.Fatalf("second CancelScheduledSend = %v, want false", ok)
	}
	// Empty status lists all.
	all, err := s.ListScheduledSends("", 10)
	if err != nil || len(all) != 1 || all[0].Status != "canceled" {
		t.Fatalf("all list = %+v, %v", all, err)
	}
}
