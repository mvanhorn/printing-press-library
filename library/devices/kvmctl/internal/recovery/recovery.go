// PATCH(library): streamer recovery parity with Python kvmctl/recovery.py
package recovery

import (
	"context"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("stream unavailable")

// Stream abstracts the streamer snapshot/nudge/open operations.
type Stream interface {
	Snapshot(context.Context) ([]byte, error)
	Nudge(context.Context) error
	Open(context.Context) error
}

type Options struct {
	Attempts int
	Delay    time.Duration
	Timeout  time.Duration
	Sleep    func(context.Context, time.Duration) error
	DryRun   bool
}

func (o Options) normalized() Options {
	if o.Attempts < 1 {
		o.Attempts = 5
	}
	if o.Delay < 0 {
		o.Delay = 0
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	return o
}

func sleepCtx(ctx context.Context, d time.Duration, sleep func(context.Context, time.Duration) error) error {
	if d <= 0 {
		return nil
	}
	if sleep != nil {
		return sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Recover revives the streamer after an OTG bounce.
//
// Sequence mirrors Python recovery.py:
//  1. probe snapshot once (tolerate single 503/ErrUnavailable),
//  2. nudge encoder via set_params desired_fps=40 quality=80 (abstracted as Nudge()),
//  3. retry snapshot until success (bounded attempts, delay),
//  4. open a fresh stream WebSocket at /api/ws?stream=1.
//
// Cancellation is honored at every sleep and operation boundary.
func Recover(parent context.Context, s Stream, opts Options) error {
	if opts.DryRun {
		return nil
	}
	if err := parent.Err(); err != nil {
		return err
	}
	o := opts.normalized()
	ctx, cancel := context.WithTimeout(parent, o.Timeout)
	defer cancel()
	if _, err := s.Snapshot(ctx); err != nil && !errors.Is(err, ErrUnavailable) {
		return err
	}
	if err := s.Nudge(ctx); err != nil {
		return err
	}
	var last error
	for i := 0; i < o.Attempts; i++ {
		if i > 0 {
			if err := sleepCtx(ctx, o.Delay, o.Sleep); err != nil {
				return err
			}
		}
		if _, err := s.Snapshot(ctx); err == nil {
			return s.Open(ctx)
		} else {
			last = err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if last == nil {
		last = ErrUnavailable
	}
	return last
}

// Default Nudge params for documentation / parity reference.
const (
	DefaultNudgeFPS     = 40
	DefaultNudgeQuality = 80
	StreamWSPath        = "/api/ws?stream=1"
)
