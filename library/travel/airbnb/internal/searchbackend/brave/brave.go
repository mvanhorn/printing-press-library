package brave

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend"
)

type Backend struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	searchbackend.Register("brave", func() searchbackend.Backend {
		return &Backend{http: &http.Client{Timeout: 20 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(1)}
	})
}
func (b *Backend) Name() string { return "brave" }

func (b *Backend) Search(ctx context.Context, query string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	u := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query)
	return b.get(ctx, u, "$.web.results", opts)
}

func (b *Backend) ImageSearch(ctx context.Context, photoURL string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	u := "https://api.search.brave.com/res/v1/images/search?q=" + url.QueryEscape(photoURL)
	return b.get(ctx, u, "$.results", opts)
}

func (b *Backend) get(ctx context.Context, target, _ string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	key := os.Getenv("BRAVE_SEARCH_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("BRAVE_SEARCH_API_KEY is not set")
	}
	var data []byte
	for attempt := 0; attempt <= 3; attempt++ {
		b.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Subscription-Token", key)
		req.Header.Set("Accept", "application/json")
		resp, err := b.http.Do(req)
		if err != nil {
			return nil, err
		}
		data, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			b.limiter.OnRateLimit()
			if attempt == 3 {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("brave search HTTP %d: %s", resp.StatusCode, string(data))
		}
		b.limiter.OnSuccess()
		break
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return resultsFromAny(root, opts), nil
}

func resultsFromAny(root any, opts searchbackend.SearchOpts) []searchbackend.Result {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	var out []searchbackend.Result
	var walk func(any)
	walk = func(v any) {
		if len(out) >= limit {
			return
		}
		switch x := v.(type) {
		case map[string]any:
			title, _ := x["title"].(string)
			link, _ := x["url"].(string)
			if link == "" {
				link, _ = x["link"].(string)
			}
			desc, _ := x["description"].(string)
			if title != "" && link != "" {
				out = append(out, searchbackend.Result{Title: title, URL: link, Snippet: desc, Domain: domainOf(link), Score: 1 - float64(len(out))*0.05})
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}
