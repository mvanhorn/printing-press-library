// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package appstore is the hand-built client for Apple's public App Store
// surfaces: the iTunes Lookup and Search APIs, the legacy iTunes RSS feeds
// (charts and customer reviews), the search-hints plist and the server-side
// rendered apps.apple.com pages (product, charts, developer, Today, see-all).
//
// Every request goes through one Client that paces per host with
// cliutil.AdaptiveLimiter, sends a Chrome user agent (owner decision), stops
// the whole run at the first 403 or 429 (the governor), retries a 5xx once,
// honours Cache-Control with a file cache, follows redirects only within
// Apple's hosts and appends a capture-log row for every network request when
// a CaptureSink is attached.
package appstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

// DefaultUserAgent is the Chrome 131 desktop user agent the owner chose at
// the absorb gate (matching the sibling google-play CLI).
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// RateClass names one pacing bucket. Each class has its own limiter.
type RateClass string

const (
	// RateSearchAPI is itunes.apple.com/search (Apple documents ~20 calls/min).
	RateSearchAPI RateClass = "itunes-search"
	// RateITunes is every other itunes.apple.com path (Lookup, legacy RSS).
	RateITunes RateClass = "itunes"
	// RatePages is the server-rendered apps.apple.com pages.
	RatePages RateClass = "apps-pages"
	// RateStoreAPI is the opt-in apps.apple.com/api JSON proxy.
	RateStoreAPI RateClass = "apps-api"
	// RateHints is search.itunes.apple.com (search hints).
	RateHints RateClass = "search-hints"
)

// DefaultRates are the per-class request rates (requests per second). The
// governor state file applies them across invocations too: a new process
// paces from the previous process's last request of the class.
var DefaultRates = map[RateClass]float64{
	RateSearchAPI: 0.3,
	RateITunes:    1.0,
	// Uncached apps.apple.com pages go one per 20 s (+0-20% jitter). At one
	// a second, Apple's edge throttled 2 of 6 page fetches on 2026-10-23,
	// with 10 s and 76 s between invocations.
	RatePages:    1.0 / 20,
	RateStoreAPI: 0.5,
	RateHints:    1.0,
}

// ClassifyURL picks the pacing class for a URL by host and path, and refuses
// every host that is not one of Apple's three App Store hosts.
func ClassifyURL(u *url.URL) (RateClass, error) {
	if u == nil {
		return "", fmt.Errorf("nil URL")
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("refusing non-https App Store URL %q", u.String())
	}
	host := strings.ToLower(u.Hostname())
	path := u.Path
	switch host {
	case "itunes.apple.com":
		if path == "/search" || strings.HasPrefix(path, "/search/") {
			return RateSearchAPI, nil
		}
		return RateITunes, nil
	case "apps.apple.com":
		if path == "/api" || strings.HasPrefix(path, "/api/") {
			return RateStoreAPI, nil
		}
		return RatePages, nil
	case "search.itunes.apple.com":
		return RateHints, nil
	}
	return "", fmt.Errorf("refusing non-App Store host %q", host)
}

// optInPathPrefixes are the apps.apple.com path prefixes robots.txt
// disallows (Disallow: /api/*, /v1/*, /WebObjects/*, /includes/*).
var optInPathPrefixes = []string{"/api", "/v1", "/webobjects", "/includes"}

// OptInURL reports whether u is a robots-disallowed apps.apple.com surface
// the owner approved only as an explicit opt-in: every path under /api, /v1,
// /WebObjects or /includes, and every path whose last segment is "search"
// (Disallow: */search?*, the web search page). The client refuses such URLs,
// including as redirect targets, until EnableOptInSurfaces is called.
func OptInURL(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Hostname(), "apps.apple.com") {
		return false
	}
	p := strings.ToLower(u.Path)
	for _, prefix := range optInPathPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	p = strings.TrimRight(p, "/")
	return p == "/search" || strings.HasSuffix(p, "/search")
}

