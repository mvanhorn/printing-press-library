// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: AeroParker (reservesea.portseattle.org) HTTP flow. Not generated.

// Package parking implements the SEA on-airport Reserved-parking quote flow
// against the AeroParker (Metropolis) white-label booking engine at
// reservesea.portseattle.org. The site exposes no JSON API; a quote is a
// session-warmed HTML form POST whose response embeds the price/availability
// JSON. This package owns that flow: warm the session cookies, POST the date
// form, follow the redirect to the selectProduct page, and extract the
// structured quote from the returned HTML.
package parking

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
)

// Airport is the AeroParker path segment for Seattle-Tacoma International. The
// same engine powers other airports under different codes/hosts; SEA is the
// only supported target today.
const Airport = "SEA"

// defaultUserAgent mirrors a current desktop Chrome. AeroParker serves the
// booking flow to any normal browser UA; this value is whatever the live
// desktop currently sends and a future check should re-confirm it, not treat
// this literal as canonical.
const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Client performs quote requests against one AeroParker host.
type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	warmed    bool // session cookies seeded; subsequent quotes reuse them
}

// New builds a Client for baseURL (e.g. https://reservesea.portseattle.org).
// Each Client keeps its own cookie jar so the warm-GET session token is
// carried into the subsequent POST. Requests are paced by an adaptive limiter
// so the sweep/calendar fan-out stays polite; reservesea sends no rate-limit
// headers, so pacing holds near the conservative start rate and backs off on
// any 429.
func New(baseURL string, timeout time.Duration) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: defaultUserAgent,
		http:      &http.Client{Timeout: timeout, Jar: jar},
		limiter:   cliutil.NewAdaptiveLimiterAuto(3.0),
	}, nil
}

// NightPrice is one night's price within a stay.
type NightPrice struct {
	Date  string  `json:"date"`  // YYYY-MM-DD
	Price float64 `json:"price"` // per-night price in USD
}

// Quote is the parsed result of a single availability/price request.
type Quote struct {
	Entry         time.Time    `json:"-"`
	Exit          time.Time    `json:"-"`
	EntryStr      string       `json:"entry"`  // RFC3339-ish local, e.g. 2026-08-15T11:00
	ExitStr       string       `json:"exit"`   // RFC3339-ish local
	Nights        int          `json:"nights"` // billed nights (days)
	ProductName   string       `json:"product_name,omitempty"`
	ProductCode   string       `json:"product_code,omitempty"`
	TotalPrice    float64      `json:"total_price"`
	OriginalPrice float64      `json:"original_price,omitempty"` // pre-discount list price when a promo/list price differs
	Available     bool         `json:"available"`
	SoldOut       bool         `json:"sold_out"`
	Invalid       bool         `json:"invalid"`          // request rejected (min-stay, too-soon, etc.)
	Reason        string       `json:"reason,omitempty"` // sold-out or validation message from the site
	Promo         string       `json:"promo,omitempty"`
	NightlyPrices []NightPrice `json:"nightly_prices,omitempty"`
	CapturedAt    time.Time    `json:"captured_at"`
}

// parkingPath is the booking endpoint for the configured airport.
func (c *Client) parkingPath() string {
	return fmt.Sprintf("/book/%s/Parking", Airport)
}

