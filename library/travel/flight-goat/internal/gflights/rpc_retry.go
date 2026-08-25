// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	rpcBlockedRetryDelays = []time.Duration{
		250 * time.Millisecond,
		750 * time.Millisecond,
		1500 * time.Millisecond,
	}
	// PATCH(amend-2026-07-31): HTTP 429 gets its own, much slower ladder.
	// Google's rate limit is IP-level and often outlasts any in-process
	// wait, but transient 429s do clear within seconds — one bounded pass
	// (~19s worst case) converts those without hand-rolled retry scripts.
	// Persistent blocks still fail fast enough to surface the exit-7 hint.
	rpcRateLimitRetryDelays = []time.Duration{
		2 * time.Second,
		5 * time.Second,
		12 * time.Second,
	}
	sleepBeforeRPCBlockedRetry = sleepContext
	// rateLimitNotice is a seam for tests; production writes to stderr so
	// agents see why the command is pausing instead of a silent hang.
	rateLimitNotice = func(wait time.Duration, attempt, max int) {
		fmt.Fprintf(os.Stderr, "google flights rate limited (HTTP 429); waiting %s before retry %d/%d\n", wait, attempt, max)
	}
)

// retryBlockedRPC retries transient blocked (code-13) envelopes and HTTP 429s
// on their own delay ladders. Once a 429 has been observed, the rate-limited
// state is terminal for this run: a later blocked envelope must not spend the
// blocked ladder (up to three more requests against an exhausted IP budget),
// and any later non-context failure keeps the rate-limit classification so
// the CLI still exits 7 with the pacing hint.
func retryBlockedRPC(ctx context.Context, call func() error) error {
	blockedAttempt, rateLimitAttempt := 0, 0
	for {
		err := call()
		switch {
		case err == nil:
			return nil
		case errors.Is(err, ErrRateLimited):
			if rateLimitAttempt >= len(rpcRateLimitRetryDelays) {
				return err
			}
			wait := rpcRateLimitRetryDelays[rateLimitAttempt]
			rateLimitNotice(wait, rateLimitAttempt+1, len(rpcRateLimitRetryDelays))
			if sleepErr := sleepBeforeRPCBlockedRetry(ctx, wait); sleepErr != nil {
				return sleepErr
			}
			rateLimitAttempt++
		case errors.Is(err, errShoppingBlocked):
			// PATCH(review-2026-08-01): after a 429, do NOT start the blocked
			// ladder — return immediately, classified as rate limited, so the
			// caller neither retries nor falls back to HTML on the same host.
			if rateLimitAttempt > 0 {
				return fmt.Errorf("google flights RPC blocked after an earlier rate limit: %w", ErrRateLimited)
			}
			if blockedAttempt >= len(rpcBlockedRetryDelays) {
				return err
			}
			if sleepErr := sleepBeforeRPCBlockedRetry(ctx, rpcBlockedRetryDelays[blockedAttempt]); sleepErr != nil {
				return sleepErr
			}
			blockedAttempt++
		default:
			// PATCH(review-2026-08-01): a generic failure (HTTP 500, malformed
			// response) after a 429 must not lose the rate-limit
			// classification — otherwise the CLI exits 1 instead of 7 and the
			// hint never shows. Context errors stay untouched so cancellation
			// and deadlines keep their own semantics.
			if rateLimitAttempt > 0 && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return errors.Join(err, ErrRateLimited)
			}
			return err
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
