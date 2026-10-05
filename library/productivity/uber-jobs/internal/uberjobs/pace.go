// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
)

// MinRequestGap is the gap the gate keeps between any two requests this CLI
// sends, across every process on the machine: the 3 s floor plus 0.5 s for
// send jitter, so gaps measured at the server (about 20 ms of jitter seen in
// testing) and second-resolution request logs both stay at or above 3 s.
// UBER_JOBS_MIN_GAP may raise it, never lower it.
const MinRequestGap = 3500 * time.Millisecond

// Gate serializes outbound requests through a lock file in the state dir and
// keeps at least Gap between them, even across separate CLI processes.
type Gate struct {
	Dir string
	Gap time.Duration
	// now and sleep are test seams.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// NewGate returns the machine-wide gate rooted at dir.
func NewGate(dir string) *Gate {
	gap := MinRequestGap
	if raw := strings.TrimSpace(os.Getenv("UBER_JOBS_MIN_GAP")); raw != "" {
		if d, err := cliutil.ParseDurationLoose(raw); err == nil && d > gap {
			gap = d
		}
	}
	return &Gate{Dir: dir, Gap: gap}
}

func (g *Gate) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *Gate) pause(ctx context.Context, d time.Duration) error {
	if g.sleep != nil {
		return g.sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gate) lastPath() string { return filepath.Join(g.Dir, "last-request-unixnano") }

// Wait blocks until Gap has passed since the last request recorded by any
// process, then records now as the new last-request time. The lock is held
// while waiting so concurrent processes form one sequential stream.
func (g *Gate) Wait(ctx context.Context) error { return g.Do(ctx, nil, nil) }

// Do waits like Wait, then runs check and, when check passes, records the
// request time and runs send, all while still holding the lock. Holding it
// through send keeps the stream strictly sequential: a refusal latched by one
// process's request is on disk before the next process runs its check, so a
// process queued at the gate never sends to a host that just refused. A check
// error returns without recording a request.
func (g *Gate) Do(ctx context.Context, check, send func() error) error {
	if g == nil || g.Dir == "" {
		return runGated(ctx, check, nil, send)
	}
	if err := os.MkdirAll(g.Dir, 0o700); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	return cliutil.WithFileLock(filepath.Join(g.Dir, "request.lock"), func() error {
		// The lock can take a while to get; a request whose deadline passed
		// meanwhile sends nothing, so it must not stamp the gate either.
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := g.waitGap(ctx); err != nil {
			return err
		}
		return runGated(ctx, check, g.stamp, send)
	})
}

// runGated runs check, stamp, and send in order, stopping at the first
// error; an expired ctx runs none of them.
func runGated(ctx context.Context, check, stamp, send func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, step := range []func() error{check, stamp, send} {
		if step != nil {
			if err := step(); err != nil {
				return err
			}
		}
	}
	return nil
}

// waitGap pauses until Gap has passed since the recorded last request.
func (g *Gate) waitGap(ctx context.Context) error {
	raw, err := os.ReadFile(g.lastPath())
	if err != nil {
		return nil
	}
	ns, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return nil
	}
	wait := g.Gap - g.clock().Sub(time.Unix(0, ns))
	// A stamp ahead of the clock means the clock moved back; one full gap
	// from now keeps the spacing without waiting out the whole step.
	if wait > g.Gap {
		wait = g.Gap
	}
	if wait <= 0 {
		return nil
	}
	return g.pause(ctx, wait)
}

// stamp records now as the last request time.
func (g *Gate) stamp() error {
	return os.WriteFile(g.lastPath(), []byte(strconv.FormatInt(g.clock().UnixNano(), 10)), 0o600)
}

// LastRequest returns the recorded last-request time, if any.
func (g *Gate) LastRequest() (time.Time, bool) {
	raw, err := os.ReadFile(g.lastPath())
	if err != nil {
		return time.Time{}, false
	}
	ns, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, ns), true
}

// RecordRefusal appends a refusal line to the state dir and latches the host,
// so every later process stops sending to it, not just the one that saw it.
func RecordRefusal(dir string, r *RefusalError) {
	if dir == "" || r == nil {
		return
	}
	_ = os.MkdirAll(dir, 0o700)
	now := time.Now().UTC()
	if f, err := os.OpenFile(filepath.Clean(filepath.Join(dir, "refusals.tsv")), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		fmt.Fprintf(f, "%s\t%s\t%d\tchallenge=%v\n", now.Format(time.RFC3339), r.URL, r.Status, r.Challenge)
		_ = f.Close()
	}
	if r.Latched || r.Host == "" {
		return
	}
	l := refusalLatch{Host: r.Host, URL: r.URL, Status: r.Status, Challenge: r.Challenge, At: now.Format(time.RFC3339), Until: latchUntil(now).Format(time.RFC3339)}
	if b, err := json.Marshal(l); err == nil {
		_ = os.WriteFile(latchPath(dir, r.Host), b, 0o600)
	}
}

// refusalLatch is the on-disk record that a host refused a request.
type refusalLatch struct {
	Host      string `json:"host"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Challenge bool   `json:"challenge"`
	At        string `json:"at"`
	Until     string `json:"until"`
}

// RefusalCooldownEnv may set how long a refusal latch holds (a duration such
// as 1h). Without it, a latch holds until the end of the UTC day it was set.
const RefusalCooldownEnv = "UBER_JOBS_REFUSAL_COOLDOWN"

// latchUntil is when a latch set at `at` expires; the floor is one minute.
func latchUntil(at time.Time) time.Time {
	if raw := strings.TrimSpace(os.Getenv(RefusalCooldownEnv)); raw != "" {
		if d, err := cliutil.ParseDurationLoose(raw); err == nil && d > 0 {
			if d < time.Minute {
				d = time.Minute
			}
			return at.UTC().Add(d)
		}
	}
	y, m, d := at.UTC().Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)
}

func latchPath(dir, host string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, strings.ToLower(host))
	return filepath.Join(dir, "refused-"+safe+".json")
}

// LatchedRefusal returns the refusal any process recorded for host while its
// cooldown lasts, or nil. An unreadable latch fails closed until the end of
// the UTC day its file was written.
func LatchedRefusal(dir, host string, now time.Time) *RefusalError {
	if dir == "" || host == "" {
		return nil
	}
	path := latchPath(dir, host)
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil
	}
	var l refusalLatch
	var until time.Time
	parsed := json.Unmarshal(raw, &l) == nil
	if parsed {
		var perr error
		until, perr = time.Parse(time.RFC3339, l.Until)
		parsed = perr == nil
	}
	if !parsed {
		written := now.UTC()
		if info, serr := os.Stat(path); serr == nil {
			written = info.ModTime().UTC()
		}
		y, m, d := written.Date()
		until = time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)
		l = refusalLatch{Host: host, At: written.Format(time.RFC3339), Until: until.Format(time.RFC3339)}
	}
	if !now.Before(until) {
		return nil
	}
	r := newRefusal(l.URL, host, l.Status, l.Challenge, "")
	r.Latched, r.LatchedAt, r.Until, r.LatchFile = true, l.At, l.Until, path
	return r
}
