// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDigestStore is a memory-backed PendingDigestStore for gate tests.
// Mirrors store.Store's contract but lives in-process so the test doesn't
// have to wire up SQLite.
type fakeDigestStore struct {
	mu   sync.Mutex
	rows map[string]*PendingDigest
}

func newFakeDigestStore() *fakeDigestStore {
	return &fakeDigestStore{rows: map[string]*PendingDigest{}}
}

func (s *fakeDigestStore) PutPendingDigest(ctx context.Context, digest, command string, plan []byte, rowCount int, expires time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.rows[digest] = &PendingDigest{
		Digest: digest, Command: command, PlanJSON: append([]byte(nil), plan...),
		RowCount: rowCount, CreatedAt: now, ExpiresAt: now.Add(expires),
	}
	return nil
}

func (s *fakeDigestStore) GetPendingDigest(ctx context.Context, digest string) (*PendingDigest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[digest]
	if !ok {
		return nil, nil
	}
	if time.Now().After(r.ExpiresAt) {
		return nil, nil
	}
	return r, nil
}

func (s *fakeDigestStore) PurgeExpiredDigests(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for d, r := range s.rows {
		if time.Now().After(r.ExpiresAt) {
			delete(s.rows, d)
		}
	}
	return nil
}

// expireRow forces a row's ExpiresAt into the past for testing.
func (s *fakeDigestStore) expireRow(digest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.rows[digest]; ok {
		r.ExpiresAt = time.Now().Add(-time.Hour)
	}
}

func TestMakeDigest_StableAcrossKeyOrder(t *testing.T) {
	// Two semantically-identical plans encoded with different key orders
	// must produce the same digest.
	a := []byte(`{"inputs":[{"id":"1","properties":{"email":"a@b.com","firstname":"A"}}]}`)
	b := []byte(`{"inputs":[{"id":"1","properties":{"firstname":"A","email":"a@b.com"}}]}`)
	da := MakeDigest("contacts bulk-update", a)
	db := MakeDigest("contacts bulk-update", b)
	if da != db {
		t.Fatalf("expected stable digest across key order, got %q vs %q", da, db)
	}
}

func TestMakeDigest_ChangesWithCommand(t *testing.T) {
	plan := []byte(`{"inputs":[]}`)
	a := MakeDigest("contacts bulk-update", plan)
	b := MakeDigest("companies bulk-update", plan)
	if a == b {
		t.Fatalf("expected distinct digests for different commands, both = %q", a)
	}
}

func TestMakeDigest_HasBlastPrefix(t *testing.T) {
	d := MakeDigest("x", []byte(`{}`))
	if !strings.HasPrefix(d, "blast-") {
		t.Fatalf("expected blast- prefix, got %q", d)
	}
}

func TestGate_BelowThresholdBypasses(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	outcome, digest, count, err := g.Evaluate(context.Background(), fake, "x", "", 0, []byte(`{"n":50}`), 50)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if outcome != GateProceedBelowThreshold {
		t.Fatalf("expected proceed-below-threshold, got %v", outcome)
	}
	if digest == "" || count != 50 {
		t.Fatalf("expected digest set and count=50, got digest=%q count=%d", digest, count)
	}
	if len(fake.rows) != 0 {
		t.Fatalf("expected no rows persisted below threshold, got %d", len(fake.rows))
	}
}

func TestGate_DryRunPersists(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	plan := []byte(`{"inputs":[{"id":"1"}],"_count":200}`)
	outcome, digest, count, err := g.Evaluate(context.Background(), fake, "contacts bulk-update", "", 0, plan, 200)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if outcome != GateDryRunPersisted {
		t.Fatalf("expected dry-run-persisted, got %v", outcome)
	}
	if count != 200 {
		t.Fatalf("expected count=200, got %d", count)
	}
	if _, ok := fake.rows[digest]; !ok {
		t.Fatalf("expected digest persisted, got rows=%v", fake.rows)
	}
}

func TestGate_ConfirmMatches(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	plan := []byte(`{"inputs":[{"id":"1"}],"_count":200}`)
	// First call: dry-run.
	_, digest, _, err := g.Evaluate(context.Background(), fake, "contacts bulk-update", "", 0, plan, 200)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Second call: confirm.
	outcome, _, _, err := g.Evaluate(context.Background(), fake, "contacts bulk-update", digest, 200, plan, 200)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if outcome != GateProceedConfirmed {
		t.Fatalf("expected proceed-confirmed, got %v", outcome)
	}
}

func TestGate_ConfirmMismatch(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	plan := []byte(`{"inputs":[{"id":"1"}],"_count":200}`)
	_, digest, _, err := g.Evaluate(context.Background(), fake, "x", "", 0, plan, 200)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	_, _, _, err = g.Evaluate(context.Background(), fake, "x", digest, 199, plan, 200)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch err, got %v", err)
	}
}

func TestGate_ExpiredDigest(t *testing.T) {
	g := DigestGate{Threshold: 100, TTL: time.Hour}
	fake := newFakeDigestStore()
	plan := []byte(`{"inputs":[{"id":"1"}],"_count":150}`)
	_, digest, _, err := g.Evaluate(context.Background(), fake, "x", "", 0, plan, 150)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	fake.expireRow(digest)
	_, _, _, err = g.Evaluate(context.Background(), fake, "x", digest, 150, plan, 150)
	if err == nil || !strings.Contains(err.Error(), "expired or unknown") {
		t.Fatalf("expected expired-or-unknown err, got %v", err)
	}
}

func TestGate_UnknownDigest(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	plan := []byte(`{"_count":150}`)
	_, _, _, err := g.Evaluate(context.Background(), fake, "x", "blast-deadbeefdeadbeef", 150, plan, 150)
	if err == nil || !strings.Contains(err.Error(), "expired or unknown") {
		t.Fatalf("expected unknown-digest err, got %v", err)
	}
}

func TestGate_PlanChangedSinceDryRun(t *testing.T) {
	g := DefaultGate()
	fake := newFakeDigestStore()
	planA := []byte(`{"_count":150,"a":1}`)
	_, digest, _, err := g.Evaluate(context.Background(), fake, "x", "", 0, planA, 150)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Re-run with a different plan but the same row count.
	planB := []byte(`{"_count":150,"a":2}`)
	_, _, _, err = g.Evaluate(context.Background(), fake, "x", digest, 150, planB, 150)
	if err == nil || !strings.Contains(err.Error(), "plan content changed") {
		t.Fatalf("expected plan-content-changed err, got %v", err)
	}
}
