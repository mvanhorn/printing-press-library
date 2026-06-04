// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package trivago

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
)

const (
	frankfurterEndpoint = "https://api.frankfurter.app/latest"
	fxCacheTTL          = 24 * time.Hour
	// fxRatePerSec paces outbound Frankfurter calls. Network calls are
	// rare (gated by the 24h cache) but we still apply the canonical
	// AdaptiveLimiter so 429s back off instead of cascading.
	fxRatePerSec = 1.0
)

// fxClient is a small in-process FX rate cache backed by Frankfurter
// (ECB daily rates, free, no key). One rate map per base currency,
// cached in memory for the process and on disk for 24h so repeated
// hotel searches don't dial out per result.
type fxClient struct {
	http     *http.Client
	cacheDir string
	limiter  *cliutil.AdaptiveLimiter

	mu    sync.Mutex
	rates map[string]frankfurterResponse
}

var defaultFX = newFXClient()

func newFXClient() *fxClient {
	homeDir, _ := os.UserHomeDir()
	return &fxClient{
		http:     &http.Client{Timeout: 10 * time.Second},
		cacheDir: filepath.Join(homeDir, ".cache", "hotel-goat-pp-cli", "fx"),
		limiter:  cliutil.NewAdaptiveLimiter(fxRatePerSec),
		rates:    map[string]frankfurterResponse{},
	}
}

type frankfurterResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`

	fetchedAt time.Time
}

// Convert returns amount converted from `from` to `to`, along with the
// exchange rate applied. ok=false means the conversion couldn't be
// resolved (network failure, unknown currency) — caller should fall
// back to displaying the native amount.
func Convert(ctx context.Context, amount float64, from, to string) (converted, rate float64, ok bool) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if from == "" || to == "" || amount == 0 {
		return 0, 0, false
	}
	if from == to {
		return amount, 1.0, true
	}
	r, err := defaultFX.rate(ctx, from, to)
	if err != nil || r == 0 {
		return 0, 0, false
	}
	return amount * r, r, true
}

func (c *fxClient) rate(ctx context.Context, from, to string) (float64, error) {
	c.mu.Lock()
	cached, ok := c.rates[from]
	c.mu.Unlock()
	if ok && time.Since(cached.fetchedAt) < fxCacheTTL {
		if r, found := cached.Rates[to]; found {
			return r, nil
		}
	}
	if disk, ok := c.readDisk(from); ok {
		c.mu.Lock()
		c.rates[from] = disk
		c.mu.Unlock()
		if r, found := disk.Rates[to]; found {
			return r, nil
		}
	}
	fresh, err := c.fetch(ctx, from)
	if err != nil {
		return 0, err
	}
	fresh.fetchedAt = time.Now()
	c.mu.Lock()
	c.rates[from] = fresh
	c.mu.Unlock()
	c.writeDisk(from, fresh)
	r, found := fresh.Rates[to]
	if !found {
		return 0, fmt.Errorf("fx: no rate for %s in %s base", to, from)
	}
	return r, nil
}

// fetch hits Frankfurter for the latest rate map keyed on `from`.
// Calls are naturally throttled by the 24h disk + in-memory cache
// (each base currency is fetched at most once per day per process);
// we still treat 429 as a typed retryable error so callers can
// fall back to the cached/native amount instead of erroring out.
func (c *fxClient) fetch(ctx context.Context, from string) (frankfurterResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", frankfurterEndpoint+"?from="+from, nil)
	if err != nil {
		return frankfurterResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	// Throttle via AdaptiveLimiter, but honor ctx cancellation. A
	// Frankfurter 429 halves the rate on the process-wide singleton, so
	// without this select a single backoff makes every subsequent
	// Convert() call inside Merge() sleep up to ~2s ignoring the
	// caller's cancellation. Matches client.waitForSlot. Buffered
	// channel so the abandoned goroutine never blocks on send.
	done := make(chan struct{}, 1)
	go func() {
		c.limiter.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return frankfurterResponse{}, ctx.Err()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return frankfurterResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return frankfurterResponse{}, &cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return frankfurterResponse{}, fmt.Errorf("fx: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return frankfurterResponse{}, err
	}
	var out frankfurterResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return frankfurterResponse{}, err
	}
	c.limiter.OnSuccess()
	return out, nil
}

func (c *fxClient) cachePath(from string) string {
	return filepath.Join(c.cacheDir, strings.ToUpper(from)+".json")
}

func (c *fxClient) readDisk(from string) (frankfurterResponse, bool) {
	path := c.cachePath(from)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > fxCacheTTL {
		return frankfurterResponse{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return frankfurterResponse{}, false
	}
	var out frankfurterResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return frankfurterResponse{}, false
	}
	out.fetchedAt = info.ModTime()
	return out, true
}

func (c *fxClient) writeDisk(from string, r frankfurterResponse) {
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.cachePath(from), data, 0o644)
}
