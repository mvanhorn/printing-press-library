// Package iana fetches the IANA RDAP bootstrap registry.
package iana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/cliutil"
)

// limiter paces outbound IANA requests. The bootstrap file is fetched at
// most a few times per day per CLI process, so the conservative 1 req/sec
// floor is plenty; the adaptive logic ramps up automatically on success
// and halves on 429.
var limiter = cliutil.NewAdaptiveLimiter(1.0)

// BootstrapURL is the IANA RDAP DNS bootstrap file.
const BootstrapURL = "https://data.iana.org/rdap/dns.json"

// BootstrapFile is the RFC 9224 bootstrap structure (subset used here).
type BootstrapFile struct {
	Version     string    `json:"version"`
	Publication string    `json:"publication"`
	Description string    `json:"description"`
	Services    [][][]any `json:"services"`
}

// TLDMap returns tld → first-RDAP-base-URL mapping.
func (b *BootstrapFile) TLDMap() map[string]string {
	out := map[string]string{}
	for _, entry := range b.Services {
		if len(entry) < 2 {
			continue
		}
		// entry[0] is []any of TLD strings; entry[1] is []any of RDAP base URLs.
		tlds := entry[0]
		urls := entry[1]
		if len(urls) == 0 {
			continue
		}
		first, _ := urls[0].(string)
		first = strings.TrimRight(first, "/")
		for _, t := range tlds {
			s, _ := t.(string)
			if s == "" {
				continue
			}
			out[strings.ToLower(s)] = first
		}
	}
	return out
}

// ErrRateLimited is returned when IANA responds with HTTP 429 (Too Many Requests).
// Callers can inspect RetryAfter to honor the Retry-After header.
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("iana: rate limited (HTTP 429), retry after %s", e.RetryAfter)
}

// Fetch downloads and parses the IANA bootstrap file. On HTTP 429 it returns
// a typed *ErrRateLimited so callers can back off appropriately. Outbound
// requests are paced via a shared AdaptiveLimiter.
func Fetch(ctx context.Context) (*BootstrapFile, error) {
	limiter.Wait()
	c := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BootstrapURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		return nil, &ErrRateLimited{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	limiter.OnSuccess()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("iana: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := &BootstrapFile{}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("decode iana: %w", err)
	}
	return out, nil
}

// parseRetryAfter parses a Retry-After header value (seconds or HTTP-date).
// Returns a sane default (30s) when the header is missing or unparseable.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 30 * time.Second
	}
	if secs, err := strconv.Atoi(h); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 30 * time.Second
}
