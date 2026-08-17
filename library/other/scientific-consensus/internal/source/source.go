// Package source implements the secondary data-source layer for
// scientific-consensus (the aggregator pattern). OpenAlex is the generated
// primary client; these are hand-authored enrichment/coverage sources
// (PubMed, Crossref, Europe PMC, Semantic Scholar) with a shared registry and
// per-source adaptive rate limiting. No generated code lives here.
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
)

// Work is the lossy cross-source record returned by a source Sync.
type Work struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Year     int    `json:"year,omitempty"`
	DOI      string `json:"doi,omitempty"`
	URL      string `json:"url,omitempty"`
}

// SyncOptions controls a source Sync.
type SyncOptions struct {
	Query string
	Limit int
}

// Source is a secondary scholarly data source.
type Source interface {
	Name() string
	Description() string
	AuthRequired() bool     // false = keyless; true = needs a key for full use
	OptionalKeyEnv() string // env var that raises limits / unlocks, "" if none
	Sync(context.Context, SyncOptions) ([]Work, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Source{}
)

// Register adds a source to the registry (called from init in each source file).
func Register(s Source) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[s.Name()] = s
}

// All returns the registered sources sorted by name.
func All() []Source {
	registryMu.RLock()
	out := make([]Source, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	registryMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Lookup finds a source by name.
func Lookup(name string) (Source, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[name]
	return s, ok
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

const userAgent = "scientific-consensus-pp-cli (https://github.com/mvanhorn/printing-press-library; mailto:scientific-consensus-pp-cli@users.noreply.github.com)"

// getJSON performs a paced GET and decodes JSON into v. On HTTP 429 it records
// the throttle with the limiter and returns *cliutil.RateLimitError so callers
// never confuse throttling with "no data". // pp:client-call
func getJSON(ctx context.Context, limiter *cliutil.AdaptiveLimiter, url string, v any) error {
	limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &cliutil.RateLimitError{URL: redactURL(url), Body: string(body)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: HTTP %d: %s", redactURL(url), resp.StatusCode, string(body))
	}
	limiter.OnSuccess()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return json.Unmarshal(data, v)
}
