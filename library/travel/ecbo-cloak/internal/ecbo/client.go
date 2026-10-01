// Package ecbo implements bounded read-only first-party ecbo cloak transport.
package ecbo

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
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/ecbo-cloak/internal/cliutil"
)

const maxBody = 2 << 20
const maxCacheFiles = 256

type Error struct {
	Code          int
	Kind, Message string
}

func (e *Error) Error() string { return e.Kind + ": " + e.Message }

type Observation struct {
	ObservedAt string `json:"observed_at"`
	CacheHit   bool   `json:"cache_hit"`
	SourceURL  string `json:"source_url"`
}
type Meta struct {
	Provider        string        `json:"provider"`
	Requests        int           `json:"requests"`
	CacheHits       int           `json:"cache_hits"`
	ElapsedMS       int64         `json:"elapsed_ms"`
	PartialCoverage bool          `json:"partial_coverage"`
	Coverage        string        `json:"coverage"`
	Observations    []Observation `json:"observations"`
}
type Client struct {
	HTTP             *http.Client
	CacheDir, Locale string
	Refresh, NoCache bool
	limiter          *cliutil.AdaptiveLimiter
	Meta             Meta
	started          time.Time
}
type cacheEntry struct {
	At   time.Time       `json:"at"`
	Data json.RawMessage `json:"data"`
}

