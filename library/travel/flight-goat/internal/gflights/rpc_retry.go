// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"context"
	"errors"
	"time"
)

var (
	rpcBlockedRetryDelays = []time.Duration{
		250 * time.Millisecond,
		750 * time.Millisecond,
		1500 * time.Millisecond,
	}
	sleepBeforeRPCBlockedRetry = sleepContext
)

func retryBlockedRPC(ctx context.Context, call func() error) error {
	for attempt := 0; ; attempt++ {
		err := call()
		if !errors.Is(err, errShoppingBlocked) {
			return err
		}
		if attempt >= len(rpcBlockedRetryDelays) {
			return err
		}
		if sleepErr := sleepBeforeRPCBlockedRetry(ctx, rpcBlockedRetryDelays[attempt]); sleepErr != nil {
			return sleepErr
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