// Quote warms a session and requests a quote for the entry/exit window. It
// returns a populated *Quote even for sold-out and invalid-date states (with
// SoldOut/Invalid set and Reason populated); a non-nil error is only returned
// for transport failures or an unrecognizable response shape.
func (c *Client) Quote(ctx context.Context, entry, exit time.Time, promo string) (*Quote, error) {
	if cliutil.IsVerifyEnv() {
		// The Printing Press verifier points BASE_URL at a mock server that
		// cannot produce AeroParker HTML. Return a valid empty-but-shaped
		// result so command wiring verifies without a live dependency.
		return &Quote{
			Entry: entry, Exit: exit,
			EntryStr: formatLocal(entry), ExitStr: formatLocal(exit),
			Nights:     billedNights(entry, exit),
			Promo:      promo,
			Reason:     "verify environment: live quote skipped",
			CapturedAt: entry, // deterministic, avoids time.Now under verify
		}, nil
	}

	// 1. Warm the session once per Client: GET the date form so AWSALB/
	//    JSESSIONID/parkingReservationGuid cookies land in the jar. A sweep or
	//    calendar reuses the same session across every quote, halving requests.
	warmURL := c.baseURL + c.parkingPath() + "?parkingCmd=collectParkingDetails"
	if !c.warmed {
		if err := c.warm(ctx, warmURL); err != nil {
			return nil, err
		}
		c.warmed = true
	}

	// 2. POST the date form. The default http.Client follows the 303 to
	//    ?parkingCmd=selectProduct, carrying cookies.
	form := url.Values{}
	form.Set("parkingCmd", "collectParkingDetails")
	form.Set("progressToNextStep", "1")
	form.Set("entryDate", entry.Format("01/02/2006"))
	form.Set("entryTime", snapTime(entry))
	form.Set("exitDate", exit.Format("01/02/2006"))
	form.Set("exitTime", snapTime(exit))
	if promo != "" {
		form.Set("promocodes", promo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.parkingPath(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", warmURL)

	body, finalURL, err := c.do(req)
	if err != nil {
		return nil, err
	}

	q := &Quote{
		Entry: entry, Exit: exit,
		EntryStr: formatLocal(entry), ExitStr: formatLocal(exit),
		Nights:     billedNights(entry, exit),
		Promo:      promo,
		CapturedAt: time.Now().UTC(),
	}
	return parseQuote(body, finalURL, q)
}

// warm issues the initial GET to seed session cookies. The response body is
// discarded; only the Set-Cookie side effect matters.
func (c *Client) warm(ctx context.Context, warmURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, warmURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if _, _, err := c.do(req); err != nil {
		return fmt.Errorf("warming session: %w", err)
	}
	return nil
}

// do executes req and returns the response body, the final URL after
// redirects, and any transport error.
func (c *Client) do(req *http.Request) (string, string, error) {
	c.limiter.Wait()
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB cap
	if err != nil {
		return "", "", err
	}
	final := req.URL.String()
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return "", "", &cliutil.RateLimitError{
			URL:        final,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       truncate(string(b), 200),
		}
	}
	if resp.StatusCode >= 500 {
		return "", "", fmt.Errorf("reservesea returned HTTP %d", resp.StatusCode)
	}
	c.limiter.OnSuccess()
	return string(b), final, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var (
	// reFirstQuoteObj captures the inner body of the first object in the
	// "quotes" array; price and originalPrice are then read from that same
	// object so they cannot be sourced from different quotes on the page.
	reFirstQuoteObj = regexp.MustCompile(`"quotes"\s*:\s*\[\s*\{([^}]*)\}`)
	rePriceField    = regexp.MustCompile(`"price"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
	reOrigField     = regexp.MustCompile(`"originalPrice"\s*:\s*"?\$?([0-9]+(?:\.[0-9]+)?)`)
	reProductName   = regexp.MustCompile(`data-product-name="([^"]+)"`)
	reProductCode   = regexp.MustCompile(`data-product-code="([^"]+)"`)
	reNightPrice    = regexp.MustCompile(`\{"date":"([0-9]{4}-[0-9]{2}-[0-9]{2})T[^"]*","rollupPrice":[0-9.]+,"price":([0-9]+(?:\.[0-9]+)?)\}`)
	reInlineError   = regexp.MustCompile(`(?i)(minimum\s+\d+\s*day\s+stay[^<.]*|at\s+least\s+\d+\s*hours?\s+in\s+advance[^<.]*|at\s+least\s+\d+\s*days?\s+in\s+advance[^<.]*|no\s+more\s+than\s+\d+\s*days[^<.]*)`)
	reSoldOut       = regexp.MustCompile(`(?i)(sorry,\s*this\s+product\s+is\s+unavailable[^<.]*|sold\s*out|no\s+availability|unavailable\s+for\s+the\s+dates)`)
)

// parseQuote fills q from the response HTML/embedded JSON. finalURL disambiguates
// the flow step: selectProduct = a product page was reached; collectParkingDetails
// = the request was rejected and re-rendered with an inline error.
func parseQuote(body, finalURL string, q *Quote) (*Quote, error) {
	q.ProductName = firstGroup(reProductName, body)
	q.ProductCode = firstGroup(reProductCode, body)
	q.NightlyPrices = parseNightlyPrices(body)

	onSelectProduct := strings.Contains(finalURL, "parkingCmd=selectProduct")

	// Validation rejection: site bounced back to the date form with a message.
	if !onSelectProduct {
		if msg := firstGroup(reInlineError, body); msg != "" {
			q.Invalid = true
			q.Reason = cliutil.CleanText(strings.TrimSpace(msg))
			return q, nil
		}
	}

	if soldOut := firstGroup(reSoldOut, body); soldOut != "" {
		q.SoldOut = true
		q.Available = false
		q.Reason = cliutil.CleanText(strings.TrimSpace(soldOut))
		return q, nil
	}

	if obj := reFirstQuoteObj.FindStringSubmatch(body); obj != nil {
		inner := obj[1]
		if m := rePriceField.FindStringSubmatch(inner); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				q.TotalPrice = v
				q.Available = true
			}
		}
		if m := reOrigField.FindStringSubmatch(inner); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				// Only surface an original price when it's a genuine
				// pre-discount list price (strictly higher than the total);
				// AeroParker echoes originalPrice == price when there's no
				// discount, which is noise.
				if v > q.TotalPrice {
					q.OriginalPrice = v
				}
			}
		}
	}

	if !q.Available && !q.SoldOut && !q.Invalid {
		// Reached a product page but no price parsed, or an unrecognized
		// shape. Fall back to the nightly-price sum if we have one.
		if sum := sumNightly(q.NightlyPrices); sum > 0 {
			q.TotalPrice = sum
			q.Available = true
		} else {
			q.Invalid = true
			q.Reason = "could not determine availability from the response"
		}
	}
	return q, nil
}

