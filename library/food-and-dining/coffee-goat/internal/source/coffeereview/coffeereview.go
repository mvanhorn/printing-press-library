// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package coffeereview is the source adapter for coffeereview.com.
// The primary path uses the WordPress REST API at
// /wp-json/wp/v2/posts; the score is parsed out of content.rendered
// via regex because the API doesn't expose it as a structured field.
//
// The RSS feed at /feed/ is the documented fallback but isn't
// exercised in this initial cut — the WP REST path covers the
// happy path and a fallback would add code surface we can't yet
// dogfood.
package coffeereview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
)

const baseURL = "https://www.coffeereview.com"

// Review is the unified shape this adapter emits. Stored in the
// reviews table.
type Review struct {
	ID          string
	SourceURL   string
	RoasterName string
	BeanName    string
	Score       int
	PublishedAt string
	Reviewer    string
	RawJSON     string
}

// Fetcher is the adapter entrypoint. Limiter may be nil for no
// pacing; HTTP defaults to a 20s timeout.
type Fetcher struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

// New returns a Fetcher with sensible defaults.
func New() *Fetcher {
	return &Fetcher{
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// scoreRE extracts a "<two-or-three-digit-score>/100" or
// "<score>.<frac>/100" pattern from the rendered HTML content. The
// brief spec mandates a \b word boundary on the digits to prevent
// matching arbitrary `1.123` substrings; the double-escape is the
// Go-string literal escape of the regex literal \b.
var scoreRE = regexp.MustCompile(`\b(\d{2,3})\s*[\./]\s*100\b`)

// titleSplitRE splits a typical Coffee Review title into roaster +
// bean. Coffee Review uses formats like "Onyx Coffee Lab Geometry"
// or "Sey - Tito Wush Wush" — the split is best-effort and tolerant.
var titleSplitRE = regexp.MustCompile(`\s*[\-–:]\s*`)

// wpPost is the slice of the WP REST response we consume.
type wpPost struct {
	ID    int `json:"id"`
	Date  string
	Slug  string
	Link  string
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
}

// Fetch returns up to `pages` pages of WP REST posts, perPage rows
// per page. Empty perPage falls back to 10; perPage > 100 is clamped
// by the upstream API.
func (f *Fetcher) Fetch(ctx context.Context, perPage, pages int) ([]Review, error) {
	if perPage <= 0 {
		perPage = 10
	}
	if pages <= 0 {
		pages = 1
	}
	// Under verify or dogfood, only fetch one tiny page.
	if cliutil.IsVerifyEnv() {
		return nil, nil
	}
	if cliutil.IsDogfoodEnv() {
		pages = 1
		if perPage > 5 {
			perPage = 5
		}
	}

	var out []Review
	for page := 1; page <= pages; page++ {
		f.Limiter.Wait()
		url := fmt.Sprintf("%s/wp-json/wp/v2/posts?per_page=%d&page=%d", baseURL, perPage, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return out, fmt.Errorf("coffeereview.Fetch: %w", err)
		}
		req.Header.Set("User-Agent", "coffee-goat-pp-cli (+specialty-coffee aggregator)")
		req.Header.Set("Accept", "application/json")

		resp, err := f.HTTP.Do(req)
		if err != nil {
			return out, fmt.Errorf("coffeereview.Fetch: %w", err)
		}
		if resp.StatusCode == 429 {
			f.Limiter.OnRateLimit()
			retry := cliutil.RetryAfter(resp)
			resp.Body.Close()
			return out, &cliutil.RateLimitError{URL: url, RetryAfter: retry}
		}
		if resp.StatusCode == 400 || resp.StatusCode == 404 {
			// Page beyond available content — clean stop, not an error.
			resp.Body.Close()
			break
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return out, fmt.Errorf("coffeereview.Fetch: HTTP %d: %s", resp.StatusCode, string(body))
		}
		f.Limiter.OnSuccess()

		var posts []wpPost
		if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
			resp.Body.Close()
			return out, fmt.Errorf("coffeereview.Fetch decode: %w", err)
		}
		resp.Body.Close()
		if len(posts) == 0 {
			break
		}

		for _, p := range posts {
			rev := normalise(p)
			if rev.Score == 0 {
				// A post without a score is editorial commentary, not a
				// review — skip it.
				continue
			}
			out = append(out, rev)
		}
		if len(posts) < perPage {
			break
		}
	}
	return out, nil
}

// normalise parses a single WP post into a Review.
func normalise(p wpPost) Review {
	raw, _ := json.Marshal(p)
	title := strings.TrimSpace(stripTags(p.Title.Rendered))
	roaster, bean := splitTitle(title)
	score := 0
	if m := scoreRE.FindStringSubmatch(p.Content.Rendered); len(m) > 1 {
		score = atoi(m[1])
	}
	return Review{
		ID:          fmt.Sprintf("coffeereview:%d", p.ID),
		SourceURL:   p.Link,
		RoasterName: roaster,
		BeanName:    bean,
		Score:       score,
		PublishedAt: p.Date,
		RawJSON:     string(raw),
	}
}

func splitTitle(title string) (roaster, bean string) {
	parts := titleSplitRE.Split(title, 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return title, ""
}

func stripTags(s string) string {
	// Same shape as extract.Cleanup but without dragging the package in.
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
