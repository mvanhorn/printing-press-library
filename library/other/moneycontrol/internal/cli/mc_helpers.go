// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored helpers for moneycontrol novel commands. Lives in its own file
// so a reprint preserves it; see AGENTS.md "Hand-edits must be regen-mergeable".
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/mvanhorn/printing-press-library/library/other/moneycontrol/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/moneycontrol/internal/config"
)

// priceAPIBase is the host that serves index and equity pricefeeds. It is open
// to plain HTTP and is what the generated indices/stocks commands target via
// base_url_override; novel compound commands reach it through this helper.
const priceAPIBase = "https://priceapi.moneycontrol.com"

// newPriceAPIClient returns a *client.Client pointed at priceapi.moneycontrol.com.
// Used by novel commands that need index/equity quotes directly, since flags.newClient()
// uses the default www.moneycontrol.com base.
func newPriceAPIClient(flags *rootFlags) (*client.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	cfg.BaseURL = priceAPIBase
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.DryRun = flags.dryRun
	c.NoCache = flags.noCache
	return c, nil
}

// articleLink is one discovered news headline from a listing or tag page.
type articleLink struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

var articleURLRegex = regexp.MustCompile(`^https?://www\.moneycontrol\.com/news/[^?#]+-\d+\.html$`)

// fetchNewsLinks fetches a moneycontrol news listing/tag page over the www
// client, parses the HTML with goquery, and returns deduplicated {url,title}
// pairs for anchors whose href matches the canonical article URL shape
// (/news/<cat>/<slug>-<id>.html). Going directly to the DOM avoids the generic
// link extractor, which returns navigation links preferentially and caps out
// before reaching article anchors deep in the page.
func fetchNewsLinks(ctx context.Context, c *client.Client, path string, limit int) ([]articleLink, error) {
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path, err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML from %s: %w", path, err)
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	out := make([]articleLink, 0)
	seen := make(map[string]bool)
	doc.Find("a[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, ok := s.Attr("href")
		if !ok {
			return true
		}
		href = strings.TrimSpace(href)
		// Resolve relative URLs against the site base.
		if strings.HasPrefix(href, "/") {
			href = baseURL + href
		}
		if !articleURLRegex.MatchString(href) {
			return true
		}
		if seen[href] {
			return true
		}
		title := strings.TrimSpace(s.Text())
		// Also try the title attribute when the link text is empty (image-only anchors).
		if title == "" {
			title, _ = s.Attr("title")
			title = strings.TrimSpace(title)
		}
		if title == "" {
			return true
		}
		seen[href] = true
		out = append(out, articleLink{URL: href, Title: title})
		if limit > 0 && len(out) >= limit {
			return false
		}
		return true
	})
	return out, nil
}

// fetchJSONWithClient GETs path on c and unmarshals into v. Used by novel
// commands that need typed JSON from priceapi or the www widgets.
func fetchJSONWithClient(ctx context.Context, c *client.Client, path string, v any) error {
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("parsing JSON from %s: %w", path, err)
	}
	return nil
}
