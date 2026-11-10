// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"
)

func TestDoctorV2StatusPreservesLiveGateAndResourceFailures(t *testing.T) {
	checkedAt := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	session := &Session{
		ProfileName: "tenant-a", Source: "sample", GateOutcome: GateVerified,
		GateCheckedAt: checkedAt, ExpectedIdentity: map[string]string{"account_id": "one"},
		ObservedIdentity: map[string]string{"account_id": "one"},
	}
	fresh := NewDoctorReport(session, []ResourceHealth{{Name: "orders", Freshness: "fresh"}})
	if fresh.Status != "ok" || !fresh.Probe.CacheBypassed || fresh.Probe.Source != "live" || !fresh.Probe.CheckedAt.Equal(checkedAt) {
		t.Fatalf("fresh report = %+v", fresh)
	}
	stale := NewDoctorReport(session, []ResourceHealth{{Name: "orders", Freshness: "stale"}})
	if stale.Status != "warn" {
		t.Fatalf("stale status = %q", stale.Status)
	}
	resourceError := NewDoctorReport(session, []ResourceHealth{{Name: "orders", Freshness: "error"}})
	if resourceError.Status != "error" {
		t.Fatalf("resource error status = %q", resourceError.Status)
	}
	session.GateOutcome = GateMismatch
	mismatch := NewDoctorReport(session, []ResourceHealth{{Name: "orders", Freshness: "stale"}})
	if mismatch.Status != "error" || mismatch.Probe.IdentityStatus != GateMismatch {
		t.Fatalf("mismatch report = %+v", mismatch)
	}
}

func TestSharedLimiterConcurrentLedgerRemainsValid(t *testing.T) {
	setPlatformRoots(t)
	paths, err := PathsFor("synthetic", "sample-pp-cli", "sample")
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{
		ProfileName: "synthetic", Source: "sample", CredentialFingerprint: "fingerprint",
		ObservedIdentity: map[string]string{"account_id": "one"}, GateOutcome: GateVerified, Paths: paths,
	}
	budget := EndpointBudget{Class: "GET /concurrent", Steady: 100, Interval: time.Second, Burst: 8, MaxAttempts: 3, RetryBudget: 10 * time.Second}
	limiters := make([]*EndpointLimiter, 8)
	for index := range limiters {
		limiters[index], err = NewEndpointLimiter(session, budget)
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	errorsFound := make(chan error, len(limiters))
	var group sync.WaitGroup
	for _, limiter := range limiters {
		group.Add(1)
		go func(limiter *EndpointLimiter) {
			defer group.Done()
			<-start
			errorsFound <- limiter.Wait(context.Background())
		}(limiter)
	}
	close(start)
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(limiters[0].path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger limiterLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("concurrent ledger is invalid JSON: %v", err)
	}
	if ledger.Tokens < 0 || ledger.Tokens > float64(budget.Burst) || ledger.UpdatedAt.IsZero() {
		t.Fatalf("concurrent ledger = %+v", ledger)
	}
}
