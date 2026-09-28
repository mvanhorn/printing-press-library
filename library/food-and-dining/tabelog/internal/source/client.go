package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
)

const Origin = "https://tabelog.com"
const BodyLimit = 4 << 20

var sourceLimiter = cliutil.NewAdaptiveLimiter(2)

type Error struct {
	Kind    string
	Message string
	Choices []domain.Choice
	Meta    domain.Meta
}

func (e *Error) Error() string    { return e.Message }
func fail(kind, msg string) error { return &Error{Kind: kind, Message: msg} }

type Client struct {
	HTTP      *http.Client
	CacheDir  string
	Mode      string
	TestBase  *url.URL
	mu        sync.Mutex
	Requests  int
	Bytes     int64
	Cached    bool
	Stale     bool
	FetchedAt time.Time
	LastURL   string
}

func NewClient(cacheDir, mode string) (*Client, error) {
	if mode != "auto" && mode != "live" && mode != "local" {
		return nil, fail("usage", "--data-source must be auto, live, or local")
	}
	c := &Client{CacheDir: cacheDir, Mode: mode, HTTP: &http.Client{Timeout: 20 * time.Second}}
	if base := os.Getenv("TABELOG_TEST_BASE_URL"); base != "" {
		u, e := url.Parse(base)
		if e != nil || os.Getenv("TABELOG_TEST_MODE") != "1" || u.User != nil || u.Scheme != "http" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
			return nil, fail("usage", "test transport requires TABELOG_TEST_MODE=1 and an HTTP loopback origin")
		}
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() || u.Port() == "" {
			return nil, fail("usage", "test transport origin must be a loopback IP with an explicit port")
		}
		c.TestBase = u
	}
	c.HTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fail("redirect", "source exceeded five redirects")
		}
		if c.TestBase != nil {
			if req.URL.Host != c.TestBase.Host || req.URL.Scheme != "http" {
				return fail("redirect", "test replay attempted a non-loopback redirect")
			}
			return nil
		}
		return ValidateSourceURL(req.URL.String())
	}
	return c, nil
}

func ValidateSourceURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "tabelog.com" || u.User != nil || !strings.HasPrefix(u.Path, "/en/") || strings.Contains(u.Path, "..") {
		return fail("usage", "use an HTTPS tabelog.com/en/ source URL")
	}
	return nil
}

func (c *Client) Fetch(ctx context.Context, raw string, ttl time.Duration) ([]byte, time.Time, error) {
	if e := ValidateSourceURL(raw); e != nil {
		return nil, time.Time{}, e
	}
	if c.Mode != "live" {
		b, t, e := c.cacheRead(raw)
		if e == nil && (c.Mode == "local" || time.Since(t) <= ttl) {
			c.mu.Lock()
			c.Cached = true
			c.Stale = c.Stale || time.Since(t) > ttl
			if c.FetchedAt.IsZero() || t.Before(c.FetchedAt) {
				c.FetchedAt = t
			}
			c.LastURL = raw
			c.mu.Unlock()
			return b, t, nil
		}
		if c.Mode == "local" {
			return nil, time.Time{}, fail("cache_miss", "no cached source response; fetch once with --data-source live")
		}
	}
	if e := sourceLimiter.Wait(ctx); e != nil {
		return nil, time.Time{}, e
	}
	target, e := url.Parse(raw)
	if e != nil {
		return nil, time.Time{}, e
	}
	if c.TestBase != nil {
		target.Scheme = c.TestBase.Scheme
		target.Host = c.TestBase.Host
	}
	req, e := http.NewRequestWithContext(ctx, "GET", target.String(), nil)
	if e != nil {
		return nil, time.Time{}, e
	}
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	if strings.HasPrefix(target.Path, "/en/suggest/") {
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Referer", Origin+"/en/")
	}
	c.mu.Lock()
	c.Requests++
	c.mu.Unlock()
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, time.Time{}, fmt.Errorf("fetch source: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		sourceLimiter.OnRateLimit()
		return nil, time.Time{}, &cliutil.RateLimitError{URL: raw, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, time.Time{}, fail("http_status", fmt.Sprintf("source returned HTTP%d; no retry or stale fallback was used", resp.StatusCode))
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, BodyLimit+1))
	if e != nil {
		return nil, time.Time{}, fmt.Errorf("read source: %w", e)
	}
	if len(b) > BodyLimit {
		return nil, time.Time{}, fail("body_limit", "source response exceeds decoded 4MiB limit")
	}
	c.mu.Lock()
	c.Bytes += int64(len(b))
	c.mu.Unlock()
	if len(b) == 0 {
		return nil, time.Time{}, fail("empty_body", "source returned an empty response")
	}
	if IsChallenge(b) {
		return nil, time.Time{}, fail("blocked", "source returned a challenge/access-denied page; no successful empty result was produced")
	}
	sourceLimiter.OnSuccess()
	now := time.Now().UTC()
	c.mu.Lock()
	if c.FetchedAt.IsZero() || now.Before(c.FetchedAt) {
		c.FetchedAt = now
	}
	c.LastURL = raw
	c.mu.Unlock()
	return b, now, nil
}

