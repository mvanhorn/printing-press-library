package parallel

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
	searchbackend.Register("parallel", func() searchbackend.Backend {
		return &Backend{http: &http.Client{Timeout: 60 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(0.5)}
	})
}
func (b *Backend) Name() string { return "parallel" }

func (b *Backend) Search(ctx context.Context, query string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	return b.task(ctx, map[string]any{"input": query, "processor": "lite"}, opts)
}

func (b *Backend) ImageSearch(ctx context.Context, photoURL string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	return b.task(ctx, map[string]any{"input": map[string]any{"image_url": photoURL}, "processor": "base"}, opts)
}

func (b *Backend) task(ctx context.Context, body map[string]any, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	key := os.Getenv("PARALLEL_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("PARALLEL_API_KEY is not set")
	}
	root, err := b.post(ctx, "https://api.parallel.ai/v1/tasks", key, body)
	if err != nil {
		return nil, err
	}
	id, _ := root["id"].(string)
	if id == "" {
		return resultsFromAny(root, opts), nil
	}
	for i := 0; i < 20; i++ {
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		poll, err := b.get(ctx, "https://api.parallel.ai/v1/tasks/"+url.PathEscape(id)+"/runs/latest", key)
		if err != nil {
			return nil, err
		}
		status, _ := poll["status"].(string)
		if status == "completed" || poll["result"] != nil {
			return resultsFromAny(poll, opts), nil
		}
	}
	return nil, fmt.Errorf("parallel task timed out")
}

func (b *Backend) post(ctx context.Context, target, key string, body map[string]any) (map[string]any, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "x-api-key "+key)
	req.Header.Set("Content-Type", "application/json")
	return b.do(req)
}

func (b *Backend) get(ctx context.Context, target, key string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "x-api-key "+key)
	return b.do(req)
}

func (b *Backend) do(req *http.Request) (map[string]any, error) {
	var data []byte
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
		data, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			b.limiter.OnRateLimit()
			if attempt == 3 {
				return nil, &cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("parallel HTTP %d: %s", resp.StatusCode, string(data))
		}
		b.limiter.OnSuccess()
		break
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root, nil
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
			title := str(x["title"])
			link := str(x["url"])
			if link == "" {
				link = str(x["link"])
			}
			if title != "" && link != "" {
				out = append(out, searchbackend.Result{Title: title, URL: link, Snippet: str(x["snippet"]), Domain: domainOf(link), Score: 1 - float64(len(out))*0.05})
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

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}
