// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"sync"
	"time"
)

// writeInterval spaces every FedEx write sent by this MCP server, across all
// concurrent tool calls, below FedEx's per-IP throttle (sustained 1 req/s for
// two minutes triggers a temporary 403 ban). Unlike the adaptive limiter it
// never ramps up.
const writeInterval = 1250 * time.Millisecond

type pacer struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

var writePacer = &pacer{interval: writeInterval}

// wait blocks until the next write slot or until ctx is done.
func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	slot := p.next
	if slot.Before(now) {
		slot = now
	}
	p.next = slot.Add(p.interval)
	p.mu.Unlock()

	delay := time.Until(slot)
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
