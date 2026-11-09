// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdaptiveLimiterWaitHonorsCancellation(t *testing.T) {
	limiter := NewAdaptiveLimiter(0.01)
	if err := limiter.WaitContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := limiter.WaitContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled wait took %s", elapsed)
	}
}
