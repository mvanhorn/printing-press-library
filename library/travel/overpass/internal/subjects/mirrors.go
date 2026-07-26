// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package subjects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/cliutil"
)

// Mirrors are the public Overpass instances, in preference order.
//
// Failover is not a nicety. On 2026-07-26 three of these four refused the same
// query with "the server is probably too busy" while the fourth served it in
// under a second, and which one is healthy changes by the hour. A client
// pinned to a single host is broken most of the time through no fault of the
// query.
// Every entry must carry PLANET data. overpass.osm.ch was in this list until
// it silently broke a search: it answers happily and quickly, but it hosts a
// Swiss regional extract, so a California query came back with zero results
// while a Swiss one returned data. An empty result from a regional mirror is
// indistinguishable from "there is nothing there", which makes a fast regional
// instance far more dangerous in a failover list than a dead one.
var Mirrors = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
	"https://overpass.private.coffee/api/interpreter",
}

// UserAgent identifies the client. Both Overpass and Nominatim require a
// descriptive one and will refuse or throttle anonymous traffic.
const UserAgent = "overpass-pp-cli (+https://github.com/mvanhorn/printing-press-library)"

// Attempt records what one mirror did, so a failure can be explained rather
// than reported as a bare error.
type Attempt struct {
	Mirror string        `json:"mirror"`
	Status int           `json:"status"`
	Err    string        `json:"error,omitempty"`
	Took   time.Duration `json:"-"`
	TookMS int64         `json:"took_ms"`
}

// DefaultRateLimit is the outbound pace for Overpass traffic, in requests per
// second. Overpass asks for moderate use and answers a burst with HTTP 429 or
// a slot-exhausted status page; Nominatim's published limit is one request per
// second. One request per second satisfies both and costs an interactive
// invocation nothing, since a single search is one request.
const DefaultRateLimit = 1.0

// DefaultProbeTimeout bounds a single mirror status probe.
//
// The status endpoint is a few lines of plain text, so a mirror that has not
// answered in this long is not healthy in any sense the caller cares about.
// Without a bound of its own the probe inherits the query timeout, and
// `mirrors` then reports on a mirror's behalf for a minute — observed on
// 2026-07-26, when one mirror took 56s to answer its own status page and the
// command that exists to say "which mirror is fast" took 60s to say it.
const DefaultProbeTimeout = 5 * time.Second

// Runner executes Overpass queries with failover.
type Runner struct {
	HTTP    *http.Client
	Mirrors []string
	// Limiter paces outbound requests and halves the rate on a 429. Nil
	// disables pacing; the methods are nil-safe.
	Limiter *cliutil.AdaptiveLimiter
	// ProbeTimeout bounds one CheckMirrors status probe. Zero uses
	// DefaultProbeTimeout.
	ProbeTimeout time.Duration
}

// NewRunner builds a runner with a per-request timeout.
func NewRunner(timeout time.Duration) *Runner {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &Runner{
		HTTP:         &http.Client{Timeout: timeout},
		Mirrors:      Mirrors,
		Limiter:      cliutil.NewAdaptiveLimiter(DefaultRateLimit),
		ProbeTimeout: DefaultProbeTimeout,
	}
}

func (r *Runner) probeTimeout() time.Duration {
	if r.ProbeTimeout > 0 {
		return r.ProbeTimeout
	}
	return DefaultProbeTimeout
}

// Run sends a query, trying each mirror until one answers with JSON.
//
// Overpass signals overload with an HTTP 200 carrying an HTML error page, so a
// status check alone is not enough — the body has to be inspected before a
// response can be called a success.
func (r *Runner) Run(ctx context.Context, query string) ([]byte, []Attempt, error) {
	var attempts []Attempt
	mirrors := r.Mirrors
	if len(mirrors) == 0 {
		mirrors = Mirrors
	}

	// throttled remembers the first typed 429 so exhausting the mirror list
	// under load reports "rate limited" rather than the generic refusal.
	// Throttling that reads as "there is nothing there" is the failure mode
	// worth spending an error message on.
	var throttled *cliutil.RateLimitError

	for _, m := range mirrors {
		start := time.Now()
		a := Attempt{Mirror: m}

		body, status, err := r.post(ctx, m, query)
		a.Status = status
		a.Took = time.Since(start)
		a.TookMS = a.Took.Milliseconds()

		var rl *cliutil.RateLimitError
		if errors.As(err, &rl) && throttled == nil {
			throttled = rl
		}

		switch {
		case err != nil:
			a.Err = err.Error()
		case status != http.StatusOK:
			a.Err = fmt.Sprintf("HTTP %d", status)
		case looksLikeHTML(body):
			// A 200 with an HTML body is Overpass's overload page.
			a.Err = "overloaded: " + extractOverpassError(body)
		default:
			attempts = append(attempts, a)
			return body, attempts, nil
		}
		attempts = append(attempts, a)

		if ctx.Err() != nil {
			return nil, attempts, ctx.Err()
		}
	}

	if throttled != nil {
		return nil, attempts, fmt.Errorf("every Overpass mirror refused the query (%d tried) and at least one throttled the client: %w", len(attempts), throttled)
	}
	return nil, attempts, fmt.Errorf("every Overpass mirror refused the query (%d tried); run `overpass-pp-cli mirrors` to see which are healthy", len(attempts))
}

