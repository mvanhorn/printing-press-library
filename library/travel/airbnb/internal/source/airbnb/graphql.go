package airbnb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/auth"
)

// buildCookieHeader serializes a slice of cookies into a Cookie header value.
// Returns empty string when there are no cookies, so the caller can decide
// not to set the header at all rather than send a blank one.
func buildCookieHeader(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

const (
	wishlistIndexHash = "b8b421d802c399b55fb6ac1111014807a454184ad38f198365beb7836c018c18"
	wishlistItemsHash = "c0f9d9474bb20eb7af2f94f8e022750a5ed9b7437613e1d9aa91aadea87e4467"
	bookItHash        = "5560c774d764520fc721f6dffca10d9cff03b25e9907478ded8530caf679d716"
	// Public web client API key embedded on every airbnb.com SSR page in
	// `"api_config":{"key":"..."}`. This is the well-known constant every
	// public scraper uses (see HAR captures under
	// .manuscripts/.../discovery/airbnb/airbnb-capture.har). The scrape
	// helper below tries to read a fresh value at process startup, but
	// falls back to this constant if the scrape fails.
	airbnbDefaultAPIKey = "d306zoyjsyarp7ifhu67rjxn52tv0t20"
	apiKeyScrapeTimeout = 30 * time.Second
)

// apiKeyRe finds the public web key in airbnb.com SSR HTML.
var apiKeyRe = regexp.MustCompile(`"api_config"\s*:\s*\{\s*"key"\s*:\s*"([a-z0-9]{20,})"`)

var (
	apiKeyOnce sync.Once
	apiKeyVal  string
)

// PATCH: Resolve Airbnb's rotating public GraphQL key from fresh SSR HTML.
// resolveAPIKey returns the current Airbnb web public API key. It scrapes
// the homepage SSR HTML on first use (cached for the process lifetime),
// and falls back to the well-known constant if the scrape fails.
func (c *Client) resolveAPIKey() string {
	apiKeyOnce.Do(func() {
		apiKeyVal = airbnbDefaultAPIKey
		scrapeCtx, cancel := context.WithTimeout(context.Background(), apiKeyScrapeTimeout)
		defer cancel()
		body, err := c.do(scrapeCtx, "GET", airbnbBase+"/", airbnbUA, nil, nil)
		if err != nil {
			return
		}
		if m := apiKeyRe.FindSubmatch(body); len(m) >= 2 {
			apiKeyVal = string(m[1])
		}
	})
	return apiKeyVal
}

// parseAPIKey is the pure regex extractor, exposed for unit tests.
func parseAPIKey(body []byte) string {
	if m := apiKeyRe.FindSubmatch(body); len(m) >= 2 {
		return string(m[1])
	}
	return ""
}

func WishlistList(ctx context.Context) ([]Wishlist, error) {
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/WishlistIndexPageQuery/"+wishlistIndexHash, nil, &root); err != nil {
		return nil, err
	}
	items := findObjects(root, []string{"wishlists", "wishlist"})
	out := make([]Wishlist, 0, len(items))
	for _, m := range items {
		id := str(m["id"])
		name := firstStringByKeys(m, "name", "title")
		if id == "" && name == "" {
			continue
		}
		out = append(out, Wishlist{ID: id, Name: name, Count: int(num(firstByKey(m, "count"))), Raw: m})
	}
	return out, nil
}

func WishlistItems(ctx context.Context, listingIDs []string) ([]WishlistItem, error) {
	params := url.Values{}
	if len(listingIDs) > 0 {
		params.Set("listing_ids", strings.Join(listingIDs, ","))
	}
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/WishlistItemsAsyncQuery/"+wishlistItemsHash, params, &root); err != nil {
		return nil, err
	}
	objects := findObjects(root, []string{"listingId", "listing_id", "id"})
	out := make([]WishlistItem, 0, len(objects))
	for _, m := range objects {
		id := firstStringByKeys(m, "listingId", "listing_id", "id")
		if id == "" {
			continue
		}
		out = append(out, WishlistItem{ListingID: id, WishlistID: firstStringByKeys(m, "wishlistId", "wishlist_id"), Title: firstStringByKeys(m, "title", "name"), Raw: m})
	}
	return out, nil
}