func (c *Client) FetchDetail(ctx context.Context, raw string) (domain.Restaurant, error) {
	canonical, _, _, e := RestaurantURL(raw)
	if e != nil {
		return domain.Restaurant{}, e
	}
	b, t, e := c.Fetch(ctx, canonical, 6*time.Hour)
	if e != nil {
		return domain.Restaurant{}, e
	}
	r, e := ParseDetail(b, canonical, t)
	if e == nil {
		if cacheErr := c.cacheWrite(canonical, b, t); cacheErr != nil {
			return domain.Restaurant{}, cacheErr
		}
	}
	return r, e
}

func (c *Client) Suggestions(ctx context.Context, query string) ([]domain.Choice, error) {
	raw := Origin + "/en/suggest/keyword_suggest?keyword=" + url.QueryEscape(query)
	b, t, e := c.Fetch(ctx, raw, 15*time.Minute)
	if e != nil {
		return nil, e
	}
	var rows []struct {
		Name     string          `json:"name"`
		Datatype string          `json:"datatype"`
		ID       json.RawMessage `json:"id_in_datatype"`
		Exact    bool            `json:"exact_match"`
		Pal      string          `json:"pal"`
		Area1    string          `json:"LstPrf"`
		Area2    string          `json:"LstAre"`
		Station  string          `json:"station_id"`
		Genre    string          `json:"site_name"`
		URL      string          `json:"url"`
	}
	if e = json.Unmarshal(b, &rows); e != nil {
		return nil, fail("parser_drift", "source suggestions were not the observed JSON array")
	}
	if e = c.cacheWrite(raw, b, t); e != nil {
		return nil, e
	}
	out := make([]domain.Choice, 0, len(rows))
	for _, r := range rows {
		kind := ""
		switch r.Datatype {
		case "RailroadStation":
			kind = "station"
		case "Prefecture":
			kind = "prefecture"
		case "Area1", "Area2", "AddressMaster":
			kind = "area"
		case "Genre0", "Genre1", "Genre2", "Genre3":
			kind = "cuisine"
		case "AreaRestaurant":
			kind = "restaurant"
		default:
			continue
		}
		sel := r.Datatype + ":" + strings.Trim(string(r.ID), "\"")
		u := r.URL
		if kind == "station" || kind == "area" || kind == "prefecture" {
			q := url.Values{}
			q.Set("pal", r.Pal)
			if r.Area1 != "" {
				q.Set("LstPrf", r.Area1)
			}
			if r.Area2 != "" {
				q.Set("LstAre", r.Area2)
			}
			if r.Station != "" {
				q.Set("station_id", r.Station)
			}
			u = Origin + "/en/rstLst/?" + q.Encode()
		}
		out = append(out, domain.Choice{Name: r.Name, Kind: kind, Selector: sel, URL: u, Prefecture: r.Pal, Area1: r.Area1, Area2: r.Area2, StationID: r.Station, Genre: r.Genre, Exact: r.Exact})
	}
	return out, nil
}

func (c *Client) Meta() domain.Meta {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := "live"
	if c.Requests == 0 && !c.Cached {
		s = "catalogue"
	}
	if c.Cached && c.Requests == 0 {
		s = "cache"
	}
	if c.Mode == "local" {
		s = "local"
	}
	at, raw := c.FetchedAt, c.LastURL
	if at.IsZero() {
		at = bootstrap.CapturedAt
		raw = Origin + "/en/"
	}
	age := int64(time.Since(at).Seconds())
	if age < 0 {
		age = 0
	}
	return domain.Meta{Source: s, Requests: c.Requests, Bytes: c.Bytes, Stale: c.Stale, FetchedAt: at, AgeSeconds: age, SourceURL: raw}
}
