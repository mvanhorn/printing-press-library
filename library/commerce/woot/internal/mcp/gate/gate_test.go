// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package gate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAcquireHonorsCancellationAndReleaseIsIdempotent(t *testing.T) {
	release, err := Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked Acquire error = %v, want deadline exceeded", err)
	}
	release()
	release()

	nextRelease, err := Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	nextRelease()
}

func TestAcquirePacesConsecutiveGrants(t *testing.T) {
	first, err := Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	first()

	started := time.Now()
	second, err := Acquire(context.Background())
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	defer second()
	if elapsed := time.Since(started); elapsed < minimumGrantInterval-50*time.Millisecond {
		t.Fatalf("consecutive grant elapsed = %v, want at least %v", elapsed, minimumGrantInterval)
	}
}
