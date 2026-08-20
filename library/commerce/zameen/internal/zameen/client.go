package zameen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

// Client fetches and parses Zameen search pages over standard HTTP.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	base    string
	ua      string
}

// NewClient returns a Zameen client with adaptive per-source rate limiting.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiterAuto(2),
		base:    BaseURL,
		ua:      UserAgent,
	}
}

// PageResult is the parsed content of one Zameen search page.
type PageResult struct {
	hits    []hit
	NbHits  int
	NbPages int
}

// stateEnvelope is the subset of window.state we parse.
type stateEnvelope struct {
	Algolia struct {
		Content struct {
			Hits    []hit `json:"hits"`
			NbHits  int   `json:"nbHits"`
			NbPages int   `json:"nbPages"`
		} `json:"content"`
	} `json:"algolia"`
}

// SearchPageURL builds the Zameen search URL for a category/location/page.
func SearchPageURL(category, location string, page int) string {
	if page < 1 {
		page = 1
	}
	return fmt.Sprintf("%s/%s/%s-%d.html", BaseURL, category, location, page)
}

// fetchPage GETs one search page and parses the embedded window.state hits.
func (c *Client) fetchPage(ctx context.Context, category, location string, page int) (*PageResult, error) {
	url := SearchPageURL(category, location, page)
	if c.limiter != nil {
		c.limiter.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       strings.TrimSpace(string(body)),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zameen returned HTTP %d for %s", resp.StatusCode, url)
	}
	if c.limiter != nil {
		c.limiter.OnSuccess()
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}
	state, err := extractState(body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", url, err)
	}
	var env stateEnvelope
	if err := json.Unmarshal(state, &env); err != nil {
		return nil, fmt.Errorf("decoding listing state from %s: %w", url, err)
	}
	return &PageResult{
		hits:    env.Algolia.Content.Hits,
		NbHits:  env.Algolia.Content.NbHits,
		NbPages: env.Algolia.Content.NbPages,
	}, nil
}

// Page fetches one page and returns the converted listings plus totals.
func (c *Client) Page(ctx context.Context, category, location string, page int) ([]types.Listing, int, int, error) {
	pr, err := c.fetchPage(ctx, category, location, page)
	if err != nil {
		return nil, 0, 0, err
	}
	out := make([]types.Listing, 0, len(pr.hits))
	seen := make(map[string]bool, len(pr.hits))
	for _, h := range pr.hits {
		l := h.toListing()
		if l.ExternalId != "" {
			if seen[l.ExternalId] {
				continue // Zameen injects featured + organic copies of a listing.
			}
			seen[l.ExternalId] = true
		}
		out = append(out, l)
	}
	return out, pr.NbHits, pr.NbPages, nil
}

// extractState locates `window.state = {…}` in the page HTML and returns the
// brace-balanced JSON object, ignoring braces inside string literals.
func extractState(html []byte) ([]byte, error) {
	marker := []byte("window.state")
	idx := bytes.Index(html, marker)
	if idx < 0 {
		return nil, fmt.Errorf("window.state not found (page shape changed or blocked)")
	}
	start := bytes.IndexByte(html[idx:], '{')
	if start < 0 {
		return nil, fmt.Errorf("window.state opening brace not found")
	}
	start += idx
	depth := 0
	inStr := false
	esc := false
	var quote byte
	for i := start; i < len(html); i++ {
		ch := html[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == quote:
				inStr = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inStr = true
			quote = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : i+1], nil
			}
		}
	}
	return nil, fmt.Errorf("window.state object was not terminated")
}

// SearchParams describes a client-side filtered search over scanned pages.
type SearchParams struct {
	Category     string
	Location     string
	Area         string
	PropertyType string
	Purpose      string
	MinPrice     int
	MaxPrice     int
	MinBeds      int
	MaxBeds      int
	MinBaths     int
	MinAreaMarla float64
	MaxAreaMarla float64
	VerifiedOnly bool
	Sort         string
	Limit        int
	MaxScanPages int
}

// SearchResult carries matched listings plus scan bookkeeping so callers can
// tell "no matches after scanning N pages" from "no listings exist".
type SearchResult struct {
	Listings   []types.Listing
	Scanned    int
	TotalHits  int
	TotalPages int
	ScanPages  int
	ScanCapHit bool
	// PartialError is set when a non-fatal fetch error stopped the scan mid-way
	// (page > 1). Callers should surface it so results are not mistaken for a
	// complete scan. Rate-limit errors are returned as errors, not stored here.
	PartialError string
}

