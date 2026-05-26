package airbnb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
)

// PATCH: jitter helper for the retry sleep (25% spread on top of base).
//
// jitter returns a random duration in [0, base/4). Used to spread retry
// sleeps across the fleet so a thundering herd does not synchronize.
// Returns 0 when base is too small for a useful jitter window.
func jitter(base time.Duration) time.Duration {
	if base < 4*time.Nanosecond {
		return 0
	}
	return time.Duration(rand.Int63n(int64(base / 4)))
}

const (
	airbnbBase = "https://www.airbnb.com"
	airbnbUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	geoUA      = "airbnb-pp-cli/0.1.0 (+https://github.com/mvanhorn/airbnb-pp)"
)

var defaultClient = &Client{
	http:    &http.Client{Timeout: 30 * time.Second},
	limiter: cliutil.NewAdaptiveLimiter(0.5),
	robots:  map[string]bool{},
	sleep:   time.Sleep,
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	mu      sync.Mutex
	robots  map[string]bool
	sleep   func(time.Duration)
}

func Search(ctx context.Context, params SearchParams) ([]Listing, *Pagination, error) {
	return defaultClient.Search(ctx, params)
}

func Get(ctx context.Context, listingID string, params GetParams) (*Listing, error) {
	return defaultClient.Get(ctx, listingID, params)
}

func Geocode(ctx context.Context, location string) (*Bbox, error) {
	return defaultClient.Geocode(ctx, location)
}

// PATCH: airbnb.SetRate pass-through to cliutil.AdaptiveLimiter.SetRate.
//
// SetRate adjusts the rate-limit cap on the package-global default client.
// rps <= 0 disables rate limiting. Any positive value sets the new cap and
// resets the adaptive-ramp state. Safe to call concurrently with in-flight
// requests; the limiter instance is preserved across the rate change so the
// OnRateLimit / OnSuccess history is not lost. Intended to be wired from
// rootCmd.PersistentPreRunE when the user explicitly passes --rate-limit;
// when the flag is unset, the existing 0.5 rps baseline is kept.
func SetRate(rps float64) {
	defaultClient.limiter.SetRate(rps)
}

func (c *Client) Search(ctx context.Context, params SearchParams) ([]Listing, *Pagination, error) {
	slug := params.Slug
	if slug == "" {
		slug = params.Location
	}
	if slug == "" {
		return nil, nil, fmt.Errorf("location or slug is required")
	}
	path := "/s/" + url.PathEscape(slug) + "/homes"
	u, _ := url.Parse(airbnbBase + path)
	q := u.Query()
	set(q, "checkin", params.Checkin)
	set(q, "checkout", params.Checkout)
	setInt(q, "adults", params.Adults)
	setInt(q, "children", params.Children)
	setInt(q, "infants", params.Infants)
	setInt(q, "pets", params.Pets)
	setInt(q, "price_min", params.MinPrice)
	setInt(q, "price_max", params.MaxPrice)
	set(q, "cursor", params.Cursor)
	for _, rt := range params.RoomTypes {
		if rt != "" {
			q.Add("room_types[]", rt)
		}
	}
	u.RawQuery = q.Encode()
	var root any
	if err := c.fetchDeferredJSON(ctx, u.String(), path, &root); err != nil {
		return nil, nil, err
	}
	resultsAny := firstByKey(root, "searchResults")
	arr, _ := resultsAny.([]any)
	listings := make([]Listing, 0, len(arr))
	for _, item := range arr {
		obj, _ := item.(map[string]any)
		lmap := asMap(obj["listing"])
		if len(lmap) == 0 {
			lmap = obj
		} else {
			merged := make(map[string]any, len(lmap)+8)
			for k, v := range lmap {
				merged[k] = v
			}
			for _, key := range []string{"id", "listingId", "roomId", "encodedId", "listingUrl", "pdpUrl", "demandStayListing"} {
				if merged[key] == nil {
					merged[key] = obj[key]
				}
			}
			lmap = merged
		}
		// PATCH: Treat flat Airbnb SSR search-result cards as their own price quote.
		// Airbnb's current SSR Apollo cache returns search-result entries
		// in a flat shape where the listing card carries `structuredDisplayPrice`
		// directly (no `pricingQuote` envelope). Fall back to `obj` itself so
		// the price extractor can find it.
		priceQuote := asMap(obj["pricingQuote"])
		if len(priceQuote) == 0 {
			priceQuote = obj
		}
		l := listingFromSearch(lmap, priceQuote)
		if l.ID != "" {
			l.URL = airbnbBase + "/rooms/" + l.ID
		}
		listings = append(listings, l)
	}
	p := &Pagination{}
	if cursors, ok := firstByKey(root, "pageCursors").([]any); ok {
		for _, c := range cursors {
			p.Cursors = append(p.Cursors, str(c))
		}
		if len(p.Cursors) > 0 {
			p.Next = p.Cursors[len(p.Cursors)-1]
		}
	}
	return listings, p, nil
}

