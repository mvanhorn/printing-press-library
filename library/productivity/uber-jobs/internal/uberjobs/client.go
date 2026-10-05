// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
)

const (
	// DefaultBaseURL is the Uber careers site.
	DefaultBaseURL = "https://jobs.uber.com"
	// UserAgent is the fixed, honest identity sent to jobs.uber.com.
	UserAgent = "uber-jobs-pp-cli/0.1.0"
	// maxBody caps any response read (the full corpus is about 4 MB).
	maxBody = 64 << 20
	// defaultCacheTTL dedupes identical GETs inside one tracker paging loop.
	defaultCacheTTL = 2 * time.Minute
)

// RequestRecord describes one request that reached the network.
type RequestRecord struct {
	Time   time.Time
	Method string
	URL    string
	Status int
	Bytes  int
	Err    string
}

// Client is the paced, single-attempt HTTP client for the Uber careers site
// and its Oracle candidate-experience fallback. It never retries: a refusal,
// a transport failure, or a bad status ends the call with a typed error.
type Client struct {
	HTTP       *http.Client
	BaseURL    string
	OracleBase string
	Gate       *Gate
	Limiter    *cliutil.AdaptiveLimiter
	StateDir   string
	CacheDir   string
	CacheTTL   time.Duration
	NoCache    bool
	// RequestLog, when set, receives one TSV line per network request.
	RequestLog string
	// OnRequest is a test seam called after every network request.
	OnRequest func(RequestRecord)

	mu       sync.Mutex
	requests int
	refused  map[string]*RefusalError // per host: a refused host gets no further requests
}

// NewClient builds a client with the machine-wide gate and the default
// timeout. baseURL may be overridden for mock servers; Oracle fallback is
// disabled unless the Oracle base is also set when the base is overridden.
func NewClient(baseURL string, timeout time.Duration, stateDir, cacheDir string) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	oracle := OracleDefaultBase
	if baseURL != DefaultBaseURL {
		oracle = ""
	}
	if v := strings.TrimSpace(os.Getenv("UBER_JOBS_ORACLE_BASE_URL")); v != "" {
		oracle = strings.TrimRight(v, "/")
	}
	return &Client{
		HTTP: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				// A redirect is a separate request; never follow one silently.
				return http.ErrUseLastResponse
			},
		},
		BaseURL:    baseURL,
		OracleBase: oracle,
		Gate:       NewGate(stateDir),
		Limiter:    cliutil.NewAdaptiveLimiter(1.0 / MinRequestGap.Seconds()),
		StateDir:   stateDir,
		CacheDir:   cacheDir,
		CacheTTL:   defaultCacheTTL,
		RequestLog: strings.TrimSpace(os.Getenv("UBER_JOBS_REQUEST_LOG")),
	}
}

// Requests reports how many requests reached the network.
func (c *Client) Requests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

// Refused returns the first refusal seen by this client from any host.
func (c *Client) Refused() *RefusalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.refused {
		return r
	}
	return nil
}

// RefusedHost returns the refusal recorded for one host, if any.
func (c *Client) RefusedHost(host string) *RefusalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refused[host]
}

type response struct {
	Body     []byte
	Header   http.Header
	Status   int
	CacheHit bool
	URL      string
}