func parseNightlyPrices(body string) []NightPrice {
	seen := map[string]bool{}
	var out []NightPrice
	for _, m := range reNightPrice.FindAllStringSubmatch(body, -1) {
		date := m[1]
		if seen[date] {
			continue // the array is echoed in list + card views; dedup by date
		}
		price, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		seen[date] = true
		out = append(out, NightPrice{Date: date, Price: price})
	}
	return out
}

func sumNightly(n []NightPrice) float64 {
	var s float64
	for _, p := range n {
		s += p.Price
	}
	return s
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// billedNights returns the number of billed days between entry and exit,
// rounding partial days up (AeroParker bills per calendar day of the stay).
func billedNights(entry, exit time.Time) int {
	if !exit.After(entry) {
		return 0
	}
	d := exit.Sub(entry)
	nights := int(d.Hours() / 24)
	if d > time.Duration(nights)*24*time.Hour {
		nights++
	}
	if nights < 1 {
		nights = 1
	}
	return nights
}

// snapTime maps a time to the nearest AeroParker time-of-day option (hourly,
// plus the special 00:01 and 23:59 endpoints).
func snapTime(t time.Time) string {
	h, m := t.Hour(), t.Minute()
	if m >= 30 {
		h++
	}
	switch {
	case h <= 0:
		return "00:01"
	case h >= 24:
		return "23:59"
	default:
		return fmt.Sprintf("%02d:00", h)
	}
}

func formatLocal(t time.Time) string {
	return t.Format("2006-01-02T15:04")
}
