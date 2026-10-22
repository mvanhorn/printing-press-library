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

// wait blocks until a write slot is free or ctx is done. A slot is claimed
// only when the caller is about to send, so canceled waiters never push back
// later writes.
func (p *pacer) wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.mu.Lock()
		now := time.Now()
		if !p.next.After(now) {
			p.next = now.Add(p.interval)
			p.mu.Unlock()
			return nil
		}
		delay := p.next.Sub(now)
		p.mu.Unlock()

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
