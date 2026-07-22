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
