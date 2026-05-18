package transcripts

import (
	"fmt"
	"sync"
	"time"
)

// scrapeLimiter is a simple req/s pacer with 429-aware backoff. Kept local to
// avoid an internal/cliutil import cycle from this leaf package.
type scrapeLimiter struct {
	mu       sync.Mutex
	rate     float64
	lastTick time.Time
}

func newScrapeLimiter(ratePerSec float64) *scrapeLimiter {
	return &scrapeLimiter{rate: ratePerSec}
}

func (l *scrapeLimiter) Wait() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rate <= 0 {
		return
	}
	interval := time.Duration(float64(time.Second) / l.rate)
	if !l.lastTick.IsZero() {
		if d := time.Since(l.lastTick); d < interval {
			time.Sleep(interval - d)
		}
	}
	l.lastTick = time.Now()
}

func (l *scrapeLimiter) onThrottle() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rate /= 2
	if l.rate < 0.25 {
		l.rate = 0.25
	}
}

// RateLimitError signals YouTube throttled us after our backoff exhausted.
type RateLimitError struct {
	Source string
	After  time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("transcripts: %s throttled (retry after ~%s)", e.Source, e.After)
}
