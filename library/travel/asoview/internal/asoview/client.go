// Package asoview implements bounded anonymous first-party GETs and source parsing.
package asoview

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const Origin = "https://www.asoview.com"
const maxBody = 4 << 20
const cacheMaxBytes = 32 << 20
const cacheMaxEntries = 128

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }
func fail(code int, format string, args ...any) error {
	return &Error{code, fmt.Sprintf(format, args...)}
}

type Source struct {
	URL        string `json:"url"`
	FetchedAt  string `json:"fetched_at"`
	Cached     bool   `json:"cached"`
	TTLSeconds int64  `json:"ttl_seconds"`
}
type Stats struct {
	Requests           int   `json:"network_requests"`
	CacheHits          int   `json:"cache_hits"`
	CacheWriteFailures int   `json:"cache_write_failures"`
	DownloadedBytes    int64 `json:"downloaded_bytes"`
	ElapsedMS          int64 `json:"elapsed_ms"`
}
type Client struct {
	HTTP        *http.Client
	CacheDir    string
	NoCache     bool
	Refresh     bool
	Offline     bool
	Sources     []Source
	Stats       Stats
	started     time.Time
	lastRequest time.Time
}
type cacheEntry struct {
	URL  string    `json:"url"`
	At   time.Time `json:"at"`
	Body []byte    `json:"body"`
}

func NewClient(cacheDir string, timeout time.Duration, noCache, refresh, offline bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 2
	tr.ResponseHeaderTimeout = timeout
	c := &Client{HTTP: &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fail(5, "redirect limit exceeded")
		}
		return validateURL(req.URL)
	}}, CacheDir: cacheDir, NoCache: noCache, Refresh: refresh, Offline: offline, started: time.Now(), Sources: []Source{}}
	c.HTTP.Transport = ReadOnlyTransport{Base: tr, BeforeRequest: c.recordAttempt}
	return c
}

func validateURL(u *url.URL) error {
	if u.Scheme != "https" || u.Host != "www.asoview.com" || u.User != nil || u.Fragment != "" {
		return fail(2, "only anonymous HTTPS www.asoview.com source URLs are supported")
	}
	p := u.Path
	allowed := p == "/search/" || p == "/location/" || p == "/leisure/" || p == "/item/category-sales-situations/" || p == "/stocks/calendars" || p == "/stocks/courses" || p == "/stocks/ticket/calendars" || p == "/stocks/ticket/courses"
	if strings.HasPrefix(p, "/item/ticket/") || strings.HasPrefix(p, "/item/activity/") {
		parts := strings.Split(strings.Trim(p, "/"), "/")
		if len(parts) == 3 {
			kind, _, err := ParseID(parts[2])
			allowed = err == nil && parts[1] == kind
		}
	}
	if strings.HasPrefix(p, "/reservations/plans/") {
		parts := strings.Split(strings.Trim(p, "/"), "/")
		if len(parts) == 6 && parts[3] == "dates" && parts[5] == "basicfees" {
			kind, _, err := ParseID(parts[2])
			_, de := ParseDate(parts[4])
			allowed = err == nil && de == nil && kind == "activity"
		}
	}
	if !allowed {
		return fail(2, "source path %q is outside the read-only allowlist", p)
	}
	return nil
}

// ReadOnlyTransport also guards generator-owned raw source commands.
type ReadOnlyTransport struct {
	Base          http.RoundTripper
	BeforeRequest func(context.Context) error
}

func (t ReadOnlyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil, fail(2, "Asoview supports read-only GET/HEAD only")
	}
	if err := validateURL(r.URL); err != nil {
		return nil, err
	}
	r = r.Clone(r.Context())
	r.Header.Del("Cookie")
	r.Header.Del("Authorization")
	if t.BeforeRequest != nil {
		if err := t.BeforeRequest(r.Context()); err != nil {
			return nil, err
		}
	}
	resp, err := t.Base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		original := resp.Body
		decoded, err := decodeHTTPBody(&boundedBody{ReadCloser: original, remaining: maxBody}, resp.Header.Get("Content-Encoding"))
		if err != nil {
			_ = original.Close()
			return nil, err
		}
		resp.Body = &boundedBody{ReadCloser: decoded, remaining: maxBody}
		if resp.Header.Get("Content-Encoding") != "" {
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			resp.Uncompressed = true
		}
	}
	return resp, nil
}