const (
	defaultJitter     = 0.2
	defaultRetryDelay = 2 * time.Second
	maxRedirects      = 4
	maxBodyBytes      = 64 << 20
	// DefaultPageTTL applies when a response carries no max-age.
	DefaultPageTTL = 15 * time.Minute
	// MaxCacheTTL caps every cache entry. Apple advertises max-age up to
	// 86,400 s on Lookup and Search, but every response feeds a time series
	// (ratings, prices, versions, keyword ranks), so a re-run more than 15
	// minutes later must observe Apple again instead of re-serving the old
	// answer. Results keep their original FetchedAt when served from cache.
	MaxCacheTTL = 15 * time.Minute
)

// Options configure a Client. The zero value is usable: storefront "us", the
// default rates, the user cache directory and the Chrome user agent.
type Options struct {
	Country   string
	Lang      string
	UserAgent string
	Timeout   time.Duration
	// NoCache bypasses the response cache for reads and writes (--no-cache).
	NoCache bool
	// CacheDir overrides the cache location (default <user cache dir>/http).
	CacheDir string
	// HTTPClient replaces the transport (tests).
	HTTPClient *http.Client
	// Rates overrides per-class rates; missing classes use DefaultRates.
	Rates map[RateClass]float64
	// MaxRate, when > 0, caps every class (the root --rate-limit flag) and
	// the client's combined rate: any two requests, of any class and across
	// processes sharing StatePath, go at least 1/MaxRate apart. It can only
	// slow the client down, never speed it past DefaultRates.
	MaxRate float64
	// Jitter spreads the requests of a class: each request after the first
	// is sent at least interval x (1 + U[0, Jitter]) after the previous one
	// (default 0.2, so the stated rate is a ceiling). Negative disables the
	// jitter; the class limiter still enforces the rate.
	Jitter float64
	// RetryDelay is the wait before the single 5xx retry (default 2 s).
	RetryDelay time.Duration
	// Capture receives one capture-log row per network request.
	Capture CaptureSink
	// Now overrides the clock (tests).
	Now func() time.Time
	// StatePath is the governor's state file: the last send slot per rate
	// class and the per-host throttle holds, shared by every process that
	// uses the same file (the CLI uses <state dir>/appstore-governor.json).
	// Empty keeps that memory in this process only.
	StatePath string
	// Notice, when set, receives a one-line message before a pacing wait of
	// 3 s or more, so a paused command does not look hung.
	Notice func(string)
	// WaitUntil, when set, is the latest send slot the client waits for: a
	// slot after it fails fast with *PacingError and nothing is sent, as one
	// past ctx's deadline does. The CLI sets it under a test harness, whose
	// per-command budget the page pacing can exceed.
	WaitUntil time.Time
}

// Client talks to Apple's public App Store surfaces. It is safe for
// concurrent use; pacing is shared across goroutines per rate class.
type Client struct {
	country   string
	lang      string
	userAgent string
	http      *http.Client
	cache     *fileCache
	noCache   bool
	jitter    float64
	retry     time.Duration
	now       func() time.Time

	limiters  map[RateClass]*cliutil.AdaptiveLimiter
	intervals map[RateClass]time.Duration

	// gov holds the send slots and throttle holds (in a state file shared
	// across processes when Options.StatePath is set).
	gov       *governor
	minGap    time.Duration // 1/MaxRate: the combined-rate floor between any two requests
	waitUntil time.Time     // Options.WaitUntil: the latest send slot this client waits for
	notice    func(string)

	mu      sync.Mutex
	capture CaptureSink
	optIn   bool
	tripped *cliutil.RateLimitError
	stats   map[RateClass]int

	// stateWarn announces, once, that the governor lost its state file.
	stateWarn sync.Once
}