func (c *Client) get(ctx context.Context, rawURL, accept, ua string) (*response, error) {
	if hit := c.readCache(rawURL, accept); hit != nil {
		return hit, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", ua)
	return c.send(ctx, req)
}

// remember caches a GET reply. Callers invoke it only after the reply passed
// its positive content check, so a bad 200 page is never served again.
func (c *Client) remember(accept string, r *response) {
	if r != nil {
		c.writeCache(r.URL, accept, r)
	}
}

func (c *Client) postJSON(ctx context.Context, rawURL string, body any, ua string) (*response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	return c.send(ctx, req)
}

// send performs exactly one network attempt behind the gate and classifies
// the outcome. It refuses to send anything after this client saw a refusal.
func (c *Client) send(ctx context.Context, req *http.Request) (*response, error) {
	host := req.URL.Host
	if r := c.refusalFor(host); r != nil {
		return nil, r
	}
	var out *response
	err := c.Gate.Do(ctx, func() error {
		// Checked again under the gate lock: another process may have
		// latched the host while this one queued.
		if r := c.refusalFor(host); r != nil {
			return r
		}
		return nil
	}, func() error {
		var err error
		out, err = c.exchange(ctx, req)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// refusalFor returns the refusal this client saw from host, or the latch any
// process recorded for it (adopting it so later calls skip the file).
func (c *Client) refusalFor(host string) *RefusalError {
	if r := c.RefusedHost(host); r != nil {
		return r
	}
	r := LatchedRefusal(c.StateDir, host, time.Now())
	if r == nil {
		return nil
	}
	c.mu.Lock()
	if c.refused == nil {
		c.refused = map[string]*RefusalError{}
	}
	c.refused[host] = r
	c.mu.Unlock()
	return r
}

// exchange sends req once and reads the reply. It runs under the gate lock.
func (c *Client) exchange(ctx context.Context, req *http.Request) (*response, error) {
	if c.Limiter != nil {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	rawURL := req.URL.String()
	// Another process may have fetched this URL while this one waited at
	// the gate (concurrent commands all queue there); a fresh cached reply
	// then saves a request. The gate stamp stays, which only slows pacing.
	if req.Method == http.MethodGet {
		if hit := c.readCache(rawURL, req.Header.Get("Accept")); hit != nil {
			return hit, nil
		}
	}
	started := time.Now().UTC()
	resp, err := c.HTTP.Do(req)
	c.mu.Lock()
	c.requests++
	c.mu.Unlock()
	if err != nil {
		c.log(RequestRecord{Time: started, Method: req.Method, URL: rawURL, Err: oneLine(err.Error())})
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, classifyTransport(rawURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	c.log(RequestRecord{Time: started, Method: req.Method, URL: rawURL, Status: resp.StatusCode, Bytes: len(body)})
	// A refusal is classified before the read error: a 403 whose body was
	// cut short is still a refusal and must latch the host.
	if challenge := IsChallenge(resp.Header, body); challenge || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		r := newRefusal(rawURL, req.URL.Host, resp.StatusCode, challenge, truncate(string(body), 512))
		c.mu.Lock()
		if c.refused == nil {
			c.refused = map[string]*RefusalError{}
		}
		c.refused[req.URL.Host] = r
		c.mu.Unlock()
		if c.Limiter != nil {
			c.Limiter.OnRateLimit()
		}
		RecordRefusal(c.StateDir, r)
		return nil, r
	}
	if err != nil {
		return nil, classifyTransport(rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{URL: rawURL, Status: resp.StatusCode, Body: truncate(string(body), 512)}
	}
	if c.Limiter != nil {
		c.Limiter.OnSuccess()
	}
	return &response{Body: body, Header: resp.Header, Status: resp.StatusCode, URL: rawURL}, nil
}

// IsChallenge recognizes bot-protection interstitials by anchored markers,
// never by loose substrings of ordinary content.
func IsChallenge(h http.Header, body []byte) bool {
	if strings.TrimSpace(h.Get("Cf-Mitigated")) != "" {
		return true
	}
	head := body
	if len(head) > 16384 {
		head = head[:16384]
	}
	lower := strings.ToLower(string(head))
	for _, marker := range []string{"<title>just a moment", "cf-error-details", "_cf_chl_opt", "<title>attention required! | cloudflare"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (c *Client) log(rec RequestRecord) {
	if c.OnRequest != nil {
		c.OnRequest(rec)
	}
	if c.RequestLog == "" {
		return
	}
	f, err := os.OpenFile(c.RequestLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	status := fmt.Sprint(rec.Status)
	if rec.Err != "" {
		status = "ERR:" + rec.Err
	}
	fmt.Fprintf(f, "%s\tuber-jobs-pp-cli\t%s\t%s\t%s\t%d\n", rec.Time.Format("2006-01-02T15:04:05Z"), rec.Method, rec.URL, status, rec.Bytes)
}

type cacheEntry struct {
	Stored int64       `json:"stored"`
	URL    string      `json:"url"`
	Header http.Header `json:"header"`
	Body   []byte      `json:"body"`
}

func (c *Client) cachePath(rawURL, accept string) string {
	sum := sha256.Sum256([]byte(accept + "\n" + rawURL))
	return filepath.Join(c.CacheDir, "uberjobs", hex.EncodeToString(sum[:12])+".json")
}

func (c *Client) readCache(rawURL, accept string) *response {
	if c.NoCache || c.CacheDir == "" || c.CacheTTL <= 0 {
		return nil
	}
	raw, err := os.ReadFile(c.cachePath(rawURL, accept))
	if err != nil {
		return nil
	}
	var e cacheEntry
	if json.Unmarshal(raw, &e) != nil || e.URL != rawURL {
		return nil
	}
	// A stamp ahead of the clock (the clock moved back) is a miss, never
	// an entry that stays fresh until the clock catches up.
	if age := time.Since(time.Unix(0, e.Stored)); age < 0 || age > c.CacheTTL {
		return nil
	}
	return &response{Body: e.Body, Header: e.Header, Status: http.StatusOK, CacheHit: true, URL: rawURL}
}

func (c *Client) writeCache(rawURL, accept string, r *response) {
	if c.NoCache || c.CacheDir == "" || c.CacheTTL <= 0 || r == nil || r.CacheHit {
		return
	}
	p := c.cachePath(rawURL, accept)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	raw, err := json.Marshal(cacheEntry{Stored: time.Now().UnixNano(), URL: rawURL, Header: r.Header, Body: r.Body})
	if err != nil {
		return
	}
	_ = os.WriteFile(p, raw, 0o600)
	c.pruneCache(filepath.Dir(p), p)
}

// pruneCache removes expired entries, so keys that change with the corpus
// size (pagesize=total+50) never pile up. Entries stamped well ahead of the
// clock go too: readCache never serves them.
func (c *Client) pruneCache(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if p == keep {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if age := now.Sub(info.ModTime()); age > c.CacheTTL || age < -c.CacheTTL {
			_ = os.Remove(p)
		}
	}
}

func joinURL(base, path string, q url.Values) string {
	u := strings.TrimRight(base, "/") + path
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