func (c *Client) Get(ctx context.Context, listingID string, params GetParams) (*Listing, error) {
	if listingID == "" {
		return nil, fmt.Errorf("listing id is required")
	}
	path := "/rooms/" + url.PathEscape(listingID)
	u, _ := url.Parse(airbnbBase + path)
	q := u.Query()
	set(q, "checkin", params.Checkin)
	set(q, "checkout", params.Checkout)
	setInt(q, "adults", params.Adults)
	u.RawQuery = q.Encode()
	var root any
	if err := c.fetchDeferredJSON(ctx, u.String(), path, &root); err != nil {
		return nil, err
	}
	l := listingFromPDPSections(root, listingID)
	if photos := collectURLs(firstByKey(root, "photos")); len(photos) > 0 {
		l.Photos = photos
	}
	enrichCounts(l, root)
	if l.PriceBreakdown == nil && params.Checkin != "" && params.Checkout != "" {
		if pb, err := BookingPrice(ctx, listingID, params.Checkin, params.Checkout, params.Adults); err == nil && pb != nil {
			applyPriceBreakdown(l, pb)
		}
	}
	return l, nil
}

func (c *Client) fetchDeferredJSON(ctx context.Context, target, path string, out *any) error {
	if err := c.allowedByRobots(ctx, path); err != nil {
		return err
	}
	body, err := c.do(ctx, "GET", target, airbnbUA, nil, nil)
	if err != nil {
		return err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("parse html: %w", err)
	}
	script := doc.Find("#data-deferred-state-0").First().Text()
	if strings.TrimSpace(script) == "" {
		return ErrNotFound
	}
	var root any
	if err := json.Unmarshal([]byte(script), &root); err != nil {
		return fmt.Errorf("parse deferred state: %w", err)
	}
	data := firstNiobeData(root)
	if data == nil {
		data = root
	}
	*out = data
	return nil
}

func (c *Client) do(ctx context.Context, method, target, ua string, body io.Reader, headers map[string]string) ([]byte, error) {
	const retries = 3
	var last []byte
	for attempt := 0; attempt <= retries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		last = data
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
			if attempt == retries {
				if challenge, ok := isBotChallenge(resp, data); ok {
					challenge.URL = target
					return nil, &challenge
				}
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp), Body: string(last)}
			}
			// PATCH: server-stated Retry-After + Backoff fallback + jitter.
			// Prefer the server-stated Retry-After header when present;
			// fall back to exponential Backoff(attempt) when absent so
			// subsequent retries actually grow instead of re-sleeping
			// the same 5s default. 25% jitter on top of the base wait
			// prevents a fleet of retrying clients from synchronizing.
			var base time.Duration
			if resp.Header.Get("Retry-After") != "" {
				base = cliutil.RetryAfter(resp)
			} else {
				base = cliutil.Backoff(attempt)
			}
			if c.sleep != nil {
				c.sleep(base + jitter(base))
			} else {
				time.Sleep(base + jitter(base))
			}
			continue
		}
		// PATCH: bot-challenge branch added before the generic >=400 fallthrough.
		if challenge, ok := isBotChallenge(resp, data); ok {
			// Treat bot challenges like 429 for adaptive-rate-cut purposes
			// (the limiter halves its rate and records the ceiling), but do
			// NOT retry — bot challenges typically require cookie refresh or
			// a longer cool-off than the retry window supports.
			c.limiter.OnRateLimit()
			challenge.URL = target
			return nil, &challenge
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("GET %s returned HTTP %d: %s", target, resp.StatusCode, truncate(string(data)))
		}
		c.limiter.OnSuccess()
		return data, nil
	}
	return nil, fmt.Errorf("request failed: %s", truncate(string(last)))
}

