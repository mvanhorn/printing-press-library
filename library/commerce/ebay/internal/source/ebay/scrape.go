package ebay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ebay/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/ebay/internal/cliutil"
)

// Source is a thin wrapper around the printed CLI's HTTP client that knows
// how to drive the public eBay search HTML surfaces. All methods return
// structured Go data, parsed locally from the response body.
type Source struct {
	c       *client.Client
	limiter *cliutil.AdaptiveLimiter
}

// New wraps an existing CLI client.
func New(c *client.Client) *Source {
	return &Source{c: c, limiter: cliutil.NewAdaptiveLimiter(c.RateLimit())}
}

// FetchSold queries /sch/i.html with LH_Sold=1&LH_Complete=1 and returns
// structured SoldItem rows.
func (s *Source) FetchSold(ctx context.Context, opts SoldOptions) ([]SoldItem, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 240
	}
	q := url.Values{}
	q.Set("_nkw", opts.Query)
	q.Set("LH_Sold", "1")
	q.Set("LH_Complete", "1")
	q.Set("_ipg", itoa(opts.PerPage))
	if opts.Category != "" {
		q.Set("_sacat", opts.Category)
	}
	if opts.MinPrice > 0 {
		q.Set("_udlo", ftoa(opts.MinPrice))
	}
	if opts.MaxPrice > 0 {
		q.Set("_udhi", ftoa(opts.MaxPrice))
	}
	if opts.Page > 1 {
		q.Set("_pgn", itoa(opts.Page))
	}
	body, err := s.getHTML("/sch/i.html", q)
	if err != nil {
		return nil, err
	}
	items, err := ParseSoldHTML(body)
	if err != nil {
		return nil, err
	}
	if opts.WindowDays > 0 {
		cut := time.Now().AddDate(0, 0, -opts.WindowDays)
		filtered := items[:0]
		for _, it := range items {
			if it.SoldDate.IsZero() || !it.SoldDate.Before(cut) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if opts.Condition != "" {
		want := strings.ToLower(opts.Condition)
		filtered := items[:0]
		for _, it := range items {
			if strings.Contains(strings.ToLower(it.Condition), want) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	return items, nil
}

// FetchActive queries /sch/i.html with optional auction/BIN filters and bid-count
// + ending-window post-filtering applied locally.
func (s *Source) FetchActive(ctx context.Context, opts SearchOptions) ([]Listing, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 60
	}
	q := url.Values{}
	q.Set("_nkw", opts.Query)
	q.Set("_ipg", itoa(opts.PerPage))
	if opts.Auction {
		q.Set("LH_Auction", "1")
	}
	if opts.BIN {
		q.Set("LH_BIN", "1")
	}
	if opts.Category != "" {
		q.Set("_sacat", opts.Category)
	}
	if opts.MinPrice > 0 {
		q.Set("_udlo", ftoa(opts.MinPrice))
	}
	if opts.MaxPrice > 0 {
		q.Set("_udhi", ftoa(opts.MaxPrice))
	}
	if opts.Page > 1 {
		q.Set("_pgn", itoa(opts.Page))
	}
	q.Set("_sop", sortCode(opts.Sort))
	body, err := s.getHTML("/sch/i.html", q)
	if err != nil {
		return nil, err
	}
	items, err := ParseSearchHTML(body)
	if err != nil {
		return nil, err
	}
	// Local post-filters
	if opts.HasBids || opts.MinBids > 0 || opts.MaxBids > 0 {
		min := opts.MinBids
		if opts.HasBids && min == 0 {
			min = 1
		}
		filtered := items[:0]
		for _, it := range items {
			if it.Bids < min {
				continue
			}
			if opts.MaxBids > 0 && it.Bids > opts.MaxBids {
				continue
			}
			filtered = append(filtered, it)
		}
		items = filtered
	}
	if opts.EndsWithin > 0 {
		cut := time.Now().Add(opts.EndsWithin)
		filtered := items[:0]
		for _, it := range items {
			if it.EndsAt.IsZero() {
				continue
			}
			if it.EndsAt.Before(cut) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	return items, nil
}

// FetchItem fetches a single item detail HTML and returns a Listing snapshot.
func (s *Source) FetchItem(ctx context.Context, itemID string) (*Listing, error) {
	body, err := s.getHTML("/itm/"+itemID, nil)
	if err != nil {
		return nil, err
	}
	// Minimal item parse: title, price.
	listings, err := ParseSearchHTML(body)
	if err != nil {
		return nil, err
	}
	if len(listings) == 0 {
		return nil, fmt.Errorf("could not parse item %s page", itemID)
	}
	listings[0].ItemID = itemID
	return &listings[0], nil
}

// getHTML drives the underlying client with HTML response handling. The
// generated client returns the raw body via json.RawMessage; we just need it
// as bytes. Honors AdaptiveLimiter and surfaces typed RateLimitError on 429
// so callers can distinguish "no data" from "throttled" — silent empty
// results would mask Akamai bot-manager challenges as legitimate misses.
func (s *Source) getHTML(path string, q url.Values) ([]byte, error) {
	params := map[string]string{}
	for k := range q {
		params[k] = q.Get(k)
	}
	if s.c.DryRun {
		return []byte("<html><!-- dry-run --></html>"), nil
	}
	if s.limiter != nil {
		s.limiter.Wait()
	}
	raw, err := s.c.Get(path, params)
	if err != nil {
		// Surface 429 as the typed RateLimitError so callers do not confuse
		// throttling with empty result sets.
		if isRateLimit(err) {
			if s.limiter != nil {
				s.limiter.OnRateLimit()
			}
			return nil, &cliutil.RateLimitError{
				URL:  "https://www.ebay.com" + path,
				Body: err.Error(),
			}
		}
		return nil, err
	}
	if s.limiter != nil {
		s.limiter.OnSuccess()
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty response from %s", path)
	}
	// Strip surrounding JSON quoting if the client decoded the response.
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return []byte(s), nil
		}
	}
	body := []byte(raw)
	// Akamai/eBay returns a 200 challenge HTML page in some throttling
	// scenarios. Detect the canonical "Access Denied" / "Just a moment"
	// shapes and surface them as RateLimitError too.
	if looksLikeChallenge(body) {
		if s.limiter != nil {
			s.limiter.OnRateLimit()
		}
		return nil, &cliutil.RateLimitError{
			URL:  "https://www.ebay.com" + path,
			Body: "bot-protection challenge response (run `ebay-pp-cli auth login --chrome` and retry)",
		}
	}
	return body, nil
}

// isRateLimit recognises HTTP 429 responses from the underlying client.
// The generated client wraps non-2xx responses in *client.APIError with a
// StatusCode field; rather than import that struct directly here we
// pattern-match the error message. This keeps the source/ebay package
// decoupled from the client error type while still emitting a typed
// RateLimitError to callers.
func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 429") || strings.Contains(msg, " 429:") || strings.Contains(strings.ToLower(msg), "rate limit")
}

func looksLikeChallenge(body []byte) bool {
	if len(body) > 32*1024 {
		return false
	}
	low := strings.ToLower(string(body))
	for _, m := range []string{
		"access denied",
		"just a moment",
		"please enable javascript",
		"captcha",
		"cf-mitigated",
		"x-vercel-mitigated",
		"akamai",
	} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
func ftoa(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.2f", f)
}

func sortCode(s string) string {
	switch strings.ToLower(s) {
	case "ending", "ending-soonest":
		return "1"
	case "newest", "newly-listed":
		return "10"
	case "newest-listed":
		return "12"
	case "ending-latest":
		return "13"
	case "price-asc":
		return "15"
	case "price-desc":
		return "16"
	default:
		return ""
	}
}