// PATCH: BookingPrice variables shape rewritten from flat
// {id, checkin, checkout, adults} to the schema-correct
// {id, dateRange, guestCounts, 4 includeFragment booleans} form.
func BookingPrice(ctx context.Context, listingID, checkin, checkout string, guests int) (*PriceBreakdown, error) {
	// Variables shape derived from a real captured browser request in
	// .manuscripts/20260502-210359/discovery/airbnb/airbnb-capture.har.
	// The earlier flat shape {id, checkin, checkout, adults} tripped a
	// ValidationError because StaysPdpBookItQuery expects dateRange +
	// guestCounts objects plus four includeFragment booleans.
	variables := map[string]any{
		"id": RelayListingID(listingID),
		"dateRange": map[string]any{
			"startDate": checkin,
			"endDate":   checkout,
		},
		"guestCounts": map[string]any{
			"numberOfAdults": guests,
		},
		// The four include*Fragment booleans gate which sub-fragments of the
		// query response are returned. The captured HAR had them all false
		// (skeleton-load path), but in that mode the response contains only
		// {data: {node: {__typename}}}. Set all true to receive the full
		// pricing/booking payload that downstream priceBreakdownFromAny parses.
		"includePdpMigrationBookItCalendarSheetFragment":  true,
		"includePdpMigrationBookItFloatingFooterFragment": true,
		"includePdpMigrationBookItNavFragment":            true,
		"includeOverviewMerchandisingTipsFragment":        true,
	}
	params := url.Values{}
	b, _ := json.Marshal(variables)
	params.Set("variables", string(b))
	var root any
	if err := defaultClient.graphQLGet(ctx, "/api/v3/StaysPdpBookItQuery/"+bookItHash, params, &root); err != nil {
		return nil, err
	}
	return priceBreakdownFromAny(root), nil
}