// New builds a Client. An unknown Country is an error (use a two-letter
// storefront code such as us, gb or pk).
func New(opts Options) (*Client, error) {
	country := "us"
	if strings.TrimSpace(opts.Country) != "" {
		cc, err := NormalizeCountry(opts.Country)
		if err != nil {
			return nil, err
		}
		country = cc
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	} else {
		cp := *hc
		hc = &cp
	}
	if opts.Timeout > 0 {
		hc.Timeout = opts.Timeout
	}
	// Redirects are followed manually so every hop is paced, host-checked
	// and counted.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	jitter := opts.Jitter
	if jitter == 0 {
		jitter = defaultJitter
	}
	if jitter < 0 {
		jitter = 0
	}
	retry := opts.RetryDelay
	if retry <= 0 {
		retry = defaultRetryDelay
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	c := &Client{
		country:   country,
		lang:      strings.TrimSpace(opts.Lang),
		userAgent: ua,
		http:      hc,
		noCache:   opts.NoCache,
		jitter:    jitter,
		retry:     retry,
		now:       now,
		capture:   opts.Capture,
		limiters:  map[RateClass]*cliutil.AdaptiveLimiter{},
		intervals: map[RateClass]time.Duration{},
		stats:     map[RateClass]int{},
		gov:       newGovernor(opts.StatePath),
		waitUntil: opts.WaitUntil,
		notice:    opts.Notice,
	}
	if opts.MaxRate > 0 {
		c.minGap = time.Duration(float64(time.Second) / opts.MaxRate)
	}
	for class, rate := range DefaultRates {
		if r, ok := opts.Rates[class]; ok && r > 0 {
			rate = r
		}
		if opts.MaxRate > 0 && opts.MaxRate < rate {
			rate = opts.MaxRate
		}
		// NewAdaptiveLimiter makes the rate a hard ceiling: the success ramp
		// can never exceed it, and OnRateLimit halves it.
		c.limiters[class] = cliutil.NewAdaptiveLimiter(rate)
		c.intervals[class] = time.Duration(float64(time.Second) / rate)
	}
	if !opts.NoCache {
		dir := opts.CacheDir
		if dir == "" {
			base, err := cliutil.CacheDir()
			if err == nil {
				dir = filepath.Join(base, "http")
			}
		}
		if dir != "" {
			c.cache = newFileCache(dir, now)
		}
	}
	return c, nil
}

// Country is the default storefront for calls that pass an empty country.
func (c *Client) Country() string { return c.country }

// Lang is the --lang value: the l= page-language parameter of apps.apple.com
// pages and an Accept-Language hint on every request ("" when unset).
func (c *Client) Lang() string { return c.lang }

// SetCaptureSink attaches (or with nil detaches) the capture log.
func (c *Client) SetCaptureSink(sink CaptureSink) {
	c.mu.Lock()
	c.capture = sink
	c.mu.Unlock()
}

// EnableOptInSurfaces unlocks the robots-disallowed surfaces (every URL
// OptInURL reports: the web search page and apps.apple.com/api among them).
// Commands call it only behind an explicit --source web|api flag; it is
// never on by default.
func (c *Client) EnableOptInSurfaces() {
	c.mu.Lock()
	c.optIn = true
	c.mu.Unlock()
}

func (c *Client) optInEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.optIn
}

// Stats returns the number of network requests sent per rate class.
func (c *Client) Stats() map[RateClass]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[RateClass]int, len(c.stats))
	for k, v := range c.stats {
		out[k] = v
	}
	return out
}

// Requests returns the total number of network requests sent.
func (c *Client) Requests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.stats {
		n += v
	}
	return n
}

// Tripped returns the rate-limit error that stopped this client, or nil.
func (c *Client) Tripped() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tripped == nil {
		return nil
	}
	return c.tripped
}

// LimiterRate reports the current pacing rate for a class (tests, --plan).
func (c *Client) LimiterRate(class RateClass) float64 {
	return c.limiters[class].Rate()
}

func (c *Client) countRequest(class RateClass) {
	c.mu.Lock()
	c.stats[class]++
	c.mu.Unlock()
}

// fetchRequest describes one logical GET.
type fetchRequest struct {
	URL     string
	Accept  string
	Headers map[string]string
	// VaryKey is appended to the cache key for responses that depend on a
	// request header (the X-Apple-Store-Front of search hints).
	VaryKey string
	// SkipCache bypasses the cache for this request.
	SkipCache bool
}

