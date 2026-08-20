// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryBlockedRPCRetriesTransientBlockedEnvelope(t *testing.T) {
	origSleep := sleepBeforeRPCBlockedRetry
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { return nil }
	defer func() { sleepBeforeRPCBlockedRetry = origSleep }()

	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errShoppingBlocked
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryBlockedRPC returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryBlockedRPCStopsBeforeNonBlockedErrors(t *testing.T) {
	origSleep := sleepBeforeRPCBlockedRetry
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { return nil }
	defer func() { sleepBeforeRPCBlockedRetry = origSleep }()

	want := errors.New("parse drift")
	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("retryBlockedRPC error = %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryBlockedRPCReturnsBlockedAfterRetryBudget(t *testing.T) {
	origSleep := sleepBeforeRPCBlockedRetry
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { return nil }
	defer func() { sleepBeforeRPCBlockedRetry = origSleep }()

	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		attempts++
		return errShoppingBlocked
	})
	if !errors.Is(err, errShoppingBlocked) {
		t.Fatalf("retryBlockedRPC error = %v, want errShoppingBlocked", err)
	}
	if attempts != len(rpcBlockedRetryDelays)+1 {
		t.Fatalf("attempts = %d, want %d", attempts, len(rpcBlockedRetryDelays)+1)
	}
}

// PATCH(amend-2026-07-31): HTTP 429 gets its own slower ladder and a stderr
// notice per wait; transient 429s recover, persistent ones exhaust the budget
// and propagate the sentinel for exit-code classification.
func TestRetryBlockedRPCRetriesTransientRateLimit(t *testing.T) {
	origSleep, origNotice := sleepBeforeRPCBlockedRetry, rateLimitNotice
	var waits []time.Duration
	sleepBeforeRPCBlockedRetry = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	notices := 0
	rateLimitNotice = func(time.Duration, int, int) { notices++ }
	defer func() { sleepBeforeRPCBlockedRetry, rateLimitNotice = origSleep, origNotice }()

	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return ErrRateLimited
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryBlockedRPC returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if notices != 2 {
		t.Fatalf("notices = %d, want 2", notices)
	}
	if len(waits) != 2 || waits[0] != rpcRateLimitRetryDelays[0] || waits[1] != rpcRateLimitRetryDelays[1] {
		t.Fatalf("waits = %v, want first two of %v", waits, rpcRateLimitRetryDelays)
	}
}

func TestRetryBlockedRPCReturnsRateLimitedAfterRetryBudget(t *testing.T) {
	origSleep, origNotice := sleepBeforeRPCBlockedRetry, rateLimitNotice
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { return nil }
	rateLimitNotice = func(time.Duration, int, int) {}
	defer func() { sleepBeforeRPCBlockedRetry, rateLimitNotice = origSleep, origNotice }()

	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		attempts++
		return ErrRateLimited
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("retryBlockedRPC error = %v, want ErrRateLimited", err)
	}
	if attempts != len(rpcRateLimitRetryDelays)+1 {
		t.Fatalf("attempts = %d, want %d", attempts, len(rpcRateLimitRetryDelays)+1)
	}
}

// PATCH(review-2026-08-01): once a 429 was seen, the rate-limited state is
// terminal — the first blocked envelope after it must return immediately
// (no blocked ladder against an exhausted IP budget), classified as
// rate-limited so flights_native never enters the HTML fallback.
func TestRetryBlockedRPCBlockedAfterRateLimitIsTerminal(t *testing.T) {
	origSleep, origNotice := sleepBeforeRPCBlockedRetry, rateLimitNotice
	sleeps := 0
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { sleeps++; return nil }
	rateLimitNotice = func(time.Duration, int, int) {}
	defer func() { sleepBeforeRPCBlockedRetry, rateLimitNotice = origSleep, origNotice }()

	seq := []error{ErrRateLimited, errShoppingBlocked}
	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		defer func() { attempts++ }()
		return seq[attempts]
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
	if errors.Is(err, errShoppingBlocked) {
		t.Fatalf("classified error must not read as errShoppingBlocked (would trigger HTML fallback): %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (no blocked ladder after a 429)", attempts)
	}
	if sleeps != 1 {
		t.Fatalf("sleeps = %d, want 1 (only the 429 retry wait)", sleeps)
	}
}

// PATCH(review-2026-08-01): a generic failure after a 429 keeps the
// rate-limit classification (exit 7, hint) while preserving the original
// error in the chain; context errors stay untouched.
func TestRetryBlockedRPCGenericAfterRateLimitKeepsClassification(t *testing.T) {
	origSleep, origNotice := sleepBeforeRPCBlockedRetry, rateLimitNotice
	sleepBeforeRPCBlockedRetry = func(context.Context, time.Duration) error { return nil }
	rateLimitNotice = func(time.Duration, int, int) {}
	defer func() { sleepBeforeRPCBlockedRetry, rateLimitNotice = origSleep, origNotice }()

	boom := errors.New("shopping endpoint returned HTTP 500")
	seq := []error{ErrRateLimited, boom}
	attempts := 0
	err := retryBlockedRPC(context.Background(), func() error {
		defer func() { attempts++ }()
		return seq[attempts]
	})
	if !errors.Is(err, ErrRateLimited) || !errors.Is(err, boom) {
		t.Fatalf("error = %v, want both ErrRateLimited and the original failure in the chain", err)
	}

	attempts = 0
	seq = []error{ErrRateLimited, context.DeadlineExceeded}
	err = retryBlockedRPC(context.Background(), func() error {
		defer func() { attempts++ }()
		return seq[attempts]
	})
	if errors.Is(err, ErrRateLimited) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context errors must pass through unclassified; got %v", err)
	}
}