// PATCH: bot-challenge detector — gates cookie/header signatures on 4xx
// to avoid false positives on legit 200 responses passing through datadome.
//
// isBotChallenge inspects an HTTP response for datadome or Akamai/Kona
// bot-defense signatures and returns a typed BotChallengeError describing
// the challenge type and a remediation hint. Negative returns mean "looks
// like a regular response, not a challenge" and let the caller fall through
// to its existing 4xx/2xx branches.
//
// Datadome sets `Set-Cookie: datadome=...` and `Server: dd-*` headers on
// EVERY response from a Datadome-protected origin, including allowed-through
// 200s (the cookie is Datadome's session token, not a challenge marker).
// The cookie and Server-header checks therefore fire only on 4xx; only the
// challenge-page-specific body markers (captcha-delivery, "bot or not",
// captcha-pwa) are safe to check unconditionally.
func isBotChallenge(resp *http.Response, body []byte) (cliutil.BotChallengeError, bool) {
	if resp == nil {
		return cliutil.BotChallengeError{}, false
	}
	// Datadome cookie / Server-header signatures: gated on 4xx so a normal
	// 200 response carrying Datadome's session cookie does not falsely
	// trigger BotChallengeError.
	if resp.StatusCode >= 400 {
		for _, c := range resp.Cookies() {
			if strings.EqualFold(c.Name, "datadome") {
				return cliutil.BotChallengeError{
					ChallengeType:   "datadome",
					StatusCode:      resp.StatusCode,
					Remediation:     "wait and retry after a cool-off, or refresh cookies via 'airbnb-pp-cli auth login --chrome'",
					ResponseSnippet: truncate(string(body)),
				}, true
			}
		}
		if server := resp.Header.Get("Server"); strings.HasPrefix(strings.ToLower(server), "dd-") {
			return cliutil.BotChallengeError{
				ChallengeType:   "datadome",
				StatusCode:      resp.StatusCode,
				Remediation:     "wait and retry after a cool-off, or refresh cookies via 'airbnb-pp-cli auth login --chrome'",
				ResponseSnippet: truncate(string(body)),
			}, true
		}
	}
	// Body-marker signatures are challenge-page-specific (CAPTCHA redirect
	// URLs, Akamai's "Bot or Not" title, the captcha-pwa script reference).
	// These are safe to check on any status code; they cannot appear in a
	// legitimate JSON / HTML body that returned actual content.
	lowerBody := strings.ToLower(string(body))
	if strings.Contains(lowerBody, "geo.captcha-delivery.com") {
		return cliutil.BotChallengeError{
			ChallengeType:   "datadome",
			StatusCode:      resp.StatusCode,
			Remediation:     "wait and retry after a cool-off, or refresh cookies via 'airbnb-pp-cli auth login --chrome'",
			ResponseSnippet: truncate(string(body)),
		}, true
	}
	// Akamai / Kona signatures: title contains "bot or not" (Akamai's stock
	// challenge page), or the body references the captcha-pwa script that
	// Akamai's challenge ships. Matches the VRBO detector pattern.
	if strings.Contains(lowerBody, "<title>bot or not") {
		return cliutil.BotChallengeError{
			ChallengeType:   "akamai",
			StatusCode:      resp.StatusCode,
			Remediation:     "Akamai challenge; wait at least 25 minutes for sensor cooldown, then retry with fresh cookies",
			ResponseSnippet: truncate(string(body)),
		}, true
	}
	if strings.Contains(lowerBody, "captcha-pwa") {
		return cliutil.BotChallengeError{
			ChallengeType:   "akamai",
			StatusCode:      resp.StatusCode,
			Remediation:     "Akamai challenge; wait at least 25 minutes for sensor cooldown, then retry with fresh cookies",
			ResponseSnippet: truncate(string(body)),
		}, true
	}
	return cliutil.BotChallengeError{}, false
}

