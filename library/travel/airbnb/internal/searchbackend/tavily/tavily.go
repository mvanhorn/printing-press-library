package tavily

import (
	"bytes"
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
	searchbackend.Register("tavily", func() searchbackend.Backend {
		return &Backend{http: &http.Client{Timeout: 30 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(0.5)}
	})
}
func (b *Backend) Name() string { return "tavily" }

func (b *Backend) Search(ctx context.Context, query string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	return b.post(ctx, "https://api.tavily.com/search", map[string]any{"api_key": os.Getenv("TAVILY_API_KEY"), "query": query, "max_results": max(opts.Limit, 10)}, opts)
}

func (b *Backend) ImageSearch(ctx context.Context, photoURL string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	return b.post(ctx, "https://api.tavily.com/image_search", map[string]any{"api_key": os.Getenv("TAVILY_API_KEY"), "query": photoURL, "max_results": max(opts.Limit, 10)}, opts)
}

func (b *Backend) post(ctx context.Context, target string, body map[string]any, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	if body["api_key"] == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY is not set")
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var respData []byte
	for attempt := 0; attempt <= 3; attempt++ {
		b.limiter.Wait()
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}
		resp, err := b.http.Do(req)
		if err != nil {
			return nil, err
		}
		respData, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			b.limiter.OnRateLimit()
			if attempt == 3 {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp), Body: string(respData)}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("tavily HTTP %d: %s", resp.StatusCode, string(respData))
		}
		b.limiter.OnSuccess()
		break
	}
	var root map[string]any
	if err := json.Unmarshal(respData, &root); err != nil {
		return nil, err
	}
	arr, _ := root["results"].([]any)
	out := make([]searchbackend.Result, 0, len(arr))
	for _, item := range arr {
		m, _ := item.(map[string]any)
		link, _ := m["url"].(string)
		title, _ := m["title"].(string)
		snippet, _ := m["content"].(string)
		out = append(out, searchbackend.Result{Title: title, URL: link, Snippet: snippet, Domain: domainOf(link), Score: num(m["score"])})
	}
	return out, nil
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

func max(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func num(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
