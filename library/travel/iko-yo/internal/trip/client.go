// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package trip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
)

type SourceError struct {
	Status  int
	URL     string
	Problem string
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("Iko-yo Trip source %s: HTTP %d: %s", e.URL, e.Status, e.Problem)
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New(rate float64) *Client {
	if rate <= 0 {
		rate = 1
	}
	if rate > 2 {
		rate = 2
	}
	return &Client{http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("Trip redirect limit exceeded")
		}
		if r.URL.Scheme != "https" || r.URL.Host != "trip.iko-yo.net" {
			return fmt.Errorf("Trip redirect leaves the supported HTTPS source")
		}
		return nil
	}}, limiter: cliutil.NewAdaptiveLimiter(rate)}
}
func (c *Client) fetch(ctx context.Context, target string) ([]byte, error) {
	u, e := url.Parse(target)
	if e != nil || u.Scheme != "https" || u.Host != "trip.iko-yo.net" || u.User != nil {
		return nil, fmt.Errorf("only canonical public Iko-yo Trip HTTPS routes are supported")
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "iko-yo-pp-cli/0.1 (Iko-yo Trip read-only planning)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &SourceError{Status: resp.StatusCode, URL: target, Problem: "public page is unavailable; no empty result was inferred"}
	}
	if resp.Request.URL.Path != u.Path {
		return nil, &SourceError{Status: resp.StatusCode, URL: target, Problem: "source redirected to a different record or listing"}
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil, &SourceError{Status: resp.StatusCode, URL: target, Problem: "expected a public HTML page"}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBody {
		return nil, &SourceError{Status: resp.StatusCode, URL: target, Problem: "response exceeds the 2 MiB source limit"}
	}
	c.limiter.OnSuccess()
	return body, nil
}
func (c *Client) Inspect(ctx context.Context, ref string) (Record, error) {
	kind, id, err := ParseReference(ref)
	if err != nil {
		return Record{}, err
	}
	body, err := c.fetch(ctx, BaseURL+"/"+kind+"/"+id)
	if err != nil {
		return Record{}, err
	}
	return ParseDetail(body, kind+"/"+id, time.Now())
}
func (c *Client) Discover(ctx context.Context, kind string, region, prefecture, maxPages int, q Query) (Discovery, []Record, error) {
	out := Discovery{Records: make([]Candidate, 0), Coverage: make([]Coverage, 0), Scope: ScopeNote}
	all := make([]Record, 0)
	if kind != "spots" && kind != "events" && kind != "all" {
		return out, all, fmt.Errorf("--kind must be spots, events or all")
	}
	if region < 0 || region > 11 {
		return out, all, fmt.Errorf("--region must be a source region ID from 1 to 11")
	}
	if prefecture < 0 || prefecture > 47 || prefecture > 0 && region == 0 {
		return out, all, fmt.Errorf("--prefecture requires --region and must be a source prefecture ID from 1 to 47")
	}
	if maxPages < 1 || maxPages > 5 {
		return out, all, fmt.Errorf("--max-pages must be from 1 to 5")
	}
	kinds := []string{kind}
	if kind == "all" {
		kinds = []string{"spots", "events"}
		if maxPages < 2 {
			return out, all, fmt.Errorf("--kind all requires --max-pages at least 2; the cap applies across both listing kinds")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for i, k := range kinds {
		target := BaseURL + "/" + k
		if region > 0 {
			target += fmt.Sprintf("/regions/%d", region)
		}
		if prefecture > 0 {
			target += fmt.Sprintf("/prefectures/%d", prefecture)
		}
		listingURL := target
		cap := maxPages / len(kinds)
		if i < maxPages%len(kinds) {
			cap++
		}
		cov := Coverage{Kind: k, Region: region, Prefecture: prefecture, SourceURLs: make([]string, 0), ObservedAt: time.Now().UTC().Format(time.RFC3339)}
		group := make([]Record, 0)
		for pageNo := 1; pageNo <= cap && target != ""; pageNo++ {
			body, err := c.fetch(ctx, target)
			if err != nil {
				return out, all, err
			}
			page, err := ParseListing(body, k, target, time.Now())
			if err != nil {
				return out, all, err
			}
			if prefecture > 0 {
				if observedArea, valid := page.Areas[prefecture]; !valid || observedArea != listingURL {
					return out, all, fmt.Errorf("--prefecture %d is not an observed source option for --region %d", prefecture, region)
				}
			}
			cov.SourceURLs = append(cov.SourceURLs, target)
			cov.PagesScanned++
			cov.RecordsScanned += len(page.Records)
			cov.LastPage = page.LastPage
			group = append(group, page.Records...)
			target = page.NextURL
		}
		if target != "" {
			u, _ := url.Parse(target)
			cov.NextPage, _ = strconv.Atoi(u.Query().Get("page"))
		}
		cov.CompleteForListing = target == ""
		remaining := cov.LastPage - cov.PagesScanned
		if remaining < 0 {
			remaining = 0
		}
		cov.RemainingPages = &remaining
		for j := range group {
			group[j].Collection = cov
		}
		all = append(all, group...)
		out.Coverage = append(out.Coverage, cov)
	}
	coverage := out.Coverage
	out = Filter(all, q)
	out.Coverage = coverage
	out.Scope = ScopeNote
	return out, all, nil
}