func (c *Client) allowedByRobots(ctx context.Context, path string) error {
	if strings.EqualFold(os.Getenv("AIRBNB_PP_IGNORE_ROBOTS_TXT"), "true") || os.Getenv("AIRBNB_PP_IGNORE_ROBOTS_TXT") == "1" {
		return nil
	}
	c.mu.Lock()
	if allowed, ok := c.robots[path]; ok {
		c.mu.Unlock()
		if !allowed {
			return fmt.Errorf("blocked by Airbnb robots.txt for %s; set AIRBNB_PP_IGNORE_ROBOTS_TXT=true to override", path)
		}
		return nil
	}
	c.mu.Unlock()
	data, err := c.do(ctx, "GET", airbnbBase+"/robots.txt", airbnbUA, nil, nil)
	if err != nil {
		return nil
	}
	allowed := robotsAllows(string(data), path)
	c.mu.Lock()
	c.robots[path] = allowed
	c.mu.Unlock()
	if !allowed {
		return fmt.Errorf("blocked by Airbnb robots.txt for %s; set AIRBNB_PP_IGNORE_ROBOTS_TXT=true to override", path)
	}
	return nil
}

func (c *Client) Geocode(ctx context.Context, location string) (*Bbox, error) {
	if location == "" {
		return nil, fmt.Errorf("location is required")
	}
	if box, err := c.photon(ctx, location); err == nil {
		return box, nil
	}
	return c.nominatim(ctx, location)
}

func (c *Client) photon(ctx context.Context, location string) (*Bbox, error) {
	u := "https://photon.komoot.io/api/?limit=5&q=" + url.QueryEscape(location)
	data, err := c.do(ctx, "GET", u, geoUA, nil, nil)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	features, _ := root["features"].([]any)
	for _, f := range features {
		props := asMap(asMap(f)["properties"])
		extent, _ := props["extent"].([]any)
		if len(extent) == 4 {
			return &Bbox{SWLng: num(extent[0]), NELat: num(extent[1]), NELng: num(extent[2]), SWLat: num(extent[3])}, nil
		}
		if coords, ok := asMap(asMap(f)["geometry"])["coordinates"].([]any); ok && len(coords) >= 2 {
			lng, lat := num(coords[0]), num(coords[1])
			return &Bbox{NELat: lat + .05, NELng: lng + .05, SWLat: lat - .05, SWLng: lng - .05}, nil
		}
	}
	return nil, fmt.Errorf("no photon geocode result")
}

func (c *Client) nominatim(ctx context.Context, location string) (*Bbox, error) {
	u := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(location)
	data, err := c.do(ctx, "GET", u, geoUA, nil, nil)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("no geocode result")
	}
	bb, _ := arr[0]["boundingbox"].([]any)
	if len(bb) == 4 {
		return &Bbox{SWLat: parseFloat(str(bb[0])), NELat: parseFloat(str(bb[1])), SWLng: parseFloat(str(bb[2])), NELng: parseFloat(str(bb[3]))}, nil
	}
	lat, lng := parseFloat(str(arr[0]["lat"])), parseFloat(str(arr[0]["lon"]))
	return &Bbox{NELat: lat + .05, NELng: lng + .05, SWLat: lat - .05, SWLng: lng - .05}, nil
}

func set(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setInt(q url.Values, key string, value int) {
	if value > 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(strings.Trim(s, "$,")), 64)
	return f
}

func truncate(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
