package diggmcp

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
)

// Refusal is Digg declining a call until a stated time: an HTTP 429, or a
// tool error such as "Digg MCP is temporarily unavailable. Retry after
// <time>" or "... Search resets at <time>". Digg says an early retry makes
// the wait longer, so a caller should not call again before ResetAt.
type Refusal struct {
	Tool string
	// ResetAt is when Digg said to come back, in UTC. Zero when Digg
	// named no time.
	ResetAt time.Time
	Message string
}

func (r *Refusal) Error() string {
	msg := r.Tool + ": Digg refused the call"
	if !r.ResetAt.IsZero() {
		msg += "; resets at " + r.ResetAt.UTC().Format(time.RFC3339)
	}
	if r.Message != "" {
		msg += ": " + r.Message
	}
	return msg
}

// resetTimeRe matches the two times Digg names in a refusal.
var resetTimeRe = regexp.MustCompile(`(?i)(?:retry after|resets at)\s+(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)`)

// refusalPhrases mark a tool error as a refusal even when it names no time.
var refusalPhrases = []string{
	"temporarily unavailable",
	"allowance",
	"early retries",
	"too many requests",
	"rate limit",
}

// AsRefusal reports whether err is Digg declining a call, and when it said
// to come back. now turns a relative wait into a time.
func AsRefusal(err error, now time.Time) (*Refusal, bool) {
	if err == nil {
		return nil, false
	}
	var r *Refusal
	if errors.As(err, &r) {
		return r, true
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Status == http.StatusTooManyRequests || isRefusalText(e.Message) {
			return &Refusal{Tool: e.Tool, ResetAt: resetFromText(e.Message), Message: e.Message}, true
		}
		return nil, false
	}
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		out := &Refusal{Message: strings.TrimSpace(rl.Body), ResetAt: resetFromText(rl.Body)}
		if out.ResetAt.IsZero() && rl.RetryAfter > 0 {
			out.ResetAt = now.Add(rl.RetryAfter).UTC()
		}
		return out, true
	}
	return nil, false
}

func isRefusalText(msg string) bool {
	if resetTimeRe.MatchString(msg) {
		return true
	}
	lower := strings.ToLower(msg)
	for _, p := range refusalPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// resetFromText returns the time named in a refusal message, or zero.
func resetFromText(msg string) time.Time {
	m := resetTimeRe.FindStringSubmatch(msg)
	if m == nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, m[1])
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// refusalFrom429 builds the Refusal for an HTTP 429. The Retry-After
// header wins; a time in the body is the fallback. Unlike the retrying
// path, the wait is not capped: the caller is told the real reset time.
func refusalFrom429(tool, retryAfter string, raw []byte, now time.Time) *Refusal {
	msg := clip(StripSSE(raw), 300)
	if _, err := Unwrap(tool, raw); err != nil {
		var e *Error
		if errors.As(err, &e) && e.Message != "" {
			msg = e.Message
		}
	}
	reset := resetFromHeader(retryAfter, now)
	if reset.IsZero() {
		reset = resetFromText(msg)
	}
	return &Refusal{Tool: tool, ResetAt: reset, Message: msg}
}

// resetFromHeader reads Retry-After as seconds, an HTTP date or an
// RFC 3339 time. It returns zero for an empty or unreadable value.
func resetFromHeader(v string, now time.Time) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs < 0 {
			return time.Time{}
		}
		return now.Add(time.Duration(secs) * time.Second).UTC()
	}
	if t, err := http.ParseTime(v); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// ResetIn renders a Refusal's reset for people: the time and how far off.
func (r *Refusal) ResetIn(now time.Time) string {
	if r.ResetAt.IsZero() {
		return "unknown"
	}
	d := r.ResetAt.Sub(now).Round(time.Second)
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%s (in %s)", r.ResetAt.UTC().Format(time.RFC3339), d)
}
