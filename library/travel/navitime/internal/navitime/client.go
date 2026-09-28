package navitime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const maxBodyBytes = 8 << 20
const sourceBase = "https://japantravel.navitime.com"

type Client struct {
	HTTP    *http.Client
	BaseURL string
	opts    Options
	limiter *cliutil.AdaptiveLimiter
	gate    chan struct{}
	mu      sync.Mutex
	metrics Metrics
	now     func() time.Time
}

func NewClient(opts Options) (*Client, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.CacheDir == "" && !opts.NoCache {
		dir, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		opts.CacheDir = filepath.Join(dir, "navitime-pp-cli", "public-v1")
	}
	h, err := NewHTTPClient(opts.Timeout)
	if err != nil {
		return nil, err
	}
	h.Jar = nil
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{HTTP: h, BaseURL: sourceBase, opts: opts, limiter: cliutil.NewAdaptiveLimiter(1), gate: make(chan struct{}, 1), now: time.Now}, nil
}
func (c *Client) Metrics() Metrics       { c.mu.Lock(); defer c.mu.Unlock(); return c.metrics }
func (c *Client) count(f func(*Metrics)) { c.mu.Lock(); defer c.mu.Unlock(); f(&c.metrics) }
func (c *Client) metadata(u string) Metadata {
	return Metadata{SourceURL: u, FetchedAt: c.now().UTC(), DataKind: "route_options", SourceUpdatedAt: nil}
}
func (c *Client) fetch(ctx context.Context, u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			var rate *cliutil.RateLimitError
			if errors.As(last, &rate) {
				rate.Cause = err
				return nil, rate
			}
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		switch req.URL.Path {
		case "/en/area/jp/route/result/":
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		case "/en/async/route/autocomplete":
			req.Header.Set("Accept", "application/json")
		}
		c.count(func(m *Metrics) {
			m.Requests++
			if attempt > 0 {
				m.Retries++
			}
		})
		resp, err := c.HTTP.Do(req)
		wait := 250 * time.Millisecond
		retry := false
		if err != nil {
			var ne net.Error
			retry = errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) && ctx.Err() == nil
			last = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
			_ = resp.Body.Close() // Cleanup must not replace the read or response-status result.
			c.count(func(m *Metrics) { m.BytesReceived += int64(len(body)) })
			if resp.StatusCode == http.StatusTooManyRequests {
				c.limiter.OnRateLimit()
				wait = sourceRetryAfter(resp)
				last = &cliutil.RateLimitError{URL: u, RetryAfter: wait}
				if wait > 2*time.Second {
					return nil, last
				}
				retry = true
			} else if resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
				last = &SourceError{Message: fmt.Sprintf("NAVITIME returned HTTP %d", resp.StatusCode), Status: resp.StatusCode, URL: u}
				retry = true
			} else if resp.StatusCode != http.StatusOK {
				return nil, &SourceError{Message: fmt.Sprintf("NAVITIME returned HTTP %d; public data unavailable", resp.StatusCode), Status: resp.StatusCode, URL: u}
			} else if readErr != nil {
				last = readErr
				retry = errors.Is(readErr, io.ErrUnexpectedEOF) && ctx.Err() == nil
			} else if len(body) > maxBodyBytes {
				return nil, &SourceError{Message: "NAVITIME response exceeds the 8 MiB limit", URL: u}
			} else {
				c.limiter.OnSuccess()
				return body, nil
			}
		}
		if !retry || attempt == 1 {
			return nil, last
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			var rate *cliutil.RateLimitError
			if errors.As(last, &rate) {
				rate.Cause = ctx.Err()
				return nil, rate
			}
			return nil, ctx.Err()
		}
	}
	return nil, last
}
func sourceRetryAfter(resp *http.Response) time.Duration {
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds >= 0 {
		if seconds > int64((1<<63-1)/time.Second) {
			return time.Duration(1<<63 - 1)
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(raw); err == nil {
		wait := time.Until(at)
		if wait < 0 {
			return 0
		}
		return wait
	}
	return cliutil.RetryAfter(resp)
}
func (c *Client) Places(ctx context.Context, query, kind string, limit int) (PlacesResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 200 {
		return PlacesResult{}, &ArgumentError{"place query must contain 1–200 characters"}
	}
	if limit < 1 || limit > 10 {
		return PlacesResult{}, &ArgumentError{"place limit must be between 1 and 10"}
	}
	types := "spot.station.airport.port"
	maxNodes := "10"
	switch kind {
	case "", "all":
		kind = "all"
	case "station":
		types = "station"
	case "spot":
		types = "spot"
		maxNodes = "3"
	default:
		return PlacesResult{}, &ArgumentError{"place type must be station, spot, or all"}
	}
	sourceLimit := limit
	if sourceLimit < 2 {
		sourceLimit = 2
	}
	params := url.Values{"word": {query}, "types": {types}, "multilingual": {"true"}, "maxNodes": {maxNodes}, "limit": {fmt.Sprint(sourceLimit)}}
	u := strings.TrimRight(c.BaseURL, "/") + "/en/async/route/autocomplete?" + params.Encode()
	var result PlacesResult
	cacheKey := u + "#display=" + fmt.Sprint(limit)
	if c.readCache("places", cacheKey, 24*time.Hour, &result) {
		c.markHit(&result.Meta)
		return result, nil
	}
	body, err := c.fetch(ctx, u)
	if err != nil {
		return result, err
	}
	places, err := parsePlaces(body, u)
	if err != nil {
		return result, err
	}
	sourceCount := len(places)
	if len(places) > limit {
		places = places[:limit]
	}
	meta := c.metadata(u)
	meta.DataKind = "location_candidates"
	result = PlacesResult{Meta: meta, Query: query, Kind: kind, Places: places, Ambiguous: sourceCount > 1, SourceCandidateCount: sourceCount, SourceLimit: sourceLimit, Notes: []string{"Candidates are source-ranked and bounded; uniqueness and pagination are not established by this response."}}
	if sourceCount > len(places) {
		result.Notes = append(result.Notes, "Displayed candidates are truncated; ambiguity is based on the larger source response.")
	}
	if err = c.writeCache("places", cacheKey, result); err != nil {
		return result, err
	}
	return result, nil
}
func (c *Client) Routes(ctx context.Context, q Query) (RouteResult, error) {
	params, _, err := queryParams(q)
	if err != nil {
		return RouteResult{}, err
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/en/area/jp/route/result/?" + params.Encode()
	var result RouteResult
	if c.readCache("routes", u, 5*time.Minute, &result) {
		result.Query = q
		if err := c.restoreCachedRouteSnapshots(result); err != nil {
			return result, err
		}
		c.markHit(&result.Meta)
		return result, nil
	}
	body, err := c.fetch(ctx, u)
	if err != nil {
		return result, err
	}
	routes, passes, err := parseRoutes(body, q, u)
	if err != nil {
		return result, err
	}
	result = RouteResult{Meta: c.metadata(u), Query: q, Routes: routes, Notes: []string{"Route timing_basis distinguishes timetable services and source road/walking estimates; live status and seat availability are unavailable.", "Overall calendar timestamps retain seconds; leg clocks have minute precision. duration_minutes floors exact duration_seconds.", "Displayed cash fares do not establish pass-holder out-of-pocket cost."}}
	if c.opts.NoCache {
		result.Notes = append(result.Notes, "Snapshot persistence is disabled by no-cache.")
		return result, nil
	}
	if err = c.writeCache("routes", u, result); err != nil {
		return result, err
	}
	if len(passes) > 0 {
		meta := result.Meta
		meta.DataKind = "pass_catalogue"
		catalog := PassResult{Meta: meta, Passes: passes, Notes: catalogNotes()}
		if err = c.writeCache("catalog", "catalog", catalog); err != nil {
			return result, err
		}
	}
	for i := range routes {
		snapshot := routeSnapshot(result.Meta, &routes[i])
		if err = c.writeCache("snapshot", routes[i].ID, snapshot); err != nil {
			return result, err
		}
	}
	if len(routes) > 0 {
		if err = c.writeCache("latest", "latest", routes[0].ID); err != nil {
			return result, err
		}
	}
	return result, nil
}
func routeSnapshot(meta Metadata, route *Route) DetailResult {
	return DetailResult{Meta: meta, Route: route, StoredSnapshot: true, Notes: []string{"Stored public route snapshot; no network request or live availability check."}}
}
func (c *Client) restoreCachedRouteSnapshots(result RouteResult) error {
	for i := range result.Routes {
		route := &result.Routes[i]
		var stored DetailResult
		if c.readStored("snapshot", route.ID, 0, &stored) && stored.Route != nil && stored.Route.ID == route.ID {
			continue
		}
		if err := c.writeCache("snapshot", route.ID, routeSnapshot(result.Meta, route)); err != nil {
			return err
		}
	}
	if len(result.Routes) == 0 {
		return nil
	}
	var latest string
	if c.readStored("latest", "latest", 0, &latest) && latest == result.Routes[0].ID {
		return nil
	}
	return c.writeCache("latest", "latest", result.Routes[0].ID)
}
func catalogNotes() []string {
	return []string{"IDs and labels are source-advertised; only japan_rail_pass has representative live verification.", "Anonymous queries support one pass; multiple selections require source membership.", "Coverage labels do not determine pass-holder cost or purchase value."}
}
func (c *Client) Passes(ctx context.Context) (PassResult, error) {
	var catalog PassResult
	if c.readCache("catalog", "catalog", 24*time.Hour, &catalog) {
		c.markHit(&catalog.Meta)
		return catalog, nil
	}
	date := c.now().In(japan).AddDate(0, 0, 1).Format("2006-01-02")
	q := Query{From: "station:00006668", To: "station:00001756", DepartAt: date + "T09:00"}
	p, _, _ := queryParams(q)
	u := strings.TrimRight(c.BaseURL, "/") + "/en/area/jp/route/result/?" + p.Encode()
	body, err := c.fetch(ctx, u)
	if err != nil {
		return catalog, err
	}
	_, passes, err := parseRoutes(body, q, u)
	if err != nil {
		return catalog, err
	}
	if len(passes) == 0 {
		return catalog, &SourceError{Message: "NAVITIME route page has no advertised pass catalog", URL: u}
	}
	meta := c.metadata(u)
	meta.DataKind = "pass_catalogue"
	catalog = PassResult{Meta: meta, Passes: passes, Notes: catalogNotes()}
	if err = c.writeCache("catalog", "catalog", catalog); err != nil {
		return catalog, err
	}
	return catalog, nil
}
func (c *Client) Show(ctx context.Context, id string) (DetailResult, error) {
	if err := ctx.Err(); err != nil {
		return DetailResult{}, err
	}
	if id == "latest" {
		var latest string
		if !c.readStored("latest", "latest", 0, &latest) {
			note := "No stored route snapshot is available under the current cache policy."
			if c.opts.NoCache {
				note = "Snapshot reads are disabled by no-cache."
			}
			return DetailResult{Meta: Metadata{DataKind: "local_snapshot"}, Route: nil, StoredSnapshot: true, Notes: []string{note}}, nil
		}
		id = latest
	}
	if !regexpLocalID.MatchString(id) {
		return DetailResult{}, &NotFoundError{"stored route snapshot was not found"}
	}
	var result DetailResult
	if !c.readStored("snapshot", id, 0, &result) || result.Route == nil {
		return result, &NotFoundError{"stored route snapshot was not found"}
	}
	c.markHit(&result.Meta)
	return result, nil
}

type cacheEnvelope struct {
	Version   int             `json:"version"`
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) cachePath(kind, key string) string {
	h := sha256.Sum256([]byte(kind + "\x00" + key))
	return filepath.Join(c.opts.CacheDir, hex.EncodeToString(h[:])+".json")
}
func (c *Client) readCache(kind, key string, ttl time.Duration, out any) bool {
	if c.opts.Refresh {
		return false
	}
	return c.readStored(kind, key, ttl, out)
}
func (c *Client) readStored(kind, key string, ttl time.Duration, out any) bool {
	if c.opts.NoCache || c.opts.CacheDir == "" {
		return false
	}
	f, err := os.Open(c.cachePath(kind, key))
	if err != nil {
		return false
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return false
	}
	var e cacheEnvelope
	if json.Unmarshal(body, &e) != nil || e.Version != 1 || e.FetchedAt.IsZero() || e.FetchedAt.After(c.now()) || (ttl > 0 && c.now().Sub(e.FetchedAt) > ttl) {
		return false
	}
	return json.Unmarshal(e.Data, out) == nil
}
func (c *Client) markHit(m *Metadata) {
	m.CacheHit = true
	m.AgeSeconds = c.now().Sub(m.FetchedAt).Seconds()
	if m.AgeSeconds < 0 {
		m.AgeSeconds = 0
	}
	c.count(func(x *Metrics) { x.CacheHits++ })
}
func (c *Client) writeCache(kind, key string, data any) error {
	if c.opts.NoCache || c.opts.CacheDir == "" {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	body, err := json.Marshal(cacheEnvelope{1, c.now().UTC(), raw})
	if err != nil {
		return err
	}
	if err = os.MkdirAll(c.opts.CacheDir, 0700); err != nil {
		return fmt.Errorf("create public cache: %w", err)
	}
	f, err := os.CreateTemp(c.opts.CacheDir, ".public-response-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(body); err != nil {
		_ = f.Close() // Preserve the primary write error during cleanup.
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), c.cachePath(kind, key)); err != nil {
		return err
	}
	c.count(func(m *Metrics) { m.CacheWrites++ })
	return nil
}
