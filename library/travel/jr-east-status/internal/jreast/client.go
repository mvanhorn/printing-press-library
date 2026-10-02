package jreast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/cliutil"
)

const MaxBodyBytes = 1 << 20
const MaxRequestBudget = 18

type Client struct {
	HTTP        *http.Client
	Limiter     *cliutil.AdaptiveLimiter
	MaxRequests int
	requests    atomic.Int32
}

// NewClient creates a bounded anonymous client; HTTP remains injectable for fixture tests.
func NewClient(maxRequests int, rate float64) *Client {
	if rate <= 0 || rate > 2 {
		rate = 2
	}
	return &Client{HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("source redirect limit exceeded")
		}
		if !allowedURL(req.URL) {
			return fmt.Errorf("source redirected outside first-party JR East hosts")
		}
		return nil
	}}, Limiter: cliutil.NewAdaptiveLimiter(rate), MaxRequests: maxRequests}
}

func allowedURL(u *url.URL) bool {
	return u.Scheme == "https" && (u.Host == "traininfo.jreast.co.jp" || u.Host == "www.jreast.co.jp") && u.User == nil
}

// Get reads one source page. It never converts access denial, throttle or oversized content to no data.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	if c.MaxRequests < 1 || c.MaxRequests > MaxRequestBudget {
		return nil, fmt.Errorf("source request budget must be between 1 and %d", MaxRequestBudget)
	}
	u, e := url.Parse(rawURL)
	if e != nil || !allowedURL(u) {
		return nil, fmt.Errorf("unsupported first-party source URL")
	}
	for {
		current := c.requests.Load()
		if int(current) >= c.MaxRequests {
			return nil, fmt.Errorf("source request budget (%d) exhausted", c.MaxRequests)
		}
		if c.requests.CompareAndSwap(current, current+1) {
			break
		}
	}
	if e = c.Limiter.Wait(ctx); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "jr-east-status-pp-cli/0.1 (public status)")
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, fmt.Errorf("JR East source request: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.Limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JR East source %s returned HTTP %d; open its source URL or retry later", u.Path, resp.StatusCode)
	}
	c.Limiter.OnSuccess()
	body, e := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes+1))
	if e != nil {
		return nil, fmt.Errorf("JR East body read: %w", e)
	}
	if len(body) > MaxBodyBytes {
		return nil, fmt.Errorf("JR East source body exceeded %d bytes", MaxBodyBytes)
	}
	return body, nil
}

func (c *Client) RequestCount() int { return int(c.requests.Load()) }

// Region fetches the original Japanese page and translated English identity, retaining failures explicitly.
func (c *Client) Region(ctx context.Context, r Region, now time.Time, staleAfter time.Duration) (Snapshot, error) {
	ja, e := c.Get(ctx, r.SourceJA)
	if e != nil {
		return Snapshot{}, e
	}
	jp, e := ParseRegion(ja, r, "ja", r.SourceJA, now, staleAfter)
	if e != nil {
		return Snapshot{}, e
	}
	// The original source explicitly closes reporting at 02:00. Some English
	// service pages then become empty shells, while others retain older rows.
	// Do not request a translation of operational facts the original withholds.
	if len(jp.rows) == 0 && jp.State.ReportingState == "outside_reporting_hours" {
		return Snapshot{Region: r, Sources: []SourceState{jp.State}, Lines: make([]Line, 0), Warnings: []string{"Original Japanese reporting is closed; the English page was not requested."}}, nil
	}
	en, e := c.Get(ctx, r.SourceEN)
	if e != nil {
		return Snapshot{}, e
	}
	ep, e := ParseRegion(en, r, "en", r.SourceEN, now, staleAfter)
	if e != nil {
		return Snapshot{}, e
	}
	return JoinRegions(jp, ep, now), nil
}
