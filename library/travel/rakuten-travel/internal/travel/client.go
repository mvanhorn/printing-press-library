package travel

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil"
	"golang.org/x/net/html/charset"
)

const maxBodyBytes int64 = 8 << 20
const metadataTTL = 24 * time.Hour

type document struct {
	Body   []byte
	Source SourceInfo
}
type Client struct {
	config    Config
	http      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	gate      chan struct{}
	statsMu   sync.Mutex
	stats     RequestStats
	lastStart time.Time
	snapshot  *document
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.InventoryTTL < 0 || cfg.InventoryTTL > 60*time.Second {
		return nil, queryError("inventory cache TTL must be between 0 and 60 seconds")
	}
	if cfg.Timeout < 0 || cfg.MaxRequests < 0 || cfg.CacheMaxEntries < 0 || cfg.CacheMaxBytes < 0 {
		return nil, queryError("timeout, request budget and cache bounds cannot be negative")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 12
	}
	if cfg.MaxRequests > 100 {
		return nil, queryError("request budget cannot exceed 100 attempts")
	}
	if cfg.CacheMaxEntries == 0 {
		cfg.CacheMaxEntries = 128
	}
	if cfg.CacheMaxBytes == 0 {
		cfg.CacheMaxBytes = 32 << 20
	}
	if cfg.CacheMaxEntries > 256 || cfg.CacheMaxBytes > 64<<20 {
		return nil, queryError("cache bound exceeds 256 entries or 64 MiB")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	injected := cfg.HTTPClient != nil
	if !injected || cfg.MinInterval == 0 {
		cfg.MinInterval = max(cfg.MinInterval, time.Second)
	}
	if cfg.MinInterval < 0 {
		if !injected {
			return nil, queryError("negative pacing requires an injected test HTTP client")
		}
		cfg.MinInterval = 0
	}
	hc := http.Client{Timeout: cfg.Timeout}
	if cfg.HTTPClient != nil {
		hc = *cfg.HTTPClient
		hc.Timeout = cfg.Timeout
	}
	hc.Jar = nil
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := &Client{config: cfg, http: &hc, gate: make(chan struct{}, 1)}
	c.gate <- struct{}{}
	if cfg.MinInterval >= time.Second {
		c.limiter = cliutil.NewAdaptiveLimiterAuto(1)
	}
	return c, nil
}
func (c *Client) Stats() RequestStats { c.statsMu.Lock(); defer c.statsMu.Unlock(); return c.stats }
func (c *Client) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.config.Timeout)
}
func (c *Client) Areas(ctx context.Context, parent string) (AreaResult, error) {
	if err := ValidateAreaParent(parent); err != nil {
		return AreaResult{}, err
	}
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	u := "https://travel.rakuten.co.jp/yado/japan.html"
	if parent != "" {
		u = "https://travel.rakuten.co.jp/yado/" + parent + "/map.html"
	}
	d, err := c.fetch(ctx, u, false)
	if err != nil {
		return AreaResult{}, err
	}
	result, err := parseAreas(d, parent)
	if err == nil {
		c.saveBestEffort(d, false)
	}
	return result, err
}
func (c *Client) SearchHotels(ctx context.Context, q HotelQuery) (HotelSearchResult, error) {
	if err := q.Validate(); err != nil {
		return HotelSearchResult{}, err
	}
	q = normalizedHotel(q)
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	u := "https://travel.rakuten.co.jp/yado/" + q.Area + ".html"
	if q.Area != "" && q.Page > 1 {
		u = "https://search.travel.rakuten.co.jp/ds/yado/" + q.Area + "-p" + integer(q.Page)
	}
	if q.Query != "" {
		u = "https://kw.travel.rakuten.co.jp/keyword/Search.do?" + url.Values{"charset": {"utf-8"}, "f_query": {q.Query}, "f_max": {"30"}, "f_next": {integer(q.Page)}}.Encode()
	}
	d, err := c.fetch(ctx, u, false)
	if err != nil {
		return HotelSearchResult{}, err
	}
	result, err := parseHotelSearch(d, q)
	if err == nil {
		c.saveBestEffort(d, false)
	}
	return result, err
}
func (c *Client) Hotel(ctx context.Context, id string) (HotelResult, error) {
	if err := ValidateHotelID(id); err != nil {
		return HotelResult{}, err
	}
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	base := "https://travel.rakuten.co.jp/HOTEL/" + id + "/" + id
	d, err := c.fetch(ctx, base+".html", false)
	if err != nil {
		return HotelResult{}, err
	}
	h, err := parseHotel(d, id, false)
	if err != nil {
		return HotelResult{}, err
	}
	c.saveBestEffort(d, false)
	result := HotelResult{Status: StatusOK, Hotel: h, Source: d.Source}
	details, err := c.fetch(ctx, base+"_std.html", false)
	if err != nil {
		return result, err
	}
	h, err = parseHotel(details, id, true)
	if err != nil {
		return result, err
	}
	if h.Name == nil {
		h.Name = result.Hotel.Name
	}
	if h.Rating == nil {
		h.Rating = result.Hotel.Rating
	}
	if h.Address == nil {
		h.Address = result.Hotel.Address
	}
	result.Hotel = h
	result.DetailsSource = details.Source
	c.saveBestEffort(details, false)
	return result, nil
}
func (c *Client) Offers(ctx context.Context, q OfferQuery) (OfferResult, error) {
	if err := q.validateAt(c.config.Now()); err != nil {
		return OfferResult{}, err
	}
	q = normalizedOffer(q)
	ctx, cancel := c.bounded(ctx)
	defer cancel()
	d, err := c.fetch(ctx, offerURL(q), true)
	if err != nil {
		return OfferResult{}, err
	}
	result, err := parseOffers(d, q)
	if err == nil {
		c.saveBestEffort(d, true)
	}
	return result, err
}
func validSourceURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch u.Hostname() {
	case "travel.rakuten.co.jp", "hotel.travel.rakuten.co.jp", "kw.travel.rakuten.co.jp", "search.travel.rakuten.co.jp":
		return true
	}
	return false
}
func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) fetch(ctx context.Context, raw string, inventory bool) (document, error) {
	if !validSourceURL(raw) {
		return document{}, sourceError("unsafe_url", raw, 0, "only approved HTTPS Rakuten document hosts are allowed", nil)
	}
	select {
	case <-ctx.Done():
		return document{}, sourceError("timeout", raw, 0, ctx.Err().Error(), ctx.Err())
	case <-c.gate:
	}
	defer func() { c.gate <- struct{}{} }()
	if inventory && c.snapshot != nil && c.snapshot.Source.URL == raw {
		d := *c.snapshot
		d.Source.CacheState = "invocation_snapshot"
		d.Source.CacheAgeSeconds = max(0, c.config.Now().Sub(d.Source.FetchedAt).Seconds())
		return d, nil
	}
	ttl := metadataTTL
	if inventory {
		ttl = c.config.InventoryTTL
	}
	if !c.config.Refresh && !c.config.NoCache && ttl > 0 && c.config.CacheDir != "" {
		if d, ok := c.load(raw, ttl); ok {
			c.statsMu.Lock()
			c.stats.CacheHits++
			c.statsMu.Unlock()
			if inventory {
				c.snapshot = &d
			}
			return d, nil
		}
	}
	var last error
	for retry := 0; retry <= 2; retry++ {
		current := raw
		for redirects := 0; redirects <= 3; redirects++ {
			if err := c.pace(ctx); err != nil {
				return document{}, sourceError("timeout", current, 0, err.Error(), err)
			}
			c.statsMu.Lock()
			if c.stats.Requests >= c.config.MaxRequests {
				c.statsMu.Unlock()
				return document{}, sourceError("request_budget", current, 0, "hard request-attempt budget exhausted", last)
			}
			c.stats.Requests++
			if retry > 0 && redirects == 0 {
				c.stats.Retries++
			}
			c.statsMu.Unlock()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
			if err != nil {
				return document{}, err
			}
			request.Header.Set("User-Agent", "rakuten-travel-pp-cli/0.1 public-read-only")
			request.Header.Set("Accept", "text/html,application/xhtml+xml")
			started := time.Now()
			response, err := c.http.Do(request)
			if err != nil {
				c.statsMu.Lock()
				c.stats.NetworkLatencyMS += time.Since(started).Milliseconds()
				c.statsMu.Unlock()
				kind := "network_error"
				if ctx.Err() != nil || isTimeout(err) {
					kind = "timeout"
				}
				last = sourceError(kind, current, 0, "public document request failed", err)
				if ctx.Err() != nil {
					return document{}, last
				}
				break
			}
			bytes, readErr := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
			// GET body read/truncation errors are handled below; Close only releases
			// transport resources and cannot change the already-read response bytes.
			_ = response.Body.Close()
			c.statsMu.Lock()
			c.stats.Bytes += int64(len(bytes))
			c.stats.NetworkLatencyMS += time.Since(started).Milliseconds()
			c.statsMu.Unlock()
			if len(bytes) > int(maxBodyBytes) {
				return document{}, sourceError("body_too_large", current, response.StatusCode, "HTML exceeds 8 MiB", nil)
			}
			if readErr != nil {
				last = sourceError("network_error", current, response.StatusCode, "failed while reading public document", readErr)
				break
			}
			code := response.StatusCode
			if code >= 300 && code < 400 {
				location := response.Header.Get("Location")
				u, _ := url.Parse(current)
				rel, e := url.Parse(location)
				if e != nil || location == "" {
					return document{}, sourceError("unsafe_redirect", current, code, "redirect has no valid location", nil)
				}
				target := u.ResolveReference(rel).String()
				if !validSourceURL(target) || redirects == 3 {
					return document{}, sourceError("unsafe_redirect", current, code, "redirect escaped approved HTTPS hosts or exceeded three hops", nil)
				}
				current = target
				continue
			}
			if code < 200 || code >= 300 {
				kind := "http_error"
				switch {
				case code == 404:
					kind = "not_found"
				case code == 401:
					kind = "unauthorized"
				case code == 403:
					kind = "forbidden"
				case code == 429:
					kind = "rate_limited"
				case code >= 500:
					kind = "upstream_error"
				}
				var cause error
				if code == 429 {
					cause = &cliutil.RateLimitError{URL: current, RetryAfter: retryDelay(response), Body: "public route throttled"}
				}
				last = sourceError(kind, current, code, "public document returned HTTP "+integer(code), cause)
				transient := code == 408 || code == 429 || code == 500 || code == 502 || code == 503 || code == 504
				if !transient || retry == 2 {
					return document{}, last
				}
				if code == 429 {
					c.limiter.OnRateLimit()
				}
				delay := cliutil.Backoff(retry)
				if response.Header.Get("Retry-After") != "" {
					delay = retryDelay(response)
				} else if c.config.MinInterval == 0 {
					delay = 0
				}
				if deadline, ok := ctx.Deadline(); ok && delay > time.Until(deadline) {
					return document{}, last
				}
				if e := waitContext(ctx, delay); e != nil {
					return document{}, sourceError("timeout", current, code, "deadline reached during retry wait", e)
				}
				break
			}
			c.limiter.OnSuccess()
			if remaining, reset, ok := cliutil.ParseRateLimitHeaders(response.Header); ok {
				c.limiter.ObserveHeaders(remaining, reset)
			}
			ct := response.Header.Get("Content-Type")
			if ct != "" && !strings.Contains(strings.ToLower(ct), "html") {
				return document{}, sourceError("parse_error", current, code, "public route returned a non-HTML document", nil)
			}
			if len(strings.TrimSpace(string(bytes))) == 0 {
				return document{}, parseFailure(current, "empty document")
			}
			decoder, e := charset.NewReader(strings.NewReader(string(bytes)), ct)
			if e != nil {
				return document{}, sourceError("encoding_error", current, code, "cannot decode declared HTML charset", e)
			}
			decoded, e := io.ReadAll(io.LimitReader(decoder, maxBodyBytes+1))
			if e != nil {
				return document{}, sourceError("encoding_error", current, code, "cannot decode public HTML", e)
			}
			if len(decoded) > int(maxBodyBytes) {
				return document{}, sourceError("body_too_large", current, code, "decoded HTML exceeds 8 MiB", nil)
			}
			if e = checkDocument(decoded, raw); e != nil {
				return document{}, e
			}
			now := c.config.Now()
			state := "miss"
			if c.config.Refresh {
				state = "refresh"
			}
			if c.config.NoCache || ttl == 0 || c.config.CacheDir == "" {
				state = "disabled"
				ttl = 0
			}
			d := document{Body: decoded, Source: SourceInfo{Name: "Rakuten Travel", URL: raw, FetchedAt: now, ObservedAt: now, CacheState: state, CacheTTLSeconds: int(ttl / time.Second)}}
			if inventory {
				c.snapshot = &d
			}
			return d, nil
		}
		if retry < 2 {
			if e := waitContext(ctx, 0); e != nil {
				return document{}, sourceError("timeout", raw, 0, e.Error(), e)
			}
		}
	}
	return document{}, last
}
func (c *Client) pace(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := c.config.MinInterval - time.Since(c.lastStart)
	if delay > 0 {
		if err := waitContext(ctx, delay); err != nil {
			return err
		}
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	c.lastStart = time.Now()
	return nil
}
func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	e, ok := err.(timeout)
	return ok && e.Timeout()
}

// retryDelay preserves the source wait; a deadline rejects excessive waits
// instead of retrying earlier than Retry-After permits.
func retryDelay(r *http.Response) time.Duration {
	h := strings.TrimSpace(r.Header.Get("Retry-After"))
	if h == "" {
		return 5 * time.Second
	}
	if regexp.MustCompile(`^[0-9]+$`).MatchString(h) {
		n, e := strconv.ParseUint(h, 10, 64)
		if e != nil || n > uint64((1<<63-1)/int64(time.Second)) {
			return time.Duration(1<<63 - 1)
		}
		return time.Duration(n) * time.Second
	}
	if when, e := http.ParseTime(h); e == nil {
		return max(0, time.Until(when))
	}
	return 5 * time.Second
}
