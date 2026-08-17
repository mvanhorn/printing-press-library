// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

// Package gate serializes MCP work that can reach Woot or launch the
// companion CLI. A single process-wide gate prevents typed and mirrored tools
// from bypassing each other's pacing through concurrent fan-out.
package gate

import (
	"context"
	"sync"
	"time"
)

var processGate = make(chan struct{}, 1)

const minimumGrantInterval = 500 * time.Millisecond

var (
	grantMu   sync.Mutex
	lastGrant time.Time
)

// Acquire waits for exclusive MCP execution and returns an idempotent release
// function. Cancellation while waiting does not reserve the gate.
func Acquire(ctx context.Context) (func(), error) {
	select {
	case processGate <- struct{}{}:
		grantMu.Lock()
		wait := time.Until(lastGrant.Add(minimumGrantInterval))
		grantMu.Unlock()
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				<-processGate
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		grantMu.Lock()
		lastGrant = time.Now()
		grantMu.Unlock()
		var once sync.Once
		return func() {
			once.Do(func() { <-processGate })
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
