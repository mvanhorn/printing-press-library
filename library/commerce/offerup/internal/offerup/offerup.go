// Package offerup is the hand-authored OfferUp data layer: a plain-HTTP client
// that reads OfferUp's server-rendered (Next.js) pages and extracts the listing,
// item-detail, and seller data embedded in each page's __NEXT_DATA__ script.
//
// OfferUp has no public API. Every read here is an unauthenticated GET of a
// public OfferUp web page (pp:client-call) — search results, item detail, and
// seller profiles are all anonymous. Location is set via the ou.location cookie
// (city/state/zip/lat/lon), which is a per-request preference, not a credential.
package offerup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
)

// DefaultBaseURL is OfferUp's web origin. Overridable via OFFERUP_BASE_URL so
// the verify/dogfood harness (which sets that env to a mock) and local tests
// can redirect the client without code changes.
const DefaultBaseURL = "https://offerup.com"

// chromeUA is a current desktop-Chrome User-Agent. OfferUp serves the SSR pages
// to a browser UA; a default Go UA can be throttled sooner.
const chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

var nextDataRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// schemaConditionRE pulls the schema.org item condition (e.g. UsedCondition)
// from the page markup — the authoritative human label, since __NEXT_DATA__
// stores condition only as an opaque numeric code.
var schemaConditionRE = regexp.MustCompile(`schema\.org/([A-Za-z]+)Condition`)

// Stateless replacers reused across calls (cookieValue runs per request,
// ParsePrice runs per listing).
var (
	locationCookieReplacer = strings.NewReplacer(`"`, "%22", ",", "%2C")
	priceReplacer          = strings.NewReplacer("$", "", ",", "", " ", "")
)

// Listing is one cleaned OfferUp search result.
type Listing struct {
	ListingID     string   `json:"listingId"`
	Title         string   `json:"title"`
	Price         float64  `json:"price"`
	PriceText     string   `json:"priceText"`
	LocationName  string   `json:"locationName"`
	ConditionText string   `json:"conditionText,omitempty"`
	IsFirmPrice   bool     `json:"isFirmPrice"`
	VehicleMiles  string   `json:"vehicleMiles,omitempty"`
	Flags         []string `json:"flags,omitempty"`
	ImageURL      string   `json:"imageUrl,omitempty"`
	URL           string   `json:"url"`
}

// Seller is a cleaned OfferUp user/seller profile.
type Seller struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DateJoined        string `json:"dateJoined,omitempty"`
	PrimaryBadge      string `json:"primaryBadge,omitempty"`
	IsBusinessAccount bool   `json:"isBusinessAccount"`
	IsAutosDealer     bool   `json:"isAutosDealer"`
	IsPremium         bool   `json:"isPremium"`
	IsTruyouVerified  bool   `json:"isTruyouVerified"`
}

// ListingDetail is one item's full detail page.
type ListingDetail struct {
	Listing
	Description   string   `json:"description,omitempty"`
	OriginalPrice float64  `json:"originalPrice,omitempty"`
	OwnerID       string   `json:"ownerId,omitempty"`
	CategoryID    string   `json:"categoryId,omitempty"`
	Photos        []string `json:"photos,omitempty"`
	Seller        *Seller  `json:"seller,omitempty"`
}

// Location is a search location preference written into the ou.location cookie.
// Lat/Lon drive OfferUp's geo most reliably; Zip/City/State are labels OfferUp
// may also resolve. An empty Location leaves the request on OfferUp's IP geo.
type Location struct {
	Zip   string
	City  string
	State string
	Lat   string
	Lon   string
}

func (l *Location) empty() bool {
	return l == nil || (l.Zip == "" && l.City == "" && l.State == "" && l.Lat == "" && l.Lon == "")
}

// cookieValue renders the ou.location cookie value exactly as OfferUp's web app
// stores it: JSON with quotes percent-encoded as %22 and commas as %2C.
func (l *Location) cookieValue() string {
	m := map[string]any{"source": "manual"}
	if l.City != "" {
		m["city"] = l.City
	}
	if l.State != "" {
		m["state"] = l.State
	}
	if l.Zip != "" {
		m["zipCode"] = l.Zip
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(l.Lat), 64); err == nil {
		m["latitude"] = f
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(l.Lon), 64); err == nil {
		m["longitude"] = f
	}
	b, _ := json.Marshal(m)
	return locationCookieReplacer.Replace(string(b))
}

// SearchOptions narrows a search. Zero values mean "no filter".
type SearchOptions struct {
	Category  string // OfferUp category id (cid), e.g. "1" or "1.2"
	Location  *Location
	Limit     int     // cap returned listings; <=0 returns all on the page
	PriceMin  float64 // client-side filter (OfferUp's SSR page is not reliably price-filtered by query string)
	PriceMax  float64
	FirmOnly  bool
	LocalOnly bool // only LOCAL_PICKUP listings
}

