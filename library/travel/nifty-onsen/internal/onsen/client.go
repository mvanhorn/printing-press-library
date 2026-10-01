package onsen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/cliutil"
	"golang.org/x/net/html"
)

const cacheVersion = "2"

const maxResponse = 4 << 20
const cacheBytes = 32 << 20
const cacheEntries = 128

type Options struct {
	CacheDir   string
	DataSource string
	NoCache    bool
	MaxAge     time.Duration
	Rate       float64
}
type Client struct {
	HTTP     *http.Client
	options  Options
	limiter  *cliutil.AdaptiveLimiter
	requests atomic.Int64
}
type counterTransport struct {
	base    http.RoundTripper
	counter *atomic.Int64
}

func (t counterTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.counter.Add(1)
	return t.base.RoundTrip(r)
}

func New(options Options) *Client {
	if options.MaxAge <= 0 {
		options.MaxAge = 6 * time.Hour
	}
	if options.DataSource == "" {
		options.DataSource = "auto"
	}
	rate := options.Rate
	if rate < 0 {
		rate = 2
	}
	c := &Client{options: options, limiter: cliutil.NewAdaptiveLimiter(rate)}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 2
	tr.ResponseHeaderTimeout = 15 * time.Second
	jar, _ := cookiejar.New(nil)
	c.HTTP = &http.Client{Jar: jar, Transport: counterTransport{tr, &c.requests}, Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return fmt.Errorf("Nifty redirected too many times")
		}
		if r.URL.Scheme != "https" || r.URL.Host != "onsen.nifty.com" {
			return fmt.Errorf("Nifty redirected outside the public provider origin")
		}
		return nil
	}}
	return c
}
func (c *Client) Requests() int { return int(c.requests.Load()) }
func (c *Client) fetch(ctx context.Context, method, link string, ajax bool) ([]byte, string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if e := c.limiter.Wait(ctx); e != nil {
			return nil, "", e
		}
		req, e := http.NewRequestWithContext(ctx, method, link, nil)
		if e != nil {
			return nil, "", fmt.Errorf("could not create public source request")
		}
		req.Header.Set("User-Agent", "nifty-onsen-pp-cli/0.0.0-dev (public read-only discovery)")
		req.Header.Set("Accept-Language", "ja")
		if ajax {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("Origin", Origin)
			req.Header.Set("Referer", Origin+"/map/")
		}
		resp, e := c.HTTP.Do(req)
		if e != nil {
			if ctx.Err() != nil {
				return nil, "", fmt.Errorf("source request timed out/canceled: %w; retry with --timeout=45s or use --data-source=local", ctx.Err())
			}
			return nil, "", fmt.Errorf("Nifty public request failed: %s; check connectivity or use --data-source=local", transportReason(e))
		}
		final := resp.Request.URL.String()
		if resp.StatusCode == 429 {
			wait := cliutil.RetryAfter(resp)
			_ = resp.Body.Close()
			c.limiter.OnRateLimit()
			if attempt == 1 {
				return nil, "", &cliutil.RateLimitError{URL: Origin + req.URL.Path, RetryAfter: wait}
			}
			if wait <= 0 {
				wait = time.Second
			}
			if wait > 5*time.Second {
				return nil, "", &cliutil.RateLimitError{URL: Origin + req.URL.Path, RetryAfter: wait}
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, "", ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != 200 {
			_ = resp.Body.Close()
			return nil, "", fmt.Errorf("Nifty HTTP %d for %s; verify the source listing or retry later", resp.StatusCode, req.URL.Path)
		}
		b, e := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
		_ = resp.Body.Close()
		if e != nil {
			return nil, "", fmt.Errorf("reading Nifty response failed")
		}
		if len(b) > maxResponse {
			return nil, "", fmt.Errorf("Nifty response exceeded 4 MiB safety limit")
		}
		c.limiter.OnSuccess()
		return b, final, nil
	}
	return nil, "", fmt.Errorf("Nifty retry budget exhausted")
}
func transportReason(e error) string {
	var ue *url.Error
	if errors.As(e, &ue) {
		return ue.Err.Error()
	}
	return "network unavailable"
}

type cacheRecord struct {
	Version string          `json:"version"`
	At      time.Time       `json:"at"`
	URL     string          `json:"url"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) cachePath(key string) string {
	sum := sha256.Sum256([]byte("onsen-parser-" + cacheVersion + ":" + key))
	return filepath.Join(c.options.CacheDir, hex.EncodeToString(sum[:])+".json")
}
func (c *Client) readCache(key string) (cacheRecord, bool) {
	if c.options.CacheDir == "" || c.options.NoCache {
		return cacheRecord{}, false
	}
	p := c.cachePath(key)
	info, e := os.Stat(p)
	if e != nil || info.Size() > maxResponse {
		return cacheRecord{}, false
	}
	b, e := os.ReadFile(p) // #nosec G304 -- dedicated cache directory and SHA-256 request key, never a source-controlled path.
	if e != nil {
		return cacheRecord{}, false
	}
	var r cacheRecord
	if json.Unmarshal(b, &r) != nil || r.Version != cacheVersion || r.At.IsZero() || r.At.After(time.Now().Add(time.Minute)) || !json.Valid(r.Data) {
		return cacheRecord{}, false
	}
	return r, true
}
func (c *Client) saveCache(key string, r cacheRecord) error {
	if c.options.CacheDir == "" || c.options.NoCache {
		return nil
	}
	dir := c.options.CacheDir
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if len(b) > maxResponse {
		return fmt.Errorf("parsed cache entry too large")
	}
	tmp, e := os.CreateTemp(dir, ".pending-*")
	if e != nil {
		return e
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, e = tmp.Write(b); e != nil {
		_ = tmp.Close()
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	if e = os.Rename(tmp.Name(), c.cachePath(key)); e != nil {
		return e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	type item struct {
		path string
		size int64
		at   time.Time
	}
	files := []item{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, e := entry.Info()
		if e == nil {
			files = append(files, item{filepath.Join(dir, entry.Name()), info.Size(), info.ModTime()})
			total += info.Size()
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for len(files) > cacheEntries || total > cacheBytes {
		f := files[0]
		if e = os.Remove(f.path); e != nil && !os.IsNotExist(e) {
			return e
		}
		total -= f.size
		files = files[1:]
	}
	return nil
}
func (c *Client) resolve(ctx context.Context, key string, out any, live func() (any, string, error)) (Provenance, error) {
	r, ok := c.readCache(key)
	p := Provenance{Warnings: []string{}}
	apply := func(mode string) error {
		if e := json.Unmarshal(r.Data, out); e != nil {
			return fmt.Errorf("cached parse invalid: use --no-cache to refresh")
		}
		p.Freshness = freshness(r.At, mode, c.options.MaxAge)
		p.URL = r.URL
		p.Requests = c.Requests()
		return nil
	}
	if c.options.DataSource == "local" {
		if !ok {
			return p, fmt.Errorf("no parsed local cache for this exact request; run it with --data-source=live first (omit --no-cache)")
		}
		return p, apply("offline")
	}
	if c.options.DataSource == "auto" && ok && time.Since(r.At) <= c.options.MaxAge {
		return p, apply("hit")
	}
	data, link, e := live()
	if e != nil {
		// A throttle is always actionable, never converted into empty or cached success.
		var rate *cliutil.RateLimitError
		if c.options.DataSource == "auto" && ok && !errors.As(e, &rate) {
			p.Warnings = append(p.Warnings, "live refresh failed; returning stale parsed cache: "+e.Error())
			return p, apply("fallback")
		}
		return p, e
	}
	b, e := json.Marshal(data)
	if e != nil {
		return p, e
	}
	r = cacheRecord{cacheVersion, time.Now(), link, b}
	if e = c.saveCache(key, r); e != nil {
		p.Warnings = append(p.Warnings, "cache write failed; live result remains usable")
	}
	e = apply("miss")
	return p, e
}

type SearchOptions struct {
	Region  string
	Query   string
	Filters []string
	Page    int
}

func SearchURL(o SearchOptions) (string, error) {
	region, e := ResolveRegion(o.Region)
	if e != nil {
		return "", e
	}
	if o.Page < 1 || o.Page > 100 {
		return "", fmt.Errorf("--page must be 1..100")
	}
	if len([]rune(o.Query)) > 120 {
		return "", fmt.Errorf("--query must be at most 120 characters")
	}
	p, e := FilterParams(o.Filters)
	if e != nil {
		return "", e
	}
	if o.Query != "" {
		p.Set("text", o.Query)
	}
	path := "/search/"
	if region != "" {
		path = "/" + region + path
	}
	if o.Page > 1 {
		path += fmt.Sprintf("page-%d/", o.Page)
	}
	link := Origin + path
	if len(p) > 0 {
		link += "?" + p.Encode()
	}
	return link, nil
}
func (c *Client) Search(ctx context.Context, o SearchOptions) (SearchData, Provenance, error) {
	var out SearchData
	link, e := SearchURL(o)
	if e != nil {
		return out, Provenance{}, e
	}
	p, e := c.resolve(ctx, link, &out, func() (any, string, error) {
		b, u, e := c.fetch(ctx, "GET", link, false)
		if e != nil {
			return nil, "", e
		}
		v, e := ParseSearch(b, u, o.Page)
		return v, u, e
	})
	return out, p, e
}
func (c *Client) Detail(ctx context.Context, input string) (Detail, Provenance, error) {
	var out Detail
	id, e := FacilityID(input)
	if e != nil {
		return out, Provenance{}, e
	}
	link := Origin + "/cs/catalog/onsen_onsen-detail/catalog_" + id + "_1.htm"
	p, e := c.resolve(ctx, "detail:"+id, &out, func() (any, string, error) {
		b, u, e := c.fetch(ctx, "GET", link, false)
		if e != nil {
			return nil, "", e
		}
		v, e := ParseDetail(b, u, id)
		return v, u, e
	})
	return out, p, e
}
func (c *Client) Coupons(ctx context.Context, input string) (CouponData, Provenance, error) {
	var out CouponData
	id, e := FacilityID(input)
	if e != nil {
		return out, Provenance{}, e
	}
	p, e := c.resolve(ctx, "coupons:"+id, &out, func() (any, string, error) {
		v, _, e := c.Detail(ctx, id)
		if e != nil {
			return nil, "", e
		}
		link := v.URL + "coupon/"
		b, u, e := c.fetch(ctx, "GET", link, false)
		if e != nil {
			return nil, "", e
		}
		data, e := ParseCoupons(b, u, id)
		return data, u, e
	})
	return out, p, e
}

type NearbyOptions struct {
	Latitude  float64
	Longitude float64
	Zoom      int
	Query     string
	Filters   []string
}

func ValidateNearby(o NearbyOptions) error {
	if o.Latitude < 20 || o.Latitude > 46 || o.Longitude < 122 || o.Longitude > 154 {
		return fmt.Errorf("--lat/--lon must be finite Japan coordinates (lat 20..46, lon 122..154)")
	}
	if o.Latitude != o.Latitude || o.Longitude != o.Longitude {
		return fmt.Errorf("--lat/--lon must be finite numbers")
	}
	if o.Zoom < 6 || o.Zoom > 16 {
		return fmt.Errorf("--zoom must be 6..16")
	}
	if len([]rune(o.Query)) > 120 {
		return fmt.Errorf("--query must be at most 120 characters")
	}
	_, e := FilterParams(o.Filters)
	return e
}
func (c *Client) Nearby(ctx context.Context, o NearbyOptions) ([]Facility, Provenance, error) {
	var out []Facility
	if e := ValidateNearby(o); e != nil {
		return out, Provenance{}, e
	}
	filters, _ := FilterParams(o.Filters)
	key := fmt.Sprintf("nearby:%.6f:%.6f:%d:%s:%s", o.Latitude, o.Longitude, o.Zoom, o.Query, filters.Encode())
	p, e := c.resolve(ctx, key, &out, func() (any, string, error) {
		b, _, e := c.fetch(ctx, "GET", Origin+"/map/", false)
		if e != nil {
			return nil, "", e
		}
		d, e := doc(b)
		if e != nil {
			return nil, "", e
		}
		node := first(d, func(n *html.Node) bool { return attr(n, "id") == "apikey" })
		token := attr(node, "value")
		if token == "" {
			return nil, "", fmt.Errorf("Nifty public map page has no query context; use bath search --region instead")
		}
		p := url.Values{"geo_center_lat": {strconv.FormatFloat(o.Latitude, 'f', 6, 64)}, "geo_center_lon": {strconv.FormatFloat(o.Longitude, 'f', 6, 64)}, "map_zoom_level": {strconv.Itoa(o.Zoom - 1)}, "search_result_max": {"20"}, "api_key": {token}}
		if cond := conditionParams(filters); cond != "" {
			p.Set("search_cond_id", cond)
		}
		if o.Query != "" {
			p.Set("search_keyword", o.Query)
		}
		b, _, e = c.fetch(ctx, "POST", Origin+"/android_app/api/get_detail_search_data.jsp?"+p.Encode(), true)
		if e != nil {
			return nil, "", e
		}
		v, e := ParseNearby(b, o.Latitude, o.Longitude)
		return v, Origin + "/map/", e
	})
	return out, p, e
}
