// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const MaxBody = 4 * 1024 * 1024

type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("JAPAN47GO source unavailable HTTP%d at %s", e.Status, e.URL)
}

type Client struct {
	HTTP    *http.Client
	Base    string
	limiter *cliutil.AdaptiveLimiter
}

func NewClient(rate float64) *Client {
	return &Client{HTTP: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || r.URL.Scheme != "https" || r.URL.Host != "www.japan47go.travel" {
			return fmt.Errorf("unexpected source redirect")
		}
		return nil
	}}, Base: BaseURL, limiter: cliutil.NewAdaptiveLimiter(rate)}
}
func (c *Client) fetch(ctx context.Context, route string) ([]byte, error) {
	if e := c.limiter.Wait(ctx); e != nil {
		return nil, e
	}
	u := c.Base + route
	r, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return nil, e
	}
	r.Header.Set("User-Agent", "japan47go-pp-cli/0.0.0-dev (read-only tourism evidence)")
	resp, e := c.HTTP.Do(r)
	if e != nil {
		return nil, fmt.Errorf("source transport: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, &HTTPError{resp.StatusCode, u}
	}
	c.limiter.OnSuccess()
	b, e := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if e != nil {
		return nil, e
	}
	if len(b) > MaxBody {
		return nil, fmt.Errorf("source response exceeds 4MiB bound")
	}
	return PageProps(b)
}
func (c *Client) Get(ctx context.Context, id string) (Service, error) {
	id, e := ID(id)
	if e != nil {
		return Service{}, e
	}
	p, e := c.fetch(ctx, "/ja/detail/"+id)
	if e != nil {
		return Service{}, e
	}
	return ParseDetail(p, id, time.Now())
}

type DiscoverOptions struct {
	Query    string
	Kind     string
	MaxPages int
	Limit    int
}

func (c *Client) Discover(ctx context.Context, o DiscoverOptions) (Discovery, error) {
	out := Discovery{Candidates: []Candidate{}, Query: map[string]any{"keyword": o.Query, "kind": o.Kind, "max_pages": o.MaxPages, "limit": o.Limit}, Coverage: Coverage{Routes: []string{}}, SourceBoundary: "Listing candidates only; details, request feasibility and live availability are unverified."}
	if o.MaxPages < 1 || o.MaxPages > 5 || o.Limit < 1 || o.Limit > 50 {
		return out, fmt.Errorf("--max-pages must be 1..5 and --limit 1..50")
	}
	tag := "62"
	if o.Kind == "experiences" {
		tag = "69"
	} else if o.Kind != "guides" {
		return out, fmt.Errorf("--kind must be guides or experiences")
	}
	seen := map[string]bool{}
	dropped := false
	for page := 1; page <= o.MaxPages; page++ {
		q := url.Values{"tag": {tag}, "page": {strconv.Itoa(page)}}
		if o.Query != "" {
			q.Set("keyword", o.Query)
		}
		route := "/ja/search/result?" + q.Encode()
		props, e := c.fetch(ctx, route)
		if e != nil {
			return out, e
		}
		items, cov, e := ParseListing(props, page, time.Now())
		if e != nil {
			return out, e
		}
		out.Coverage.Pages++
		out.Coverage.Records += len(items)
		out.Coverage.Routes = append(out.Coverage.Routes, BaseURL+route)
		out.Coverage.MatchingTotal = cov.MatchingTotal
		out.Coverage.TotalPages = cov.TotalPages
		out.Coverage.PageSize = cov.PageSize
		for _, v := range items {
			if !seen[v.ID] {
				seen[v.ID] = true
				if len(out.Candidates) < o.Limit {
					out.Candidates = append(out.Candidates, v)
				} else {
					dropped = true
				}
			}
		}
		if page >= cov.TotalPages {
			out.Coverage.NextPage = nil
			break
		}
		n := page + 1
		out.Coverage.NextPage = &n
		if len(out.Candidates) >= o.Limit {
			break
		}
	}
	out.Coverage.More = out.Coverage.NextPage != nil || dropped
	out.Coverage.Returned = len(out.Candidates)
	out.Coverage.Complete = !out.Coverage.More && !dropped
	out.Coverage.Note = "Only reported source listing pages were inspected; an empty bounded window does not establish absence. --max-pages widens page coverage and --limit widens returned candidates."
	return out, nil
}