// fetchResult is the raw answer to a fetchRequest.
type fetchResult struct {
	RequestedURL string
	FinalURL     string
	Host         string
	Status       int
	Header       http.Header
	Body         []byte
	FromCache    bool
	FetchedAt    time.Time
	CacheControl string
	XCache       string
	// Age is the response's Age header (how long Apple's CDN had held the
	// copy); AgeKnown says the header was sent.
	Age       time.Duration
	AgeKnown  bool
	cacheKeys []string
}

func (r *fetchResult) meta(outcome string, count int) FetchMeta {
	maxAge, _ := maxAgeDirective(r.CacheControl)
	return FetchMeta{
		URL:          r.RequestedURL,
		FinalURL:     r.FinalURL,
		Status:       r.Status,
		FromCache:    r.FromCache,
		FetchedAt:    r.FetchedAt,
		Outcome:      outcome,
		EntryCount:   count,
		CacheControl: r.CacheControl,
		MaxAge:       maxAge,
		Age:          r.Age,
		DataAsOf:     r.dataAsOf(),
	}
}

// lookupCDNMaxAge is the max-age Apple's iTunes Lookup and Search API edge
// sends with a fresh copy. It sends no Age header; a cached copy counts
// max-age down instead (max-age=39985 is a copy 46,415 s, about 12.9 h, old).
const lookupCDNMaxAge = 24 * time.Hour

// dataAsOf estimates when Apple produced the data in this response: the
// fetch time minus the CDN copy's age. The age is the Age header when the
// CDN sent one (apps.apple.com pages); for the iTunes Lookup and Search
// APIs it is the part of the 86,400 s max-age budget already spent; other
// surfaces count as fresh. A response from the local cache keeps both its
// fetch time and its age, so it keeps its data time too.
func (r *fetchResult) dataAsOf() time.Time {
	if r == nil || r.FetchedAt.IsZero() {
		return time.Time{}
	}
	return r.FetchedAt.Add(-r.dataAge())
}

func (r *fetchResult) dataAge() time.Duration {
	if r.AgeKnown {
		if r.Age > 0 {
			return r.Age
		}
		return 0
	}
	u, err := url.Parse(r.FinalURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "itunes.apple.com") || (u.Path != "/lookup" && u.Path != "/search") {
		return 0
	}
	if ma, ok := maxAgeDirective(r.CacheControl); ok && ma > 0 && ma < lookupCDNMaxAge {
		return lookupCDNMaxAge - ma
	}
	return 0
}

// parseAgeHeader reads an Age header (delta-seconds).
func parseAgeHeader(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	secs, err := strconv.ParseInt(v, 10, 64)
	if err != nil || secs < 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

func (c *Client) governorErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tripped == nil {
		return nil
	}
	return &cliutil.RateLimitError{
		URL:        c.tripped.URL,
		RetryAfter: c.tripped.RetryAfter,
		Body:       "governor stopped this run after an earlier throttle; no further App Store requests were sent",
		Cause:      c.tripped,
	}
}

func (c *Client) trip(err *cliutil.RateLimitError) {
	c.mu.Lock()
	if c.tripped == nil {
		c.tripped = err
	}
	c.mu.Unlock()
}

