package cliutil

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// UpstreamHTMLMaxAttempts bounds how many times a niftyindices fetch is tried
// when the site answers a CSV or JSON endpoint with an HTML page.
const UpstreamHTMLMaxAttempts = 5

// UpstreamHTMLBackoffBase and UpstreamHTMLBackoffMax shape the wait between
// attempts: base*2^n capped at max, then jittered into [d/2, d].
var (
	UpstreamHTMLBackoffBase = 2 * time.Second
	UpstreamHTMLBackoffMax  = 10 * time.Second
)

// UpstreamHTMLError reports that an endpoint that should return CSV or JSON
// kept returning an HTML page. It is not an authentication failure.
type UpstreamHTMLError struct {
	URL      string
	Expected string
	Attempts int
}

func (e *UpstreamHTMLError) Error() string {
	return fmt.Sprintf("niftyindices returned an HTML page instead of the %s after %d attempts (upstream block or error page; an unknown slug or index name also gets one): %s", e.Expected, e.Attempts, e.URL)
}

// LooksLikeHTML reports whether a response is an HTML page, by content type
// or by a body that starts with '<'.
func LooksLikeHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	for _, b := range body {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '<':
			return true
		}
		return false
	}
	return false
}

// UpstreamHTMLBackoff returns the wait before the next attempt, where attempt
// is the zero-based index of the attempt that just failed.
func UpstreamHTMLBackoff(attempt int) time.Duration {
	d := UpstreamHTMLBackoffBase << uint(min(attempt, 16))
	if d <= 0 || d > UpstreamHTMLBackoffMax {
		d = UpstreamHTMLBackoffMax
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

// SleepContext waits for d or until ctx is done.
func SleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
