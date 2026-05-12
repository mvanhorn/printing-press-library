// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package crawl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
)

// Options controls a crawl.
type Options struct {
	// FullArchive also crawls the pre-2010 /tribe/FAQ/ day-page era.
	FullArchive bool
	// RatePerSec is the polite request ceiling (default 1.5/s).
	RatePerSec float64
	// MaxFAQ caps the number of FAQ month-pages fetched (0 = no cap);
	// used for fast smoke crawls, not normal refreshes.
	MaxFAQ int
	// UserAgent override; defaults to seykota-pp-cli's UA.
	UserAgent string
}

const userAgent = "seykota-pp-cli (+https://github.com/mvanhorn) — crawls seykota.com politely for an offline archive"

// Crawler fetches and parses seykota.com. Outbound requests are paced by an
// adaptive limiter and 429s are retried with backoff; an exhausted retry
// budget surfaces as *cliutil.RateLimitError rather than empty results.
type Crawler struct {
	hc      *http.Client
	lim     *cliutil.AdaptiveLimiter
	ua      string
	onPage  func(msg string)
}

// New returns a Crawler. progress, if non-nil, receives one line per page.
func New(opts Options, progress func(string)) *Crawler {
	rate := opts.RatePerSec
	if rate <= 0 {
		rate = 1.5
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = userAgent
	}
	if progress == nil {
		progress = func(string) {}
	}
	return &Crawler{
		hc:     &http.Client{Timeout: 30 * time.Second},
		lim:    cliutil.NewAdaptiveLimiter(rate),
		ua:     ua,
		onPage: progress,
	}
}

func (c *Crawler) get(ctx context.Context, url string) (string, error) {
	const maxAttempts = 4
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		c.lim.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", c.ua)
		resp, err := c.hc.Do(req)
		if err != nil {
			if attempt+1 < maxAttempts {
				time.Sleep(cliutil.Backoff(attempt))
				continue
			}
			return "", fmt.Errorf("GET %s: %w", url, err)
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if err != nil {
				return "", fmt.Errorf("reading %s: %w", url, err)
			}
			c.lim.OnSuccess()
			return string(b), nil
		case resp.StatusCode == http.StatusTooManyRequests:
			ra := cliutil.RetryAfter(resp)
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			c.lim.OnRateLimit()
			if attempt+1 < maxAttempts {
				wait := ra
				if wait <= 0 {
					wait = cliutil.Backoff(attempt)
				}
				time.Sleep(wait)
				continue
			}
			return "", &cliutil.RateLimitError{URL: url, RetryAfter: ra, Body: strings.TrimSpace(string(body))}
		case resp.StatusCode >= 500:
			resp.Body.Close()
			if attempt+1 < maxAttempts {
				time.Sleep(cliutil.Backoff(attempt))
				continue
			}
			return "", fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
		default:
			resp.Body.Close()
			return "", fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
		}
	}
	return "", fmt.Errorf("GET %s: exhausted retries", url)
}

// Crawl walks the FAQ index, the TSP index, and the risk essay, returning
// every parsed document. The slice is ordered FAQ (newest first) → TSP →
// risk essay, with Ord assigned within each source.
func (c *Crawler) Crawl(ctx context.Context, opts Options) ([]corpus.Doc, error) {
	var docs []corpus.Doc

	// --- FAQ ---
	c.onPage("fetching FAQ index …")
	faqIdxURL := base + "/tt/FAQ_Index/"
	idxHTML, err := c.get(ctx, faqIdxURL)
	if err != nil {
		return nil, fmt.Errorf("FAQ index: %w", err)
	}
	monthLinks := FAQMonthLinks(idxHTML, faqIdxURL)
	if opts.FullArchive {
		oldIdxURL := base + "/tribe/FAQ/index.htm"
		if oldHTML, err := c.get(ctx, oldIdxURL); err == nil {
			seen := map[string]bool{}
			for _, u := range monthLinks {
				seen[u] = true
			}
			for _, u := range FAQMonthLinks(oldHTML, oldIdxURL) {
				if !seen[u] {
					seen[u] = true
					monthLinks = append(monthLinks, u)
				}
			}
		} else {
			c.onPage(fmt.Sprintf("warning: legacy FAQ index unavailable: %v", err))
		}
	}
	if opts.MaxFAQ > 0 && len(monthLinks) > opts.MaxFAQ {
		monthLinks = monthLinks[:opts.MaxFAQ]
	}
	c.onPage(fmt.Sprintf("FAQ: %d month-pages to fetch", len(monthLinks)))
	for i, u := range monthLinks {
		h, err := c.get(ctx, u)
		if err != nil {
			var rle *cliutil.RateLimitError
			if errors.As(err, &rle) {
				return nil, err
			}
			c.onPage(fmt.Sprintf("  skip %s: %v", u, err))
			continue
		}
		d := ParseFAQMonth(u, h)
		d.Ord = i
		docs = append(docs, d)
		if (i+1)%25 == 0 || i+1 == len(monthLinks) {
			c.onPage(fmt.Sprintf("  FAQ %d/%d", i+1, len(monthLinks)))
		}
	}

	// --- TSP ---
	c.onPage("fetching TSP index …")
	tspIdxURL := base + "/tribe/TSP/index.htm"
	tspHTML, err := c.get(ctx, tspIdxURL)
	if err != nil {
		c.onPage(fmt.Sprintf("warning: TSP index unavailable: %v", err))
	} else {
		links := TSPSectionLinks(tspHTML, tspIdxURL)
		// belt-and-suspenders: a few TSP sections use Index.html / index.html
		for _, extra := range []string{
			base + "/tribe/TSP/Skid/Index.html",
			base + "/tribe/TSP/Core/index.html",
			base + "/tribe/TSP/Further_Research_CC/index.html",
		} {
			found := false
			for _, l := range links {
				if strings.EqualFold(l, extra) {
					found = true
					break
				}
			}
			if !found {
				links = append(links, extra)
			}
		}
		c.onPage(fmt.Sprintf("TSP: %d section pages", len(links)))
		for i, u := range links {
			h, err := c.get(ctx, u)
			if err != nil {
				var rle *cliutil.RateLimitError
				if errors.As(err, &rle) {
					return nil, err
				}
				c.onPage(fmt.Sprintf("  skip %s: %v", u, err))
				continue
			}
			d := ParseTSPSection(u, h, i)
			if strings.TrimSpace(d.Body) == "" {
				continue
			}
			docs = append(docs, d)
		}
	}

	// --- Risk essay ---
	c.onPage("fetching risk essay …")
	riskURL := base + "/tribe/risk/index.htm"
	if h, err := c.get(ctx, riskURL); err != nil {
		c.onPage(fmt.Sprintf("warning: risk essay unavailable: %v", err))
	} else {
		docs = append(docs, ParseRiskEssay(riskURL, h))
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("crawl produced no documents")
	}
	return docs, nil
}