// pace holds a request to u until its class may send again. Under the
// governor's lock (a file lock when the state is shared across processes) it
//   - refuses the request when Apple throttled u's host earlier and the hold
//     (max(Retry-After, ThrottleHold)) is still in force: a 429 seen by an
//     earlier invocation is not forgotten when that process exits;
//   - reserves a send slot at least interval x (1 + U[0, Jitter]) after the
//     latest slot of the class, wherever it was reserved (this process or an
//     earlier one), and at least 1/MaxRate after the latest slot of any
//     class when --rate-limit is set, so the stated rates stay ceilings
//     across invocations and requests never fall into a fixed rhythm;
//   - refuses, without reserving, a slot that lies past ctx's deadline
//     (*PacingError), instead of sleeping into the timeout.
//
// The jitter is measured from the previous send itself, so a fast response
// cannot absorb it. The class limiter still runs (its adaptive slow-down
// included); the reserved slot is never earlier than its wait.
func (c *Client) pace(ctx context.Context, u *url.URL, class RateClass) error {
	host := strings.ToLower(u.Hostname())
	interval := c.intervals[class]
	if r := c.limiters[class].Rate(); r > 0 {
		interval = time.Duration(float64(time.Second) / r)
	}
	gap := interval
	if c.jitter > 0 {
		gap = time.Duration(float64(interval) * (1 + rand.Float64()*c.jitter)) // #nosec G404 -- pacing jitter is not cryptographic.
	}
	var slot time.Time
	var refuse error
	c.gov.update(func(st *governorState) bool {
		now := time.Now()
		if h, ok := st.activeHold(host, now); ok {
			refuse = holdError(h, u.String(), now)
			return false
		}
		slot = now
		if prev, ok := st.LastSend[class]; ok && prev.Sub(now) <= maxHoldAhead {
			if next := prev.Add(gap); next.After(slot) {
				slot = next
			}
		}
		if c.minGap > 0 && !st.LastAny.IsZero() && st.LastAny.Sub(now) <= maxHoldAhead {
			if next := st.LastAny.Add(c.minGap); next.After(slot) {
				slot = next
			}
		}
		dl, hasDeadline := ctx.Deadline()
		if hasDeadline && !now.Before(dl) {
			refuse = fmt.Errorf("no time left to send to %s: %w", host, context.DeadlineExceeded)
			return false
		}
		// A slot the run must wait for has to leave sendReserve of the
		// deadline for the answer, or Apple serves a response nobody reads.
		late := hasDeadline && slot.After(now) && slot.Add(sendReserve).After(dl)
		if late || (!c.waitUntil.IsZero() && slot.After(c.waitUntil)) {
			refuse = &PacingError{Host: host, Class: class, Wait: slot.Sub(now), Interval: interval}
			return false
		}
		st.LastSend[class] = slot
		if slot.After(st.LastAny) {
			st.LastAny = slot
		}
		return true
	})
	if err := c.gov.stateErr(); err != nil && c.notice != nil {
		c.stateWarn.Do(func() {
			c.notice(fmt.Sprintf("warning: the App Store governor cannot use its state file (%v); pacing and throttle holds now cover this run only", err))
		})
	}
	if refuse != nil {
		if rl, ok := AsRateLimit(refuse); ok {
			c.trip(rl)
		}
		return refuse
	}
	// Announce a long wait before any sleeping (the class limiter's own wait
	// included), so a paused command does not look hung.
	if wait := time.Until(slot); wait >= waitNoticeAfter && c.notice != nil {
		c.notice(fmt.Sprintf("note: waiting %s for the next %s request slot (%s requests go at least %s apart, across runs)",
			roundUpSecond(wait), host, classLabel(class), roundUpSecond(interval)))
	}
	if err := c.limiters[class].Wait(ctx); err != nil {
		return err
	}
	if err := sleepCtx(ctx, time.Until(slot)); err != nil {
		return err
	}
	// Another process may have been throttled by this host while this one
	// waited: re-read the shared holds before sending.
	var held error
	c.gov.update(func(st *governorState) bool {
		now := time.Now()
		if h, ok := st.activeHold(host, now); ok {
			held = holdError(h, u.String(), now)
		}
		return false
	})
	if held != nil {
		if rl, ok := AsRateLimit(held); ok {
			c.trip(rl)
		}
		return held
	}
	return nil
}

// sendReserve is how much of ctx's deadline a send slot the run waits for
// must leave for Apple's answer.
const sendReserve = time.Second