// Client reads OfferUp's public web pages.
type Client struct {
	http    *http.Client
	baseURL string
	limiter *cliutil.AdaptiveLimiter
}

// NewClient returns a Client. ratePerSec<=0 disables pacing; the OfferUp default
// of 2 rps is conservative (OfferUp throttles rapid bursts).
func NewClient(timeout time.Duration, ratePerSec float64) *Client {
	base := strings.TrimRight(os.Getenv("OFFERUP_BASE_URL"), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		http:    &http.Client{Timeout: timeout},
		baseURL: base,
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// BaseURL returns the resolved origin (for building item URLs).
func (c *Client) BaseURL() string { return c.baseURL }

// fetchNextData GETs path?query with the location cookie and returns the parsed
// __NEXT_DATA__ JSON object plus the raw page body. It retries once on HTTP
// 429, then surfaces a *cliutil.RateLimitError so callers never mistake
// throttling for "no results".
func (c *Client) fetchNextData(ctx context.Context, path string, q url.Values, loc *Location) (map[string]any, []byte, error) {
	u := c.baseURL + path
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	for attempt := 0; attempt < 2; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", chromeUA)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		if !loc.empty() {
			req.Header.Set("Cookie", "ou.location="+loc.cookieValue())
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching %s: %w", u, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			if attempt == 0 {
				wait := cliutil.RetryAfter(resp)
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			return nil, nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp), Body: snippet(body)}
		}
		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("OfferUp returned HTTP %d for %s: %s", resp.StatusCode, u, snippet(body))
		}
		c.limiter.OnSuccess()
		m := nextDataRE.FindSubmatch(body)
		if m == nil {
			return nil, nil, fmt.Errorf("no __NEXT_DATA__ found at %s (page shape may have changed)", u)
		}
		var data map[string]any
		if err := json.Unmarshal(m[1], &data); err != nil {
			return nil, nil, fmt.Errorf("parsing __NEXT_DATA__ at %s: %w", u, err)
		}
		return data, body, nil
	}
	return nil, nil, fmt.Errorf("unreachable: retry loop exited for %s", u)
}

// Search returns cleaned listings for a keyword query.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]Listing, error) {
	q := url.Values{}
	q.Set("q", query)
	if opts.Category != "" {
		q.Set("cid", opts.Category)
	}
	data, _, err := c.fetchNextData(ctx, "/search", q, opts.Location)
	if err != nil {
		return nil, err
	}
	tiles, _ := dig(data, "props", "pageProps", "searchFeedResponse", "looseTiles").([]any)
	listings := make([]Listing, 0, len(tiles))
	for _, t := range tiles {
		tile, ok := t.(map[string]any)
		if !ok || str(tile["tileType"]) != "LISTING" {
			continue
		}
		raw, ok := tile["listing"].(map[string]any)
		if !ok {
			continue
		}
		l := listingFromMap(raw, c.baseURL)
		if !opts.match(l) {
			continue
		}
		listings = append(listings, l)
		if opts.Limit > 0 && len(listings) >= opts.Limit {
			break
		}
	}
	return listings, nil
}

func (o SearchOptions) match(l Listing) bool {
	if o.PriceMin > 0 && l.Price < o.PriceMin {
		return false
	}
	if o.PriceMax > 0 && l.Price > o.PriceMax {
		return false
	}
	if o.FirmOnly && !l.IsFirmPrice {
		return false
	}
	if o.LocalOnly && !hasFlag(l.Flags, "LOCAL_PICKUP") {
		return false
	}
	return true
}

// GetItem returns one listing's full detail by id.
func (c *Client) GetItem(ctx context.Context, listingID string) (*ListingDetail, error) {
	data, body, err := c.fetchNextData(ctx, "/item/detail/"+url.PathEscape(listingID), nil, nil)
	if err != nil {
		return nil, err
	}
	apollo, _ := dig(data, "props", "pageProps", "initialApolloState").(map[string]any)
	if apollo == nil {
		return nil, fmt.Errorf("no listing data for %s", listingID)
	}
	root, _ := apollo["ROOT_QUERY"].(map[string]any)
	var raw map[string]any
	for k, v := range root {
		if strings.HasPrefix(k, "listing(") {
			raw, _ = v.(map[string]any)
			break
		}
	}
	if raw == nil {
		return nil, fmt.Errorf("listing %s not found (it may have been removed)", listingID)
	}
	d := &ListingDetail{Listing: listingFromMap(raw, c.baseURL)}
	if d.ListingID == "" {
		d.ListingID = listingID
		d.URL = c.baseURL + "/item/detail/" + listingID
	}
	d.Description = str(raw["description"])
	d.OriginalPrice = toPrice(raw["originalPrice"])
	d.CategoryID = str(raw["listingCategory"])
	d.OwnerID = str(raw["ownerId"])
	if cond := conditionText(raw["condition"]); cond != "" {
		d.ConditionText = cond
	}
	if d.ConditionText == "" {
		// OfferUp stores condition as a numeric code in __NEXT_DATA__; the
		// human label lives in the page's schema.org markup.
		d.ConditionText = schemaConditionLabel(body)
	}
	if loc, ok := raw["locationDetails"].(map[string]any); ok {
		if name := str(loc["locationName"]); name != "" {
			d.LocationName = name
		}
	}
	d.Photos = extractPhotoURLs(raw["photos"], apollo)
	if d.OwnerID != "" {
		if s := sellerFromApollo(apollo, d.OwnerID); s != nil {
			d.Seller = s
		}
	}
	return d, nil
}