// PATCH: graphQLGet sets Cookie header directly per-request instead of
// swapping c.http.Jar, eliminating the data race on the shared client
// field when multiple goroutines call graphQLGet concurrently.
func (c *Client) graphQLGet(ctx context.Context, path string, params url.Values, out *any) error {
	u, _ := url.Parse(airbnbBase + path)
	q := u.Query()
	for k, vals := range params {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	q.Set("extensions", `{"persistedQuery":{"version":1,"sha256Hash":"`+path[strings.LastIndex(path, "/")+1:]+`"}}`)
	u.RawQuery = q.Encode()
	apiKey := c.resolveAPIKey()
	cookies, err := auth.LoadCookies()
	if err != nil {
		return err
	}
	// PATCH: Send Airbnb's public web GraphQL key and browser companion headers.
	// The Airbnb GraphQL gateway rejects requests without an api key with
	// {error:"invalid_key", error_code:400}. Send the public web key plus
	// the companion headers every real-world request carries (per the HAR
	// in .manuscripts/.../discovery/airbnb/airbnb-capture.har) — without
	// them, Airbnb's heuristics flag the call as non-browser.
	headers := map[string]string{
		"Accept":                           "application/json",
		"X-Airbnb-API-Key":                 apiKey,
		"X-Airbnb-GraphQL-Platform":        "web",
		"X-Airbnb-GraphQL-Platform-Client": "minimalist-niobe",
		"X-Airbnb-Supports-Airlock-V2":     "true",
	}
	if cookieHeader := buildCookieHeader(cookies); cookieHeader != "" {
		headers["Cookie"] = cookieHeader
	}
	data, err := c.do(ctx, "GET", u.String(), airbnbUA, nil, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse Airbnb GraphQL response: %w", err)
	}
	return nil
}

// PATCH: priceBreakdownFromAny gains a structuredDisplayPrice walker.
func priceBreakdownFromAny(root any) *PriceBreakdown {
	p := &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}, Raw: root}

	// New schema (post-PdpMigrationBookIt): the price lives under
	// pdpPresentation.bookIt.structuredDisplayPrice as a primaryLine.price
	// string ("$514") plus explanationData.priceDetails[].items[] objects
	// with description + priceString pairs. The walker below picks up the
	// structuredDisplayPrice objects no matter how deep they sit in the
	// response, so the same logic handles both the listing-page payload
	// and any future surface that returns the same shape.
	for _, sdp := range findObjects(root, []string{"structuredDisplayPrice"}) {
		inner, _ := sdp["structuredDisplayPrice"].(map[string]any)
		if inner == nil {
			continue
		}
		if line, _ := inner["primaryLine"].(map[string]any); line != nil {
			if total := amountFromText(firstStringByKeys(line, "price", "accessibilityLabel")); total > 0 && p.Total == 0 {
				p.Total = total
			}
		}
		exp, _ := inner["explanationData"].(map[string]any)
		groups, _ := exp["priceDetails"].([]any)
		for _, g := range groups {
			gm, _ := g.(map[string]any)
			items, _ := gm["items"].([]any)
			for _, it := range items {
				m, _ := it.(map[string]any)
				if m == nil {
					continue
				}
				desc := strings.ToLower(firstStringByKeys(m, "description", "title"))
				amt := amountFromText(firstStringByKeys(m, "priceString", "price", "formattedAmount"))
				switch {
				case strings.Contains(desc, "clean"):
					p.Fees["cleaning"] += amt
				case strings.Contains(desc, "service"):
					p.Fees["service"] += amt
				case strings.Contains(desc, "tax"):
					p.Fees["tax"] += amt
				case strings.Contains(desc, "subtotal"):
					p.Subtotal = amt
				case strings.Contains(desc, "total"):
					if amt > 0 && p.Total == 0 {
						p.Total = amt
					}
				case strings.Contains(desc, "x $"), strings.Contains(desc, "night"):
					// "3 nights x $171.18" or similar per-night lines.
					// The line-item priceString is the multi-night subtotal,
					// not the per-night rate, so use it as Subtotal when one
					// is not already set; PerNight derivation comes from the
					// "$/night" parse inside the description string.
					if p.Subtotal == 0 {
						p.Subtotal = amt
					}
					if p.PerNight == 0 {
						if idx := strings.Index(desc, "x $"); idx >= 0 {
							p.PerNight = amountFromText(desc[idx+2:])
						}
					}
				}
			}
		}
	}

	// Legacy schema fallback: older endpoints (and possibly future ones)
	// return {label, amount} fee objects. Keep walking the old way so this
	// function stays robust when the response carries the legacy shape. If
	// structuredDisplayPrice already populated fees, skip this pass so a
	// migration response containing both shapes does not double-count fees.
	if len(p.Fees) == 0 {
		for _, obj := range findObjects(root, []string{"label", "amount"}) {
			label := strings.ToLower(firstStringByKeys(obj, "label", "title", "feeType"))
			amount := num(firstByKey(obj, "amount"))
			if amount == 0 {
				amount = amountFromText(firstStringByKeys(obj, "price", "formattedAmount"))
			}
			switch {
			case strings.Contains(label, "clean"):
				p.Fees["cleaning"] += amount
			case strings.Contains(label, "service"):
				p.Fees["service"] += amount
			case strings.Contains(label, "tax"):
				p.Fees["tax"] += amount
			case strings.Contains(label, "subtotal"):
				if p.Subtotal == 0 {
					p.Subtotal = amount
				}
			case strings.Contains(label, "total"):
				if p.Total == 0 {
					p.Total = amount
				}
			}
		}
	}
	if p.Total == 0 {
		p.Total = amountFromText(firstStringByKeys(root, "total", "totalPrice", "localizedTotalPrice"))
	}
	if p.PerNight == 0 {
		p.PerNight = amountFromText(firstStringByKeys(root, "perNight", "perNightPrice"))
	}
	return p
}

func findObjects(root any, keys []string) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, k := range keys {
				if _, ok := x[k]; ok {
					out = append(out, x)
					break
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

func graphQLBody(operation, hash string, variables map[string]any) *bytes.Buffer {
	body := map[string]any{
		"operationName": operation,
		"variables":     variables,
		"extensions": map[string]any{"persistedQuery": map[string]any{
			"version": 1, "sha256Hash": hash,
		}},
	}
	b, _ := json.Marshal(body)
	return bytes.NewBuffer(b)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

var _ = http.MethodGet