func (r *Runner) post(ctx context.Context, mirror, query string) ([]byte, int, error) {
	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mirror, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	r.Limiter.Wait()
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// A 429 is returned as a typed error rather than a bare status so callers
	// can tell throttling apart from "this mirror has no data". Both otherwise
	// arrive as an empty result, and the two demand opposite responses.
	if resp.StatusCode == http.StatusTooManyRequests {
		r.Limiter.OnRateLimit()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		// Overpass answers a 429 with a full XHTML page. Passing that body
		// through verbatim put a doctype and a <strong style="color:#FF0000">
		// into the error line — twice, since the attempt is recorded and then
		// wrapped again in the final error. extractOverpassError already
		// knows how to pull the one useful sentence out of that page.
		return nil, resp.StatusCode, &cliutil.RateLimitError{
			URL:        mirror,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       humanReadableErrorBody(snippet),
		}
	}
	if resp.StatusCode == http.StatusOK {
		r.Limiter.OnSuccess()
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return body, resp.StatusCode, err
}

// humanReadableErrorBody reduces a mirror's error body to something that fits
// on an error line: the extracted reason when the body is an HTML/XHTML error
// page, a bounded single line otherwise.
func humanReadableErrorBody(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '<' {
		return extractOverpassError(trimmed)
	}
	s := strings.Join(strings.Fields(string(trimmed)), " ")
	if len(s) > 220 {
		s = s[:220] + "..."
	}
	return s
}

func looksLikeHTML(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return true
	}
	return trimmed[0] == '<'
}

// extractOverpassError pulls the human-readable reason out of an HTML error
// page so the user sees "the server is too busy" rather than a wall of markup.
func extractOverpassError(body []byte) string {
	s := string(body)
	i := strings.Index(s, "Error")
	if i < 0 {
		return "unrecognised error page"
	}
	rest := s[i:]
	if j := strings.Index(rest, "</p>"); j > 0 {
		rest = rest[:j]
	}
	rest = stripTags(rest)
	rest = strings.Join(strings.Fields(rest), " ")
	if len(rest) > 220 {
		rest = rest[:220] + "..."
	}
	return rest
}

func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// MirrorStatus is one mirror's health.
type MirrorStatus struct {
	Mirror    string `json:"mirror"`
	Healthy   bool   `json:"healthy"`
	SlotsFree int    `json:"slots_free"`
	RateLimit int    `json:"rate_limit"`
	Detail    string `json:"detail,omitempty"`
	TookMS    int64  `json:"took_ms"`
}

// CheckMirrors queries each mirror's status endpoint.
//
// Probes run concurrently, each under its own ProbeTimeout. Sequential probes
// made the slowest mirror set the command's runtime — the one mirror worth
// avoiding decided how long it took to learn to avoid it. Concurrency does not
// need the limiter: this is one request to each of a handful of distinct
// hosts, not repeated traffic to any single one.
//
// Results stay in Mirrors order, so the output still reads as the failover
// preference order rather than as a race finish.
func (r *Runner) CheckMirrors(ctx context.Context) []MirrorStatus {
	mirrors := r.Mirrors
	if len(mirrors) == 0 {
		mirrors = Mirrors
	}
	out := make([]MirrorStatus, len(mirrors))

	var wg sync.WaitGroup
	for i, m := range mirrors {
		wg.Add(1)
		go func(i int, m string) {
			defer wg.Done()
			out[i] = r.probeMirror(ctx, m)
		}(i, m)
	}
	wg.Wait()
	return out
}

func (r *Runner) probeMirror(ctx context.Context, mirror string) MirrorStatus {
	statusURL := strings.TrimSuffix(mirror, "/interpreter") + "/status"
	st := MirrorStatus{Mirror: mirror}
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, r.probeTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := r.HTTP.Do(req)
	st.TookMS = time.Since(start).Milliseconds()
	if err != nil {
		// Distinguish "this host did not answer in time" from "this host
		// refused the connection". A bare "context deadline exceeded" reads
		// as a client bug; the mirror being too slow to be worth using is the
		// actual finding, and it is the one the caller acts on.
		if errors.Is(err, context.DeadlineExceeded) {
			st.Detail = fmt.Sprintf("no answer within the %s probe timeout", r.probeTimeout())
		} else {
			st.Detail = err.Error()
		}
		return st
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		r.Limiter.OnRateLimit()
		st.Detail = (&cliutil.RateLimitError{URL: statusURL, RetryAfter: cliutil.RetryAfter(resp)}).Error()
		return st
	}
	if resp.StatusCode != http.StatusOK {
		st.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return st
	}
	st.Healthy = true
	st.RateLimit, st.SlotsFree, st.Detail = parseStatus(string(body))
	return st
}

// parseStatus reads Overpass's plain-text status page.
func parseStatus(s string) (rateLimit, slotsFree int, detail string) {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Rate limit:"):
			fmt.Sscanf(line, "Rate limit: %d", &rateLimit)
		case strings.Contains(line, "slots available now"):
			fmt.Sscanf(line, "%d slots available now", &slotsFree)
		case strings.Contains(line, "Currently running queries"):
			detail = "queries running"
		}
	}
	return rateLimit, slotsFree, detail
}
