// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package imagecache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SupabaseStoragePrefix is the path prefix Mobbin's Supabase storage URLs share.
// The bytes AFTER this prefix (e.g. "content/app_screens/<uuid>.png") are appended
// verbatim to BytescaleBase. Critical: keep `content/` IN the suffix, not in the
// prefix — the live Bytescale CDN expects `.../prod/content/app_screens/...`.
const SupabaseStoragePrefix = "/storage/v1/object/public/"
const BytescaleBase = "https://bytescale.mobbin.com/FW25bBB/image/mobbin.com/prod"

// httpClient bounds each image download so a stalled CDN connection can't hang
// deck/grab/app indefinitely (the CLI root --timeout does not reach this
// sibling package). A per-request ceiling is the floor even when the caller's
// context carries no deadline.
var httpClient = &http.Client{Timeout: 90 * time.Second}

type Cache struct{ Root string }
type CDNOpts struct {
	Width   int
	Quality int
	Format  string
}
type FetchItem struct{ ImageURL, Platform, AppSlug, ScreenID string }

// RateLimitError is returned when the Bytescale/Mobbin image CDN responds 429.
// Callers (e.g. concurrent FetchMany) can use errors.As to detect throttling
// and back off rather than swallowing the failure as a missing image.
type RateLimitError struct {
	URL        string
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter != "" {
		return fmt.Sprintf("image CDN rate-limited (HTTP 429) for %s; retry after %s", e.URL, e.RetryAfter)
	}
	return fmt.Sprintf("image CDN rate-limited (HTTP 429) for %s", e.URL)
}

func New(rootOverride string) (*Cache, error) {
	root := rootOverride
	if root == "" {
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			root = filepath.Join(xdg, "mobbin-pp-cli", "images")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			root = filepath.Join(home, ".cache", "mobbin-pp-cli", "images")
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Cache{Root: root}, nil
}

func (c *Cache) Path(platform, appSlug, screenID, format string) string {
	if format == "" {
		format = "webp"
	}
	format = strings.TrimPrefix(format, ".")
	return filepath.Join(c.Root, clean(platform), clean(appSlug), clean(screenID)+"."+clean(format))
}

func (c *Cache) Has(platform, appSlug, screenID, format string) bool {
	st, err := os.Stat(c.Path(platform, appSlug, screenID, format))
	return err == nil && !st.IsDir()
}

func ToCDNURL(imageURL string, opts CDNOpts) string {
	if imageURL == "" {
		return imageURL
	}
	if strings.HasPrefix(imageURL, "https://bytescale.mobbin.com/") {
		return withCDNQuery(imageURL, opts)
	}
	idx := strings.Index(imageURL, SupabaseStoragePrefix)
	if idx < 0 {
		return imageURL
	}
	p := imageURL[idx+len(SupabaseStoragePrefix):]
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	return withCDNQuery(strings.TrimRight(BytescaleBase, "/")+"/"+strings.TrimLeft(p, "/"), opts)
}

func (c *Cache) Fetch(ctx context.Context, imageURL, platform, appSlug, screenID string, opts CDNOpts) (string, bool, error) {
	opts = opts.defaults()
	path := c.Path(platform, appSlug, screenID, opts.Format)
	if c.Has(platform, appSlug, screenID, opts.Format) {
		return path, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ToCDNURL(imageURL, opts), nil)
	if err != nil {
		return "", false, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", false, &RateLimitError{URL: imageURL, RetryAfter: resp.Header.Get("Retry-After")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("fetching image %s returned HTTP %d", imageURL, resp.StatusCode)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", false, err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", false, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", false, closeErr
	}
	return path, false, os.Rename(tmp, path)
}

func (c *Cache) FetchMany(ctx context.Context, items []FetchItem, opts CDNOpts, concurrency int) (map[string]string, []error) {
	if concurrency <= 0 || concurrency > 8 {
		concurrency = 8
	}
	sem := make(chan struct{}, concurrency)
	paths := map[string]string{}
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, it := range items {
		it := it
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p, _, err := c.Fetch(ctx, it.ImageURL, it.Platform, it.AppSlug, it.ScreenID, opts)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			paths[it.ScreenID] = p
		}()
	}
	wg.Wait()
	return paths, errs
}

func (c *Cache) Stats() (int64, int, error) {
	var bytes int64
	var files int
	err := filepath.WalkDir(c.Root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		bytes += st.Size()
		files++
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	return bytes, files, err
}

func (c *Cache) Prune(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	deleted := 0
	err := filepath.WalkDir(c.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		if st.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return deleted, err
}

func (o CDNOpts) defaults() CDNOpts {
	if o.Width == 0 {
		o.Width = 1920
	}
	if o.Quality == 0 {
		o.Quality = 85
	}
	if o.Format == "" {
		o.Format = "webp"
	}
	return o
}
func withCDNQuery(raw string, opts CDNOpts) string {
	opts = opts.defaults()
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("f", opts.Format)
	q.Set("w", fmt.Sprint(opts.Width))
	q.Set("q", fmt.Sprint(opts.Quality))
	q.Set("fit", "shrink-cover")
	u.RawQuery = q.Encode()
	return u.String()
}
func clean(s string) string {
	s = strings.TrimSpace(s)
	r := strings.NewReplacer("/", "-", "\\", "-", "..", "-")
	if s == "" {
		return "unknown"
	}
	return r.Replace(s)
}
