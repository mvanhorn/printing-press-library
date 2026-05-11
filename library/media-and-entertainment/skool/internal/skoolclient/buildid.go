// Package skoolclient holds Skool-specific runtime helpers that don't fit
// the factory-generated REST shape — chiefly the rotating buildId that
// Skool's Next.js data routes embed in every read URL.
package skoolclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// buildIDPattern matches "buildId":"<id>" in __NEXT_DATA__ JSON or any HTML
// payload that embeds it.
var buildIDPattern = regexp.MustCompile(`"buildId":"([^"]+)"`)

// Resolver fetches and caches Skool's current Next.js buildId. The buildId
// rotates on every Skool deploy (hours/days). Cache TTL is conservative.
type Resolver struct {
	HTTPClient *http.Client
	UserAgent  string
	AuthCookie string // raw "auth_token=..." cookie value, no "Cookie: " prefix
	CachePath  string // file path; empty = disable disk cache
	TTL        time.Duration

	mu     sync.Mutex
	cached string
	cachedAt time.Time
}

// NewResolver returns a Resolver with sane defaults.
func NewResolver(httpClient *http.Client, userAgent, authCookie, cachePath string) *Resolver {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	return &Resolver{
		HTTPClient: httpClient,
		UserAgent:  userAgent,
		AuthCookie: authCookie,
		CachePath:  cachePath,
		TTL:        4 * time.Hour,
	}
}

// Resolve returns a usable buildId, fetching and caching when needed.
// communitySlug is the slug of any community the user can reach (used as
// the source page); the returned buildId is global to Skool, not per-community.
func (r *Resolver) Resolve(ctx context.Context, communitySlug string) (string, error) {
	r.mu.Lock()
	if r.cached != "" && time.Since(r.cachedAt) < r.TTL {
		id := r.cached
		r.mu.Unlock()
		return id, nil
	}
	r.mu.Unlock()

	// Try disk cache first.
	if r.CachePath != "" {
		if id, ts, ok := r.readDiskCache(); ok && time.Since(ts) < r.TTL {
			r.mu.Lock()
			r.cached, r.cachedAt = id, ts
			r.mu.Unlock()
			return id, nil
		}
	}

	id, err := r.fetch(ctx, communitySlug)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	r.cached, r.cachedAt = id, time.Now()
	r.mu.Unlock()
	if r.CachePath != "" {
		_ = r.writeDiskCache(id)
	}
	return id, nil
}

// Invalidate clears the cached buildId (call on 404 from a data route).
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	r.cached = ""
	r.cachedAt = time.Time{}
	r.mu.Unlock()
	if r.CachePath != "" {
		_ = os.Remove(r.CachePath)
	}
}

func (r *Resolver) fetch(ctx context.Context, communitySlug string) (string, error) {
	url := "https://www.skool.com/"
	if communitySlug != "" {
		url = "https://www.skool.com/" + communitySlug
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", r.UserAgent)
	if r.AuthCookie != "" {
		// AuthCookie may already be "auth_token=<jwt>" or just the bare jwt.
		cookie := r.AuthCookie
		if !strings.Contains(cookie, "=") {
			cookie = "auth_token=" + cookie
		}
		req.Header.Set("Cookie", cookie)
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching skool homepage for buildId: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("buildId fetch returned HTTP %d (auth_token may be expired or User-Agent blocked)", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", fmt.Errorf("reading skool homepage: %w", err)
	}
	matches := buildIDPattern.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("buildId not found in skool homepage HTML — page format may have changed")
	}
	return string(matches[1]), nil
}

func (r *Resolver) readDiskCache() (string, time.Time, bool) {
	data, err := os.ReadFile(r.CachePath)
	if err != nil {
		return "", time.Time{}, false
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "|", 2)
	if len(parts) != 2 {
		return "", time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return "", time.Time{}, false
	}
	return parts[0], ts, true
}

func (r *Resolver) writeDiskCache(id string) error {
	if err := os.MkdirAll(filepath.Dir(r.CachePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.CachePath, []byte(id+"|"+time.Now().UTC().Format(time.RFC3339)), 0o600)
}