// throttleText is throttleSummary plus, when the governor lost its state
// file, the warning that the hold covers this run only.
func (c *Client) throttleText(status int, retryAfter time.Duration, hold HostHold, body []byte) string {
	msg := throttleSummary(status, retryAfter, hold, body)
	if c.gov.stateErr() != nil {
		msg += " (this hold covers this run only: the governor cannot write its state file)"
	}
	return msg
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func cacheKeyFor(rawURL, vary string) string {
	if vary == "" {
		return "GET " + rawURL
	}
	return "GET " + rawURL + "\n" + vary
}

// fetch performs one paced GET with governor, redirect, retry and cache
// handling. It does not log successful captures; endpoint methods call
// finish after parsing so the row carries the parsed entry count.
func (c *Client) fetch(ctx context.Context, req fetchRequest) (*fetchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid App Store URL %q: %w", req.URL, err)
	}
	class, err := ClassifyURL(u)
	if err != nil {
		return nil, err
	}
	if OptInURL(u) && !c.optInEnabled() {
		return nil, ErrOptInRequired
	}
	if err := c.governorErr(); err != nil {
		return nil, err
	}
	key := cacheKeyFor(u.String(), req.VaryKey)
	useCache := c.cache != nil && !c.noCache && !req.SkipCache
	if useCache {
		if hit, ok := c.cache.get(key); ok {
			hit.RequestedURL = u.String()
			return hit, nil
		}
	}

	current := u
	retried := false
	var retrying *HTTPError // the 5xx answer being retried, if any
	for hop := 0; ; {
		if err := c.pace(ctx, current, class); err != nil {
			var paced *PacingError
			if retrying != nil && errors.As(err, &paced) {
				// The single 5xx retry found no send slot in time: report the
				// 5xx Apple sent, not the pacing refusal.
				return nil, retrying
			}
			return nil, err
		}
		if err := c.governorErr(); err != nil {
			return nil, err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("User-Agent", c.userAgent)
		accept := req.Accept
		if accept == "" {
			accept = "*/*"
		}
		httpReq.Header.Set("Accept", accept)
		if c.lang != "" {
			httpReq.Header.Set("Accept-Language", c.lang)
		}
		for k, v := range req.Headers {
			httpReq.Header[http.CanonicalHeaderKey(k)] = []string{v}
		}
		started := c.now()
		resp, err := c.http.Do(httpReq)
		c.countRequest(class)
		if err != nil {
			c.logFailure(ctx, current, 0, nil, started, err)
			return nil, fmt.Errorf("fetching %s: %w", current.String(), err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			c.logFailure(ctx, current, resp.StatusCode, resp.Header, started, readErr)
			return nil, fmt.Errorf("reading %s: %w", current.String(), readErr)
		}
		status := resp.StatusCode
		switch {
		case status == http.StatusTooManyRequests || status == http.StatusForbidden:
			c.limiters[class].OnRateLimit()
			// Apple's full Retry-After (not cliutil's one-minute retry cap):
			// the hold is max(Retry-After, ThrottleHold).
			retryAfter := HoldRetryAfter(resp.Header, time.Now())
			// Remember the throttle beyond this process: later invocations
			// send nothing to this host until the hold ends.
			hold := c.recordHold(current.Hostname(), status, current.String(), retryAfter)
			rlErr := &cliutil.RateLimitError{
				URL:        current.String(),
				RetryAfter: roundUpSecond(time.Until(hold.Until)),
				Body:       c.throttleText(status, retryAfter, hold, body),
			}
			c.trip(rlErr)
			c.logFailure(ctx, current, status, resp.Header, started, rlErr)
			return nil, rlErr
		case status >= 300 && status < 400 && resp.Header.Get("Location") != "":
			hop++
			if hop > maxRedirects {
				err := fmt.Errorf("too many redirects fetching %s", u.String())
				c.logFailure(ctx, current, status, resp.Header, started, err)
				return nil, err
			}
			next, err := current.Parse(resp.Header.Get("Location"))
			if err != nil {
				c.logFailure(ctx, current, status, resp.Header, started, err)
				return nil, fmt.Errorf("bad redirect from %s: %w", current.String(), err)
			}
			nextClass, err := ClassifyURL(next)
			if err != nil {
				err = fmt.Errorf("refusing redirect from %s: %w", current.String(), err)
				c.logFailure(ctx, current, status, resp.Header, started, err)
				return nil, err
			}
			if OptInURL(next) && !c.optInEnabled() {
				err := fmt.Errorf("refusing redirect from %s to %s: %w", current.String(), next.String(), ErrOptInRequired)
				c.logFailure(ctx, current, status, resp.Header, started, err)
				return nil, err
			}
			retrying = nil // the retry got an answer; a later refusal is about the redirect hop
			c.limiters[class].OnSuccess()
			class = nextClass
			current = next
			continue
		case status >= 500:
			if !retried {
				retried = true
				retrying = &HTTPError{URL: current.String(), Status: status, Body: snippet(body)}
				c.logFailure(ctx, current, status, resp.Header, started, fmt.Errorf("HTTP %d (retrying once)", status))
				if err := sleepCtx(ctx, c.retry); err != nil {
					return nil, err
				}
				continue
			}
			httpErr := &HTTPError{URL: current.String(), Status: status, Body: snippet(body)}
			c.logFailure(ctx, current, status, resp.Header, started, httpErr)
			return nil, httpErr
		case status == http.StatusNotFound && notFoundMeansMissing(current):
			nf := &NotFoundError{URL: current.String()}
			c.logFailure(ctx, current, status, resp.Header, started, nf)
			return nil, nf
		case status >= 400 || status >= 300:
			// Includes a 404 from a surface that answers unknown ids with an
			// empty 200 (Lookup, Search, RSS, hints, charts, Today): there
			// a 404 is an outage, not a missing app.
			httpErr := &HTTPError{URL: current.String(), Status: status, Body: snippet(body)}
			c.logFailure(ctx, current, status, resp.Header, started, httpErr)
			return nil, httpErr
		}
		c.limiters[class].OnSuccess()
		res := &fetchResult{
			RequestedURL: u.String(),
			FinalURL:     current.String(),
			Host:         current.Hostname(),
			Status:       status,
			Header:       resp.Header,
			Body:         body,
			FetchedAt:    started,
			CacheControl: resp.Header.Get("Cache-Control"),
			XCache:       resp.Header.Get("X-Cache"),
		}
		res.Age, res.AgeKnown = parseAgeHeader(resp.Header.Get("Age"))
		if useCache {
			res.cacheKeys = []string{key}
			if current.String() != u.String() {
				res.cacheKeys = append(res.cacheKeys, cacheKeyFor(current.String(), req.VaryKey))
			}
		}
		return res, nil
	}
}

// finish records the capture-log row for a parsed network response and, when
// the result is a non-empty 200 that parsed (outcome ok), stores it in the
// cache. Callers run their typed parser first, so a body that fails it is
// logged as failed and never cached. Cache hits are not network requests and
// are not logged.
func (c *Client) finish(ctx context.Context, res *fetchResult, outcome string, count int, oldest, newest string, parseErr error) {
	if res == nil || res.FromCache {
		return
	}
	capRow := Capture{
		CapturedAt:    res.FetchedAt,
		URL:           res.FinalURL,
		Host:          res.Host,
		Status:        res.Status,
		Outcome:       outcome,
		EntryCount:    count,
		CacheControl:  res.CacheControl,
		XCache:        res.XCache,
		Oldest:        oldest,
		Newest:        newest,
		ParserVersion: ParserVersion,
	}
	if parseErr != nil {
		capRow.Outcome = OutcomeFailed
		capRow.Error = parseErr.Error()
	}
	c.record(ctx, capRow)
	if parseErr == nil && outcome == OutcomeOK && res.Status == http.StatusOK && c.cache != nil && len(res.cacheKeys) > 0 {
		if ttl := cacheTTL(res.CacheControl); ttl > 0 {
			for _, k := range res.cacheKeys {
				c.cache.put(k, res, ttl)
			}
		}
	}
}

func (c *Client) logFailure(ctx context.Context, u *url.URL, status int, header http.Header, at time.Time, err error) {
	row := Capture{
		CapturedAt:    at,
		URL:           u.String(),
		Host:          u.Hostname(),
		Status:        status,
		Outcome:       OutcomeFailed,
		ParserVersion: ParserVersion,
	}
	if header != nil {
		row.CacheControl = header.Get("Cache-Control")
		row.XCache = header.Get("X-Cache")
	}
	if err != nil {
		row.Error = err.Error()
	}
	c.record(ctx, row)
}

func (c *Client) record(ctx context.Context, row Capture) {
	c.mu.Lock()
	sink := c.capture
	c.mu.Unlock()
	if sink == nil {
		return
	}
	if row.CapturedAt.IsZero() {
		row.CapturedAt = c.now()
	}
	// A failed capture-log write never fails the fetch itself; the store
	// reports its own errors on the next write.
	_ = sink.RecordCapture(context.WithoutCancel(ctx), row)
}

func throttleSummary(status int, retryAfter time.Duration, hold HostHold, body []byte) string {
	msg := fmt.Sprintf("HTTP %d from Apple", status)
	if retryAfter > 0 {
		msg += fmt.Sprintf(" (Retry-After %s)", roundUpSecond(retryAfter))
	}
	msg += fmt.Sprintf("; the governor stopped this run (no retry) and holds every request to %s until %s", hold.Host, FormatTime(hold.Until))
	if s := snippet(body); s != "" {
		msg += ": " + s
	}
	return msg
}

func snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if strings.HasPrefix(s, "<") {
		// HTML error pages carry no useful text for a one-line error.
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// cacheTTL returns how long a response may be cached: its max-age (capped at
// MaxCacheTTL, 15 minutes), DefaultPageTTL when no max-age is sent, and 0 for
// no-store, no-cache or max-age=0.
func cacheTTL(cacheControl string) time.Duration {
	cc := strings.ToLower(cacheControl)
	if strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache") || strings.Contains(cc, "private") {
		return 0
	}
	ttl, ok := maxAgeDirective(cc)
	if !ok {
		return DefaultPageTTL
	}
	if ttl <= 0 {
		return 0
	}
	if ttl > MaxCacheTTL {
		ttl = MaxCacheTTL
	}
	return ttl
}

// maxAgeDirective reads the max-age directive of a Cache-Control header; ok
// is false when there is none. An unparsable value reads as 0.
func maxAgeDirective(cacheControl string) (time.Duration, bool) {
	for _, part := range strings.Split(strings.ToLower(cacheControl), ",") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "max-age="); ok {
			var secs int64
			if _, err := fmt.Sscanf(v, "%d", &secs); err != nil || secs <= 0 {
				return 0, true
			}
			return time.Duration(secs) * time.Second, true
		}
	}
	return 0, false
}

// notFoundMeansMissing reports whether a 404 from u means that the thing it
// names does not exist: only the id-addressed apps.apple.com resources (a
// product page and its see-all pages, a developer page, and the opt-in /api
// app resources). Every other surface answers an unknown id with an empty
// 200 (Lookup, RSS reviews) or is not id-addressed at all (Search, charts,
// Today, hints), so a 404 there is an API failure (HTTPError, exit 5); the
// research error catalogue records a transient /search 404.
func notFoundMeansMissing(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Hostname(), "apps.apple.com") {
		return false
	}
	return idAddressedPath.MatchString(strings.ToLower(u.Path))
}

// idAddressedPath matches /{cc}/app/..., /{cc}/developer/... and
// /api/apps/v1/catalog/{cc}/apps/{id}[/...].
var idAddressedPath = regexp.MustCompile(`^/(?:[a-z]{2}/(?:app|developer)/|api/apps/v1/catalog/[a-z]{2}/apps/\d+(?:/|$))`)

// AsRateLimit reports whether err is (or wraps) a *cliutil.RateLimitError.
func AsRateLimit(err error) (*cliutil.RateLimitError, bool) {
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}