func (p SearchParams) matches(h hit) bool {
	if p.MinPrice > 0 && h.Price < p.MinPrice {
		return false
	}
	if p.MaxPrice > 0 && h.Price > p.MaxPrice {
		return false
	}
	if p.MinBeds > 0 && h.Rooms < p.MinBeds {
		return false
	}
	if p.MaxBeds > 0 && h.Rooms > p.MaxBeds {
		return false
	}
	if p.MinBaths > 0 && h.Baths < p.MinBaths {
		return false
	}
	marla := 0.0
	if h.Area > 0 {
		marla = h.Area / AreaSqmPerMarla
	}
	if p.MinAreaMarla > 0 && marla < p.MinAreaMarla {
		return false
	}
	if p.MaxAreaMarla > 0 && marla > p.MaxAreaMarla {
		return false
	}
	if p.VerifiedOnly && !h.IsVerified {
		return false
	}
	if strings.TrimSpace(p.PropertyType) != "" {
		want := strings.ToLower(p.PropertyType)
		got := ""
		if len(h.Category) > 0 {
			got = strings.ToLower(h.Category[0].Name)
		}
		// Also match against deeper subtype names (flat/apartment/house...).
		matched := strings.Contains(got, want)
		for _, cc := range h.Category {
			if strings.Contains(strings.ToLower(cc.Name), want) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	if !h.matchesArea(p.Area) {
		return false
	}
	return true
}

func sortListings(items []types.Listing, sortKey string) {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "price-asc", "price_asc", "price":
		sort.SliceStable(items, func(i, j int) bool { return items[i].Price < items[j].Price })
	case "price-desc", "price_desc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].Price > items[j].Price })
	case "area-asc", "area_asc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].AreaMarla < items[j].AreaMarla })
	case "area-desc", "area_desc", "area":
		sort.SliceStable(items, func(i, j int) bool { return items[i].AreaMarla > items[j].AreaMarla })
	case "newest", "date", "":
		sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	}
}

// Search pages through Zameen results applying client-side filters and sort.
// It bounds scan effort with MaxScanPages (records examined) separately from
// Limit (matches returned). Under live-dogfood it curtails scanning to 1 page.
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	if p.Limit <= 0 {
		p.Limit = 25
	}
	if p.MaxScanPages <= 0 {
		p.MaxScanPages = 5
	}
	if cliutil.IsDogfoodEnv() && p.MaxScanPages > 1 {
		p.MaxScanPages = 1
	}

	res := &SearchResult{ScanCapHit: true}
	var matched []types.Listing
	seen := make(map[string]bool)

	for page := 1; page <= p.MaxScanPages; page++ {
		pr, err := c.fetchPage(ctx, p.Category, p.Location, page)
		if err != nil {
			// Rate limits always propagate so callers emit a rate-limit exit
			// code instead of silently returning a truncated result set.
			var rl *cliutil.RateLimitError
			if errors.As(err, &rl) || page == 1 {
				return nil, err
			}
			// Non-fatal mid-scan error: stop, return the partial set, but do
			// NOT claim we hit the scan cap, and record the error so the caller
			// can surface it rather than advise "raise --max-scan-pages".
			res.ScanCapHit = false
			res.PartialError = err.Error()
			break
		}
		if page == 1 {
			res.TotalHits = pr.NbHits
			res.TotalPages = pr.NbPages
		}
		res.ScanPages = page
		for _, h := range pr.hits {
			res.Scanned++
			if !p.matches(h) {
				continue
			}
			l := h.toListing()
			if l.ExternalId != "" {
				if seen[l.ExternalId] {
					continue // dedup Zameen featured + organic copies
				}
				seen[l.ExternalId] = true
			}
			matched = append(matched, l)
		}
		if len(pr.hits) == 0 || (pr.NbPages > 0 && page >= pr.NbPages) {
			res.ScanCapHit = false
			break
		}
		if p.Sort == "" && len(matched) >= p.Limit {
			// No global sort requested: streaming cap once we have enough.
			break
		}
	}

	sortListings(matched, p.Sort)
	if len(matched) > p.Limit {
		matched = matched[:p.Limit]
	}
	res.Listings = matched
	return res, nil
}
