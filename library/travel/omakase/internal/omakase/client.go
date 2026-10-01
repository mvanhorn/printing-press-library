package omakase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxBody = 2 << 20
const maxCacheEntries = 128

// Bump when normalized parser semantics change; old cache must not preserve corrected financial states.
const documentCacheVersion = 3

type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("OMAKASE HTTP %d for %s; inspect the canonical website or retry later", e.Status, e.URL)
}

type Client struct {
	HTTP     *http.Client
	CacheDir string
	Refresh  bool
	Offline  bool
	NoCache  bool
	MaxAge   time.Duration
	Pace     time.Duration
	Stats    Stats
	limiter  *cliutil.AdaptiveLimiter
}
type cacheEntry struct {
	Version   int             `json:"version"`
	URL       string          `json:"url"`
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

func New(cache string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("OMAKASE redirect limit reached")
		}
		return safeURL(r.URL.String())
	}}, CacheDir: cache, MaxAge: 30 * time.Minute, Pace: 500 * time.Millisecond}
}
func safeURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "omakase.in" || u.User != nil {
		return fmt.Errorf("only first-party HTTPS omakase.in URLs are supported")
	}
	p := u.Path
	if p == "/en/r" || p == "/r" || p == "/en/premium/green" || p == "/en" || p == "/" {
		return nil
	}
	if cardRE.MatchString(p) {
		return nil
	}
	if regexpPage(p) {
		return nil
	}
	return fmt.Errorf("unsupported public OMAKASE path")
}
func regexpPage(p string) bool {
	for _, pre := range []string{"/en/r/page/", "/r/page/"} {
		if strings.HasPrefix(p, pre) {
			s := strings.TrimPrefix(p, pre)
			if s == "" {
				return false
			}
			for _, r := range s {
				if r < '0' || r > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}
func (c *Client) fetch(ctx context.Context, raw string) ([]byte, error) {
	if e := safeURL(raw); e != nil {
		return nil, e
	}
	for attempt := 0; attempt < 2; attempt++ {
		if c.limiter == nil && c.Pace > 0 {
			c.limiter = cliutil.NewAdaptiveLimiter(float64(time.Second) / float64(c.Pace))
		}
		if e := c.limiter.Wait(ctx); e != nil {
			return nil, e
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "omakase-pp-cli/0.1.0 (read-only planning)")
		req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
		c.Stats.Requests++
		resp, e := c.HTTP.Do(req)
		if e != nil {
			return nil, fmt.Errorf("OMAKASE request failed: %w", e)
		}
		body, e := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		_ = resp.Body.Close()
		c.Stats.ResponseBytes += int64(len(body))
		if e != nil {
			return nil, fmt.Errorf("OMAKASE response read: %w", e)
		}
		if len(body) > maxBody {
			return nil, fmt.Errorf("OMAKASE response exceeds 2 MiB limit")
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if resp.StatusCode == 429 {
				c.limiter.OnRateLimit()
			}
			if attempt == 0 {
				delay := time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			if resp.StatusCode == 429 {
				return nil, &cliutil.RateLimitError{URL: raw, RetryAfter: time.Second}
			}
		}
		if resp.StatusCode != 200 {
			return nil, &HTTPError{resp.StatusCode, raw}
		}
		if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
			return nil, fmt.Errorf("OMAKASE expected HTML, received a different content type")
		}
		c.limiter.OnSuccess()
		return body, nil
	}
	return nil, fmt.Errorf("OMAKASE retries exhausted")
}
func (c *Client) key(raw string) string {
	v := sha256.Sum256([]byte(raw))
	return filepath.Join(c.CacheDir, hex.EncodeToString(v[:])+".json")
}
func (c *Client) load(ctx context.Context, raw string, out any, parse func([]byte) (any, error)) (Evidence, error) {
	if e := safeURL(raw); e != nil {
		return Evidence{}, e
	}
	if c.Offline && c.Refresh {
		return Evidence{}, fmt.Errorf("offline reads cannot refresh")
	}
	ev := Evidence{URL: raw}
	if !c.NoCache && !c.Refresh {
		if b, e := readBounded(c.key(raw), maxBody); e == nil {
			var ce cacheEntry
			if json.Unmarshal(b, &ce) == nil && ce.Version == documentCacheVersion && ce.URL == raw && !ce.FetchedAt.IsZero() {
				age := time.Since(ce.FetchedAt)
				stale := age < 0 || (c.MaxAge > 0 && age > c.MaxAge)
				if (c.Offline || !stale) && json.Unmarshal(ce.Data, out) == nil {
					c.Stats.CacheHits++
					ev.FetchedAt = ce.FetchedAt
					ev.Cached = true
					ev.Stale = stale
					return ev, nil
				}
			}
		}
	}
	if c.Offline {
		return ev, fmt.Errorf("no cached OMAKASE result; fetch this item online first")
	}
	body, e := c.fetch(ctx, raw)
	if e != nil {
		return ev, e
	}
	v, e := parse(body)
	if e != nil {
		return ev, e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return ev, e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return ev, e
	}
	ev.FetchedAt = time.Now().UTC()
	if !c.NoCache {
		ce := cacheEntry{documentCacheVersion, raw, ev.FetchedAt, b}
		data, _ := json.Marshal(ce)
		if e = atomicWrite(c.key(raw), data); e != nil {
			return ev, fmt.Errorf("write OMAKASE cache: %w", e)
		}
		if e = c.prune(); e != nil {
			return ev, e
		}
	}
	return ev, nil
}
func readBounded(path string, cap int) ([]byte, error) {
	f, e := os.Open(filepath.Clean(path))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(cap+1)))
	if e != nil {
		return nil, e
	}
	if len(b) > cap {
		return nil, fmt.Errorf("local OMAKASE file exceeds size limit")
	}
	return b, nil
}
func atomicWrite(path string, data []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".omakase-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, e = f.Write(data); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (c *Client) prune() error {
	entries, e := os.ReadDir(c.CacheDir)
	if e != nil {
		return e
	}
	type file struct {
		path string
		t    time.Time
	}
	files := []file{}
	for _, en := range entries {
		if !en.IsDir() && len(en.Name()) == 69 && strings.HasSuffix(en.Name(), ".json") {
			in, e := en.Info()
			if e == nil {
				files = append(files, file{filepath.Join(c.CacheDir, en.Name()), in.ModTime()})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].t.Before(files[j].t) })
	for len(files) > maxCacheEntries {
		if e = os.Remove(files[0].path); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		files = files[1:]
	}
	return nil
}
func (c *Client) Catalogue(ctx context.Context, locale string, page int, query, area, cuisine string) (Page, Evidence, error) {
	pre, e := LocalePath(locale)
	if e != nil {
		return Page{}, Evidence{}, e
	}
	if page < 1 || page > 25 {
		return Page{}, Evidence{}, fmt.Errorf("page must be 1..25")
	}
	u := Origin + pre + "/r"
	if page > 1 {
		u += fmt.Sprintf("/page/%d", page)
	}
	q := url.Values{}
	if query != "" {
		q.Set("search_keywords", query)
	}
	if area != "" {
		q.Set("area", area)
	}
	if cuisine != "" {
		q.Set("cuisine", cuisine)
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var p Page
	ev, e := c.load(ctx, u, &p, func(b []byte) (any, error) { return ParsePage(b, locale) })
	return p, ev, e
}
func (c *Client) Restaurant(ctx context.Context, id, locale string) (Detail, error) {
	pre, e := LocalePath(locale)
	if e != nil {
		return Detail{}, e
	}
	if e = ValidateID(id); e != nil {
		return Detail{}, e
	}
	var d Detail
	ev, e := c.load(ctx, Origin+pre+"/r/"+id, &d, func(b []byte) (any, error) { return ParseDetail(b, id, locale) })
	if e != nil {
		return d, e
	}
	d.Evidence = []Evidence{ev}
	return d, nil
}
func (c *Client) Premium(ctx context.Context) (Membership, Evidence, error) {
	var m Membership
	ev, e := c.load(ctx, Origin+"/en/premium/green", &m, func(b []byte) (any, error) { return ParseMembership(b) })
	return m, ev, e
}

type Inventory struct {
	Version     int        `json:"version"`
	Locale      string     `json:"locale"`
	FetchedAt   time.Time  `json:"fetched_at"`
	Pages       int        `json:"pages"`
	SourceTotal *int       `json:"source_total"`
	Complete    bool       `json:"complete"`
	Results     []Summary  `json:"results"`
	Evidence    []Evidence `json:"evidence"`
}

func (c *Client) RefreshInventory(ctx context.Context, path, locale string, pages int) (Inventory, error) {
	if pages < 1 || pages > 25 {
		return Inventory{}, fmt.Errorf("pages must be 1..25")
	}
	if c.Offline {
		return Inventory{}, fmt.Errorf("inventory refresh needs online source access")
	}
	c.Refresh = true
	in := Inventory{Version: 1, Locale: locale, Results: []Summary{}, Evidence: []Evidence{}}
	seen := map[string]bool{}
	for page := 1; page <= pages; page++ {
		p, ev, e := c.Catalogue(ctx, locale, page, "", "", "")
		if e != nil {
			return in, e
		}
		in.Pages = page
		in.SourceTotal = p.Total
		in.Evidence = append(in.Evidence, ev)
		for _, r := range p.Results {
			if !seen[r.ID] {
				seen[r.ID] = true
				in.Results = append(in.Results, r)
			}
		}
		if p.Next == nil {
			in.Complete = true
			break
		}
	}
	in.FetchedAt = time.Now().UTC()
	b, e := json.Marshal(in)
	if e != nil {
		return in, e
	}
	if e = atomicWrite(path, b); e != nil {
		return in, e
	}
	return in, nil
}
func ReadInventory(path string) (Inventory, error) {
	b, e := readBounded(path, maxBody)
	if e != nil {
		return Inventory{}, e
	}
	var in Inventory
	if e = json.Unmarshal(b, &in); e != nil {
		return in, e
	}
	if in.Version != 1 || in.FetchedAt.IsZero() {
		return in, fmt.Errorf("invalid OMAKASE inventory snapshot")
	}
	return in, nil
}
