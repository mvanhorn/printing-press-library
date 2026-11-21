package diggmcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
)

func TestAsRefusalReadsDiggsTwoResetForms(t *testing.T) {
	now := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		err  error
		want string // RFC 3339, or "" for no time
	}{
		{"retry after", &Error{Tool: "digg_get_feed", Message: "Digg MCP is temporarily unavailable. Retry after 2026-11-21T19:05:00.000Z"}, "2026-11-21T19:05:00Z"},
		{"search resets", &Error{Tool: "digg_search_posts", Message: "This client has used its per-minute search allowance on Digg MCP. Search resets at 2026-11-21T19:01:30Z; do not call digg_search_posts again before then"}, "2026-11-21T19:01:30Z"},
		{"early retry, no time", &Error{Tool: "digg_search_posts", Message: "This client searched again before its last search refusal reset, and early retries make the wait longer"}, ""},
		{"http 429 status", &Error{Tool: "digg_get_feed", Status: 429, Message: "HTTP 429: slow down"}, ""},
		{"rate limit error", &cliutil.RateLimitError{URL: "u", RetryAfter: 30 * time.Second}, "2026-11-21T19:00:30Z"},
	}
	for _, tc := range cases {
		r, ok := AsRefusal(tc.err, now)
		if !ok {
			t.Errorf("%s: not seen as a refusal", tc.name)
			continue
		}
		got := ""
		if !r.ResetAt.IsZero() {
			got = r.ResetAt.Format(time.RFC3339)
		}
		if got != tc.want {
			t.Errorf("%s: reset %q, want %q", tc.name, got, tc.want)
		}
	}
	if _, ok := AsRefusal(&Error{Tool: "digg_get_post", Message: "Post not found"}, now); ok {
		t.Error("an ordinary tool error is not a refusal")
	}
	if _, ok := AsRefusal(errors.New("dial tcp: refused"), now); ok {
		t.Error("a network error is not a refusal")
	}
}

func TestNoRetryReturnsUncappedResetOnFirst429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		// Ten minutes: past cliutil.MaxRetryWait, which the retrying path caps.
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	now := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	c := &Client{Endpoint: srv.URL, NoRetry: true, now: func() time.Time { return now }}
	_, err := c.Call(context.Background(), "digg_get_feed", nil)
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("want *Refusal, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("NoRetry must send one request, sent %d", calls.Load())
	}
	if want := now.Add(10 * time.Minute); !r.ResetAt.Equal(want) {
		t.Fatalf("reset %s, want %s", r.ResetAt, want)
	}
}