func (c *Client) Get(ctx context.Context, path string, q url.Values, ttl time.Duration) ([]byte, error) {
	u, err := url.Parse(Origin + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = q.Encode()
	if err = validateURL(u); err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(u.String()))
	cacheFile := filepath.Join(c.CacheDir, hex.EncodeToString(key[:])+".json")
	if !c.NoCache && !c.Refresh {
		raw, e := readCacheFile(c.CacheDir, filepath.Base(cacheFile), maxBody*2)
		var entry cacheEntry
		if e == nil && json.Unmarshal(raw, &entry) == nil && entry.URL == u.String() && len(entry.Body) <= maxBody && time.Since(entry.At) >= 0 && time.Since(entry.At) < ttl {
			c.Stats.CacheHits++
			c.Sources = append(c.Sources, Source{u.String(), entry.At.UTC().Format(time.RFC3339), true, int64(ttl.Seconds())})
			return entry.Body, nil
		}
	}
	if c.Offline {
		return nil, fail(5, "no fresh cached response for %s; run without --data-source local or refresh online", path)
	}
	var body []byte
	for attempt := 0; attempt < 2; attempt++ {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "asoview-pp-cli/0.1 (read-only leisure discovery)")
		req.Header.Set("Accept", "application/json,text/html;q=0.9")
		resp, e := c.HTTP.Do(req)
		if e != nil {
			return nil, fail(5, "Asoview GET %s failed: %v", path, e)
		}
		body, e = io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		closeErr := resp.Body.Close()
		if e != nil {
			return nil, fail(5, "reading Asoview response: %v", e)
		}
		if closeErr != nil {
			return nil, fail(5, "closing Asoview response: %v", closeErr)
		}
		c.Stats.DownloadedBytes += int64(len(body))
		if len(body) > maxBody {
			return nil, fail(5, "Asoview response exceeds 4 MiB limit")
		}
		if resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			if attempt == 0 {
				delay := time.Second
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if seconds, e := time.ParseDuration(ra + "s"); e == nil && seconds >= 0 {
						delay = seconds
					} else if dt, e := http.ParseTime(ra); e == nil {
						delay = time.Until(dt)
					}
				}
				if delay < 0 {
					delay = 0
				}
				if delay > 5*time.Second {
					if resp.StatusCode == 429 {
						return nil, fail(7, "Asoview rate limited; Retry-After exceeds bounded retry budget; retry later")
					}
					return nil, fail(5, "Asoview unavailable; retry later")
				}
				if err = waitContext(ctx, delay); err != nil {
					return nil, fail(5, "retry deadline: %v", err)
				}
				continue
			}
			if resp.StatusCode == 429 {
				return nil, fail(7, "Asoview rate limited after one retry; retry later")
			}
		}
		if resp.StatusCode == 404 {
			return nil, fail(3, "Asoview source not found: %s", path)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fail(4, "Asoview denied public access (HTTP %d); no credentials or challenge bypass is attempted", resp.StatusCode)
		}
		if resp.StatusCode != 200 {
			return nil, fail(5, "Asoview GET %s returned HTTP %d", path, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("Content-Type"), "html") && !strings.Contains(resp.Header.Get("Content-Type"), "json") {
			return nil, fail(5, "unexpected Asoview content type %q", resp.Header.Get("Content-Type"))
		}
		break
	}
	at := time.Now().UTC()
	c.Sources = append(c.Sources, Source{u.String(), at.Format(time.RFC3339), false, int64(ttl.Seconds())})
	if !c.NoCache && c.CacheDir != "" {
		if e := c.saveCache(cacheFile, cacheEntry{u.String(), at, body}); e != nil {
			// Persistence is optional for a successful live read. Keep provenance
			// and expose the failure count without leaking local paths.
			c.Stats.CacheWriteFailures++
		}
	}
	return body, nil
}
func waitContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) Metrics() Stats {
	s := c.Stats
	s.ElapsedMS = time.Since(c.started).Milliseconds()
	return s
}
func (c *Client) saveCache(path string, entry cacheEntry) error {
	if err := os.MkdirAll(c.CacheDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(c.CacheDir, ".asoview-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return pruneCache(c.CacheDir)
}
func pruneCache(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type file struct {
		name string
		size int64
		at   time.Time
	}
	files := []file{}
	var size int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() != "taxonomy-regions.json" && e.Name() != "taxonomy-categories.json" {
			if len(e.Name()) != 69 || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if _, err = hex.DecodeString(e.Name()[:64]); err != nil {
				continue
			}
		}
		info, e2 := e.Info()
		if e2 != nil {
			continue
		}
		files = append(files, file{e.Name(), info.Size(), info.ModTime()})
		size += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for len(files) > cacheMaxEntries || size > cacheMaxBytes {
		f := files[0]
		if err = os.Remove(filepath.Join(dir, f.name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		size -= f.size
		files = files[1:]
	}
	return nil
}

// recordAttempt runs inside the transport, so redirects and retries share one wire budget.
func (c *Client) recordAttempt(ctx context.Context) error {
	if c.Stats.Requests >= 20 {
		return fail(5, "request budget exceeded (20 wire attempts per command)")
	}
	if wait := 300*time.Millisecond - time.Since(c.lastRequest); wait > 0 {
		if err := waitContext(ctx, wait); err != nil {
			return fail(5, "request deadline: %v", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(5, "request deadline: %v", err)
	}
	c.lastRequest = time.Now()
	c.Stats.Requests++
	return nil
}

type boundedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *boundedBody) Read(p []byte) (int, error) {
	if int64(len(p)) > b.remaining+1 {
		p = p[:b.remaining+1]
	}
	n, err := b.ReadCloser.Read(p)
	if int64(n) > b.remaining {
		return 0, fail(5, "Asoview response exceeds 4 MiB limit")
	}
	b.remaining -= int64(n)
	return n, err
}

// Cache filenames are rooted and read with a cap even if another process changes a file.
func readCacheFile(dir, name string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fail(10, "cache file exceeds bounded size")
	}
	return raw, nil
}

type decodedHTTPBody struct {
	reader   io.Reader
	original io.Closer
	decoders []io.Closer
}

func (b *decodedHTTPBody) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *decodedHTTPBody) Close() error {
	var first error
	for i := len(b.decoders) - 1; i >= 0; i-- {
		if err := b.decoders[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	if err := b.original.Close(); err != nil && first == nil {
		first = err
	}
	return first
}
func decodeHTTPBody(body io.ReadCloser, encoding string) (io.ReadCloser, error) {
	if strings.TrimSpace(encoding) == "" {
		return body, nil
	}
	encodings := strings.Split(strings.ToLower(encoding), ",")
	if len(encodings) > 3 {
		return nil, fail(5, "source content-encoding stack exceeds three layers")
	}
	out := &decodedHTTPBody{reader: body, original: body}
	for i := len(encodings) - 1; i >= 0; i-- {
		enc := strings.TrimSpace(encodings[i])
		var reader io.ReadCloser
		var err error
		switch enc {
		case "identity", "":
			continue
		case "gzip":
			reader, err = gzip.NewReader(out.reader)
		case "deflate":
			b := bufio.NewReader(out.reader)
			head, e := b.Peek(2)
			if e != nil {
				return nil, fail(5, "invalid source deflate header")
			}
			if head[0]&15 == 8 && (int(head[0])*256+int(head[1]))%31 == 0 {
				reader, err = zlib.NewReader(b)
			} else {
				reader = flate.NewReader(b)
			}
		default:
			return nil, fail(5, "unsupported source content-encoding %q", enc)
		}
		if err != nil {
			return nil, fail(5, "invalid source content-encoding %q: %v", enc, err)
		}
		out.reader = reader
		out.decoders = append(out.decoders, reader)
	}
	return out, nil
}