func New(cacheDir, locale string, timeout time.Duration, refresh, noCache bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 2
	tr.MaxConnsPerHost = 1
	tr.ResponseHeaderTimeout = timeout
	c := &Client{CacheDir: cacheDir, Locale: locale, Refresh: refresh, NoCache: noCache, started: time.Now(), limiter: cliutil.NewAdaptiveLimiter(3), Meta: Meta{Provider: "ecbo cloak", PartialCoverage: true, Observations: []Observation{}}}
	c.HTTP = &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("redirect limit reached")
		}
		if r.URL.Scheme != "https" || r.URL.Host != "cloak.ecbo.io" {
			return fmt.Errorf("refusing cross-provider redirect")
		}
		return nil
	}}
	return c
}
func (c *Client) Finish() Meta {
	c.Meta.ElapsedMS = time.Since(c.started).Milliseconds()
	return c.Meta
}
func (c *Client) Request(ctx context.Context, method, target string, body any, ttl time.Duration) (map[string]any, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || !allowed(u.Host, u.Path, method) {
		return nil, &Error{2, "request", "unsupported provider operation"}
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256([]byte(method + target + c.Locale + string(payload)))
	path := filepath.Join(c.CacheDir, "ecbo-response-"+hex.EncodeToString(sum[:])+".json")
	if ttl > 0 && !c.NoCache && !c.Refresh {
		if b, e := readLimited(path); e == nil && len(b) <= maxBody+1024 {
			var x cacheEntry
			if json.Unmarshal(b, &x) == nil && time.Since(x.At) >= 0 && time.Since(x.At) < ttl {
				var out map[string]any
				if json.Unmarshal(x.Data, &out) == nil && out != nil {
					c.Meta.CacheHits++
					c.Meta.Observations = append(c.Meta.Observations, Observation{x.At.UTC().Format(time.RFC3339), true, target})
					return out, nil
				}
			}
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = c.limiter.Wait(ctx); err != nil {
			return nil, &Error{5, "timeout", err.Error()}
		}
		req, e := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
		if e != nil {
			return nil, e
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("LOCALE", c.Locale)
		req.Header.Set("Accept-Language", map[string]string{"ja": "ja-JP", "en": "en-US", "zh-TW": "zh-TW", "zh-CN": "zh-CN"}[c.Locale])
		req.Header.Set("User-Agent", "ecbo-cloak-cli/0.1 read-only")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		c.Meta.Requests++
		resp, e := c.HTTP.Do(req)
		if e != nil {
			return nil, &Error{5, "transport", e.Error()}
		}
		b, e := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		closeErr := resp.Body.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return nil, &Error{5, "transport", e.Error()}
		}
		if len(b) > maxBody {
			return nil, &Error{5, "response_size", "source response exceeded 2 MiB"}
		}
		if resp.StatusCode == 429 {
			if attempt == 0 {
				select {
				case <-time.After(time.Second):
					continue
				case <-ctx.Done():
					return nil, &Error{5, "timeout", ctx.Err().Error()}
				}
			}
			return nil, &cliutil.RateLimitError{URL: target}
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			select {
			case <-time.After(300 * time.Millisecond):
				continue
			case <-ctx.Done():
				return nil, &Error{5, "timeout", ctx.Err().Error()}
			}
		}
		var out map[string]any
		if json.Unmarshal(b, &out) != nil || out == nil {
			return nil, &Error{5, "source_shape", fmt.Sprintf("HTTP %d response was not a JSON object", resp.StatusCode)}
		}
		if resp.StatusCode == 404 {
			return nil, &Error{3, "not_found", "facility does not exist or is unavailable"}
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, &Error{4, "access", "public operation denied; no account credentials used"}
		}
		if resp.StatusCode == 422 && u.Path == "/api/web/reservations/validate" {
			if !validationErrors(out["errors"]) {
				return nil, &Error{5, "source_shape", "validation rejection lacks recognized structured domain errors"}
			}
			out["valid"] = false
			out["http_status"] = resp.StatusCode
			now := time.Now().UTC()
			c.Meta.Observations = append(c.Meta.Observations, Observation{now.Format(time.RFC3339), false, target})
			return out, nil
		}
		if resp.StatusCode >= 400 {
			return nil, &Error{5, "upstream", fmt.Sprintf("HTTP %d: %s", resp.StatusCode, boundedErrors(out))}
		}
		now := time.Now().UTC()
		c.Meta.Observations = append(c.Meta.Observations, Observation{now.Format(time.RFC3339), false, target})
		if ttl > 0 && !c.NoCache {
			entry, _ := json.Marshal(cacheEntry{now, b})
			if err := writeCache(path, entry); err != nil {
				return nil, &Error{10, "cache", err.Error()}
			}
		}
		return out, nil
	}
	return nil, &Error{5, "upstream", "retry budget exhausted"}
}
func allowed(host, path, method string) bool {
	switch host {
	case "search.ecbo.io":
		return method == "GET" && path == "/api/v1/spaces"
	case "api.ecbo.io":
		return method == "GET" && strings.HasPrefix(path, "/api/web/spaces/") || (method == "POST" && (path == "/api/web/reservations/price" || path == "/api/web/reservations/validate"))
	}
	return false
}
func boundedErrors(out map[string]any) string {
	b, _ := json.Marshal(out["errors"])
	if len(b) > 2000 {
		b = b[:2000]
	}
	return string(b)
}
func writeCache(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	owned := []os.DirEntry{}
	for _, x := range entries {
		if !x.IsDir() && managedCacheName(x.Name()) {
			owned = append(owned, x)
		}
	}
	if len(owned) >= maxCacheFiles && managedCacheName(filepath.Base(path)) {
		if _, existsErr := os.Stat(path); os.IsNotExist(existsErr) {
			var oldest string
			var t time.Time
			for _, x := range owned {
				i, e := x.Info()
				if e != nil {
					return e
				}
				if oldest == "" || i.ModTime().Before(t) {
					oldest = filepath.Join(dir, x.Name())
					t = i.ModTime()
				}
			}
			if oldest != "" {
				if e := os.Remove(oldest); e != nil {
					return e
				}
			}
		}
	}
	f, e := os.CreateTemp(dir, ".ecbo-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Close()
	} else {
		_ = f.Close()
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

func readLimited(path string) ([]byte, error) {
	// #nosec G304 -- Cache reads use a hash or fixed inventory filename under an explicitly selected cache directory, with a hard size bound.
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxBody+1025))
	if len(b) > maxBody+1024 {
		return nil, fmt.Errorf("cached file exceeds size limit")
	}
	return b, e
}

func managedCacheName(name string) bool {
	const prefix = "ecbo-response-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
		return false
	}
	hexName := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")
	if len(hexName) != 64 {
		return false
	}
	_, e := hex.DecodeString(hexName)
	return e == nil
}
func validationErrors(v any) bool {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return false
	}
	for k, x := range m {
		switch k {
		case "from", "to", "reservation_items", "space_id", "base":
		default:
			return false
		}
		items, ok := x.([]any)
		if !ok || len(items) == 0 {
			return false
		}
		for _, item := range items {
			if text, ok := item.(string); !ok || strings.TrimSpace(text) == "" {
				return false
			}
		}
	}
	return true
}