// GetSeller resolves a seller profile by id by reading any item the seller has
// listed. OfferUp has no standalone public seller-profile page that embeds the
// full User record, so the detail page (which carries the User entity) is the
// source. listingID is a known listing owned by the seller.
func (c *Client) GetSeller(ctx context.Context, listingID string) (*Seller, error) {
	d, err := c.GetItem(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if d.Seller == nil {
		return nil, fmt.Errorf("no seller data on listing %s", listingID)
	}
	return d.Seller, nil
}

func listingFromMap(raw map[string]any, baseURL string) Listing {
	l := Listing{
		ListingID:     str(raw["listingId"]),
		Title:         cliutil.CleanText(str(raw["title"])),
		PriceText:     str(raw["price"]),
		Price:         toPrice(raw["price"]),
		LocationName:  str(raw["locationName"]),
		ConditionText: str(raw["conditionText"]),
		VehicleMiles:  str(raw["vehicleMiles"]),
		IsFirmPrice:   boolish(raw["isFirmPrice"]),
		Flags:         strSlice(raw["flags"]),
	}
	if img, ok := raw["image"].(map[string]any); ok {
		l.ImageURL = str(img["url"])
	}
	if l.ListingID != "" {
		l.URL = strings.TrimRight(baseURL, "/") + "/item/detail/" + l.ListingID
	}
	return l
}

func sellerFromApollo(apollo map[string]any, ownerID string) *Seller {
	user, _ := apollo["User:"+ownerID].(map[string]any)
	if user == nil {
		// Fall back to the first User entity present.
		for k, v := range apollo {
			if strings.HasPrefix(k, "User:") {
				user, _ = v.(map[string]any)
				break
			}
		}
	}
	if user == nil {
		return nil
	}
	s := &Seller{ID: ownerID}
	if s.ID == "" {
		s.ID = strings.TrimPrefix(str(user["id"]), "User:")
	}
	prof, _ := user["profile"].(map[string]any)
	if prof == nil {
		return s
	}
	s.Name = cliutil.CleanText(str(prof["name"]))
	s.DateJoined = str(prof["dateJoined"])
	s.IsBusinessAccount = boolish(prof["isBusinessAccount"])
	s.IsAutosDealer = boolish(prof["isAutosDealer"])
	s.IsPremium = boolish(prof["isPremium"])
	s.IsTruyouVerified = boolish(prof["isTruyouVerified"])
	if badges, ok := prof["avatarBadges"].(map[string]any); ok {
		s.PrimaryBadge = str(badges["primaryBadge"])
	}
	return s
}

func extractPhotoURLs(v any, apollo map[string]any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var urls []string
	for _, p := range arr {
		switch pv := p.(type) {
		case map[string]any:
			if ref := str(pv["__ref"]); ref != "" {
				if ent, ok := apollo[ref].(map[string]any); ok {
					if u := photoURL(ent); u != "" {
						urls = append(urls, u)
					}
				}
				continue
			}
			if u := photoURL(pv); u != "" {
				urls = append(urls, u)
			}
		case string:
			if pv != "" {
				urls = append(urls, pv)
			}
		}
	}
	return urls
}

func photoURL(m map[string]any) string {
	for _, k := range []string{"detail", "url", "image", "square"} {
		if u := str(m[k]); strings.HasPrefix(u, "http") {
			return u
		}
		if sub, ok := m[k].(map[string]any); ok {
			if u := str(sub["url"]); strings.HasPrefix(u, "http") {
				return u
			}
		}
	}
	return ""
}

// --- small JSON helpers ---

// dig walks nested map[string]any by key, returning nil on any miss.
func dig(m any, keys ...string) any {
	cur := m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolish(v any) bool {
	b, _ := v.(bool)
	return b
}

func strSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func conditionText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case map[string]any:
		for _, k := range []string{"conditionText", "text", "label", "name"} {
			if s := str(c[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

// ParsePrice converts an OfferUp price string ("40", "1,200", "$95") to a
// float. Empty or non-numeric returns 0.
func ParsePrice(s string) float64 {
	s = strings.TrimSpace(priceReplacer.Replace(s))
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func toPrice(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		return ParsePrice(n)
	}
	return 0
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// schemaConditionLabel extracts a human condition label (e.g. "Used", "New",
// "Refurbished") from the page's schema.org itemCondition markup, or "" when
// absent.
func schemaConditionLabel(body []byte) string {
	m := schemaConditionRE.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
