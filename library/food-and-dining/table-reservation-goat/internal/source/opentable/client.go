// Package opentable wraps OpenTable's consumer surface (the dapi GraphQL
// endpoint, REST booking, and SSR-rendered HTML pages). The OpenTable
// Partner API is out of scope.
//
// Auth model: OpenTable's bot defense (Akamai) requires a Chrome TLS
// fingerprint AND, for authenticated reads, the `authCke` session cookie
// the user has after logging in to opentable.com. We use enetx/surf for
// the TLS fingerprint and the session cookies imported via auth login.
package opentable

// PATCH: cross-network-source-clients — see .printing-press-patches.json for the change-set rationale.

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enetx/surf"
	"golang.org/x/sync/singleflight"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

const (
	// Origin is the OpenTable consumer host. Every request goes through here.
	Origin = "https://www.opentable.com"

	// GraphQLPath is the persisted-query GraphQL endpoint. CSRF + cookies
	// authenticate; persisted-query hashes drift over bundle releases.
	GraphQLPath = "/dapi/fe/gql"

	// AutocompleteHash is the live persisted-query hash captured during
	// browser-sniff (2026-05-09). On `PersistedQueryNotFound` 400, the
	// client re-fetches the homepage and bootstraps a fresh hash.
	AutocompleteHash = "fe1d118abd4c227750693027c2414d43014c2493f64f49bcef5a65274ce9c3c3"

	// RestaurantsAvailabilityHash is the persisted-query hash for
	// `RestaurantsAvailability` cited by 21Bruce/resolved-bot's Go client
	// (`FindKey`). The same hash appears across multiple community wrappers
	// as the working hash; it drifts on bundle releases but a working hash
	// is always discoverable. v1 caches this value; on a
	// PersistedQueryNotFound (400) error, the client surfaces a clear hint
	// that the user should run `doctor --refresh-hashes` (a v0.2 escape
	// hatch).
	// RestaurantsAvailabilityHash captured live from the OT consumer frontend
	// in May 2026. The hash rotates roughly per-bundle release; if it drifts
	// the gateway returns 409 (Apollo persisted-query mismatch) and a
	// follow-up needs to re-capture from a fresh /r/<slug> request.
	RestaurantsAvailabilityHash = "cbcf4838a9b399f742e3741785df64560a826d8d3cc2828aa01ab09a8455e29e"

	defaultTimeout = 30 * time.Second
)

// Client is a Surf-based OpenTable client with the user's session cookies
// attached.
type Client struct {
	mu      sync.Mutex
	http    *http.Client
	session *auth.Session
	limiter *cliutil.AdaptiveLimiter

	// bootstrapSF dedupes concurrent Bootstrap() calls. Two goroutines
	// that both observe a stale csrfToken would otherwise both fire
	// the home-page GET; singleflight collapses that into a single
	// in-flight request whose result every waiter receives.
	bootstrapSF singleflight.Group

	// availSF dedupes concurrent RestaurantsAvailability calls for the
	// same logical request. The key includes restID/date/time/party,
	// the request window, and noCache so callers asking for explicitly
	// fresh data don't piggyback on cache-allowed leaders' results.
	availSF singleflight.Group

	csrfToken      string
	csrfFetchedAt  time.Time
	csrfTTL        time.Duration
	autocompleteH  string
	autocompleteHM time.Time
}

// New creates a Surf-backed OpenTable client. Pass the loaded auth.Session
// to attach the user's cookies; pass nil for an anonymous client (search,
// availability — but not booking, my-reservations, or wishlist).
//
// Akamai's anti-bot cookies (`bm_sz`, `_abck`, `ftc`, …) rotate every ~30
// minutes; the snapshot saved by `auth login --chrome` goes stale within
// the hour and Akamai 403s any request without fresh values. We re-read
// just those cookies from Chrome at construction time so each invocation
// rides on whatever Chrome's challenge handling has earned recently. The
// long-lived auth cookies still come from the saved session, so the CLI
// keeps working when Chrome is closed.
func New(s *auth.Session) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}
	otURL, _ := auth.CookieURLFor(auth.NetworkOpenTable)
	if s != nil && otURL != nil {
		fresh := auth.RefreshAkamaiCookies(context.Background(), "opentable.com")
		jar.SetCookies(otURL, s.HTTPCookiesWithRefresh(auth.NetworkOpenTable, fresh))
	}
	surfClient := surf.NewClient().
		Builder().
		Impersonate().Chrome().
		Session().
		Build().
		Unwrap()
	std := surfClient.Std()
	std.Jar = jar
	std.Timeout = defaultTimeout
	return &Client{
		http:    std,
		session: s,
		// Default rate: 0.5 req/s (1 every 2s) — conservative against
		// Akamai's WAF on this client. AdaptiveLimiter ramps up after 10
		// consecutive successes and halves on rate-limit signals. Power
		// users can override the initial rate via TRG_OT_THROTTLE_RATE
		// (calls per second, clamped to [0.01, 5.0]).
		limiter:       cliutil.NewAdaptiveLimiter(readThrottleRate()),
		csrfTTL:       30 * time.Minute,
		autocompleteH: AutocompleteHash,
	}, nil
}

// do429Aware paces a request through the adaptive limiter, retries once on
// HTTP 429 with the Retry-After hint, and returns a typed
// `*cliutil.RateLimitError` when retries are exhausted. Empty-on-throttle is
// a recipe for silent data corruption: callers MUST surface this error
// rather than treating it as "no data exists".
//
// Bot-detection coverage: the unified HTTP entry point also fast-fails when
// a disk-persisted cooldown is active and records a new cooldown when the
// upstream returns 403. SSR fetches (FetchInitialState) and GraphQL calls
// (gqlCall) both flow through this path, so cooldown coverage is uniform
// across read paths.
func (c *Client) do429Aware(req *http.Request) (*http.Response, error) {
	if active := loadActiveCooldown(); active != nil {
		return nil, active
	}
	c.limiter.Wait()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 403 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		_ = body
		// Only the bootstrap path warrants a *session-wide* cooldown — a
		// 403 there means our Surf-Chrome client is shadow-banned and
		// subsequent calls won't get past Akamai. A 403 on any other path
		// is operation-specific (e.g. Akamai's WAF rule on
		// `opname=RestaurantsAvailability`) and shouldn't poison sibling
		// operations like Autocomplete that work fine on the same session.
		if req.URL.Path != bootstrapPath {
			// Akamai's opname-specific WAF rule is PROBABILISTIC — same
			// request retried 750ms later often goes through. Single retry
			// caps the cost; more retries here would compound the session's
			// reputation hit. Callers needing a longer retry budget (e.g.
			// `RestaurantsAvailability`) layer their own second retry on
			// top of the BotDetectionError this returns.
			if req.GetBody != nil {
				ctx := req.Context()
				select {
				case <-time.After(750 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				retry := req.Clone(ctx)
				if newBody, gerr := req.GetBody(); gerr == nil {
					retry.Body = newBody
				}
				retryResp, retryErr := c.http.Do(retry)
				if retryErr == nil {
					if retryResp.StatusCode == 200 {
						c.limiter.OnSuccess()
						return retryResp, nil
					}
					retryResp.Body.Close()
				}
			}
			return nil, &BotDetectionError{
				URL:    req.URL.String(),
				Status: 403,
				Streak: 1,
				Until:  time.Now().Add(1 * time.Minute),
				Reason: fmt.Sprintf("403 from %s after retry (operation-specific WAF rule, not a session-wide block)", req.URL.Path),
			}
		}
		bde, sErr := setCooldown(fmt.Sprintf("403 from %s (Akamai anti-bot)", req.URL.Path))
		if sErr != nil {
			return nil, &BotDetectionError{
				URL: req.URL.String(), Status: 403, Streak: 1,
				Until:  time.Now().Add(5 * time.Minute),
				Reason: fmt.Sprintf("403 from %s; cooldown not persisted: %v", req.URL.Path, sErr),
			}
		}
		return nil, bde
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		c.limiter.OnSuccess()
		return resp, nil
	}
	// Drain + close the 429 body so we can retry the request.
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	c.limiter.OnRateLimit()
	wait := cliutil.RetryAfter(resp)
	time.Sleep(wait)
	// Single retry. Clone the request to reset the body reader (if any)
	// and avoid mutating the caller's req.
	clonedReq := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		newBody, gerr := req.GetBody()
		if gerr == nil {
			clonedReq.Body = newBody
		}
	}
	c.limiter.Wait()
	resp2, err := c.http.Do(clonedReq)
	if err != nil {
		return nil, err
	}
	if resp2.StatusCode != http.StatusTooManyRequests {
		c.limiter.OnSuccess()
		return resp2, nil
	}
	// Retry also rate-limited. Surface the typed error so the caller
	// can distinguish "throttled" from "no results".
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	c.limiter.OnRateLimit()
	return nil, &cliutil.RateLimitError{
		URL:        req.URL.String(),
		RetryAfter: cliutil.RetryAfter(resp2),
		Body:       string(body2) + " (initial body: " + string(body) + ")",
	}
}

// LoggedIn reports whether the client is configured with an OpenTable
// session cookie.
func (c *Client) LoggedIn() bool {
	return c.session != nil && c.session.LoggedIn(auth.NetworkOpenTable)
}

// bootstrapURL is the SSR page we fetch to extract a fresh `__CSRF_TOKEN__`.
// Akamai's WAF on opentable.com is configured with a stricter rule on the
// home page (`/`) than on `/restaurant/profile/<id>` — `/` 403s a Surf-Chrome
// client almost immediately, while a numeric profile page returns 200 with
// the SSR Redux state intact. We picked id=100 (Walnut Creek Yacht Club, a
// long-lived listing) as a stable bootstrap source. The CSRF token returned
// from this page is bound to the consumer-frontend GraphQL gateway just like
// the home page's, so the rest of the flow is unchanged.
const bootstrapPath = "/restaurant/profile/100"

// Bootstrap fetches the OpenTable bootstrap page to extract `__CSRF_TOKEN__`.
// Idempotent — only refreshes when the cached token is older than csrfTTL.
// Concurrent callers are deduplicated via singleflight so a single in-flight
// fetch satisfies all waiters.
//
// Bot-detection cooldown: before any HTTP fetch, this method checks the
// disk-persisted cooldown (set on prior 403s) and fast-fails with a typed
// `*BotDetectionError` if the cooldown is still active. On a 403 from the
// bootstrap page, it writes a new cooldown with exponential backoff per
// consecutive 403, so the next CLI invocation doesn't waste a 30s timeout
// before failing. A successful 200 clears the cooldown.
func (c *Client) Bootstrap(ctx context.Context) error {
	c.mu.Lock()
	fresh := c.csrfToken != "" && time.Since(c.csrfFetchedAt) < c.csrfTTL
	c.mu.Unlock()
	if fresh {
		return nil
	}
	// Fast-fail if a prior 403 cooldown is still active.
	if active := loadActiveCooldown(); active != nil {
		return active
	}
	_, err, _ := c.bootstrapSF.Do("csrf", func() (any, error) {
		c.mu.Lock()
		if c.csrfToken != "" && time.Since(c.csrfFetchedAt) < c.csrfTTL {
			c.mu.Unlock()
			return nil, nil
		}
		c.mu.Unlock()
		// Re-check cooldown inside the singleflight closure — another
		// caller may have updated it between our outer check and here.
		if active := loadActiveCooldown(); active != nil {
			return nil, active
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, Origin+bootstrapPath, nil)
		if err != nil {
			return nil, fmt.Errorf("building bootstrap request: %w", err)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, err := c.do429Aware(req)
		if err != nil {
			return nil, fmt.Errorf("fetching opentable.com: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 403 {
			// Persistent anti-bot block — record cooldown and surface
			// a typed error so callers render the right remediation.
			bde, sErr := setCooldown("bootstrap returned 403 from " + bootstrapPath + " (Akamai anti-bot)")
			if sErr != nil {
				// Best effort — even if persistence failed, return a
				// transient bot-detection error so the user sees the
				// right error class.
				return nil, &BotDetectionError{
					URL: Origin + bootstrapPath, Status: 403, Streak: 1,
					Until:  time.Now().Add(5 * time.Minute),
					Reason: "bootstrap returned 403 from " + bootstrapPath + " (Akamai anti-bot); cooldown not persisted: " + sErr.Error(),
				}
			}
			return nil, bde
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("opentable.com%s returned HTTP %d during bootstrap", bootstrapPath, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading bootstrap body: %w", err)
		}
		token := extractCSRFToken(body)
		if token == "" {
			return nil, fmt.Errorf("could not extract __CSRF_TOKEN__ from opentable.com%s; site structure may have changed", bootstrapPath)
		}
		// 200 clears any previous cooldown so the next 403 starts a
		// fresh streak rather than escalating from stale state.
		clearCooldown()
		c.mu.Lock()
		c.csrfToken = token
		c.csrfFetchedAt = time.Now()
		c.mu.Unlock()
		return nil, nil
	})
	return err
}

// csrfRE matches both the JSON-embedded form (which is what the SSR HTML
// actually serves: `"__CSRF_TOKEN__":"<uuid>"`) and the runtime JS-assignment
// form (`window.__CSRF_TOKEN__ = "<uuid>"`). The JSON form is what we get
// from a Surf-cleared GET on the home page; the JS form is what real Chrome
// sees after JS hydration. Either is acceptable.
var csrfRE = regexp.MustCompile(`(?:window\.__CSRF_TOKEN__\s*=\s*['"]|"__CSRF_TOKEN__"\s*:\s*")([0-9a-fA-F-]{16,})`)

func extractCSRFToken(html []byte) string {
	m := csrfRE.FindSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// CSRF returns the current cached CSRF token (call Bootstrap first).
func (c *Client) CSRF() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.csrfToken
}

// AutocompleteHash returns the current cached persisted-query hash for
// the Autocomplete operation.
func (c *Client) AutocompleteHash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.autocompleteH
}

// AutocompleteResult is one entry in the Autocomplete response.
type AutocompleteResult struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Country          string  `json:"country,omitempty"`
	MetroName        string  `json:"metroName,omitempty"`
	NeighborhoodName string  `json:"neighborhoodName,omitempty"`
	Type             string  `json:"type"` // "Restaurant", "Cuisine", "Location"
	Latitude         float64 `json:"latitude,omitempty"`
	Longitude        float64 `json:"longitude,omitempty"`
	URLSlug          string  `json:"urlSlug,omitempty"`
}

// Autocomplete searches OpenTable for restaurants matching `term` near the
// provided lat/lng. Works without auth (CSRF token is sufficient).
func (c *Client) Autocomplete(ctx context.Context, term string, lat, lng float64) ([]AutocompleteResult, error) {
	if err := c.Bootstrap(ctx); err != nil {
		return nil, err
	}
	body := map[string]any{
		"operationName": "Autocomplete",
		"variables": map[string]any{
			"term":          term,
			"latitude":      lat,
			"longitude":     lng,
			"useNewVersion": true,
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": c.AutocompleteHash(),
			},
		},
	}
	parsed, err := c.gqlCall(ctx, "Autocomplete", body)
	if err != nil {
		return nil, err
	}
	// Response shape: data.autocomplete.autocompleteResults[]
	type respShape struct {
		Data struct {
			Autocomplete struct {
				Results []AutocompleteResult `json:"autocompleteResults"`
			} `json:"autocomplete"`
		} `json:"data"`
	}
	var r respShape
	if err := json.Unmarshal(parsed, &r); err != nil {
		return nil, fmt.Errorf("parsing Autocomplete response: %w", err)
	}
	return r.Data.Autocomplete.Results, nil
}

// AvailabilitySlot is one open reservation slot returned by
// `RestaurantsAvailability`. The slot tokens are short-lived (~minutes) and
// are required to actually book the slot via `make-reservation`.
type AvailabilitySlot struct {
	IsAvailable           bool     `json:"isAvailable"`
	TimeOffsetMinutes     int      `json:"timeOffsetMinutes"`
	SlotHash              string   `json:"slotHash"`
	SlotAvailabilityToken string   `json:"slotAvailabilityToken"`
	PointsType            string   `json:"pointsType,omitempty"`
	PointsValue           int      `json:"pointsValue,omitempty"`
	Attributes            []string `json:"attributes,omitempty"`
}

// AvailabilityDay is one day in the availability response. The new gateway
// (May 2026) returns `DayOffset` (days from the requested `date`) instead of
// a `Date` field — `Date` is left for back-compat but is unset on fresh
// responses; callers compute the actual date as request.Date + DayOffset.
type AvailabilityDay struct {
	Date      string             `json:"date,omitempty"`
	DayOffset int                `json:"dayOffset"`
	Slots     []AvailabilitySlot `json:"slots"`
}

// RestaurantAvailability is the per-restaurant chunk of the response: one
// restaurant's availability across N days starting from `date`.
//
// `CachedAt`, `Stale`, and `Source` are stale-cache-fallback metadata:
//   - CachedAt: when the entry was originally fetched (set on cache write).
//   - Stale: true when the entry is past TTL (independent of how it was reached).
//   - Source: "cache_fallback" when the data came from the BotDetectionError
//     fallback path; empty when fresh-from-network or fresh-cache-hit.
//
// All three are zero/empty on fresh network responses.
type RestaurantAvailability struct {
	RestaurantID     int               `json:"restaurantId"`
	AvailabilityDays []AvailabilityDay `json:"availabilityDays"`
	CachedAt         time.Time         `json:"cached_at,omitempty"`
	Stale            bool              `json:"stale,omitempty"`
	Source           string            `json:"source,omitempty"`
}

// RestaurantsAvailability calls the documented `RestaurantsAvailability`
// GraphQL persisted-query and returns one chunk per requested restaurant ID.
// Slot tokens in the response are short-lived (~minutes) and are required
// for the actual booking POST.
//
// Wraps the network call with three resilience layers:
//   - Disk cache (per-key, schema+hash invalidated, default 3min TTL)
//   - Singleflight dedupe (concurrent same-key calls share one fetch)
//   - Two-attempt retry budget (750ms in do429Aware, 5s here for the
//     second attempt) — only `RestaurantsAvailability` pays the second
//     retry; sibling ops keep their existing single-retry behavior
//
// `noCache=true` bypasses the cache read but still writes on success.
// Singleflight key includes `noCache` so cache-bypass callers don't
// piggyback on cache-allowed leaders' fresh-fetch results.
func (c *Client) RestaurantsAvailability(ctx context.Context, restaurantIDs []int, date, hhmm string, partySize, forwardDays, forwardMinutes, forwardSlots int, noCache bool) ([]RestaurantAvailability, error) {
	if forwardMinutes <= 0 {
		forwardMinutes = 150
	}
	if hhmm == "" {
		hhmm = "19:00"
	}
	if partySize <= 0 {
		partySize = 2
	}
	// Cache + singleflight only meaningful for single-restaurant calls
	// (the typical caller pattern). Multi-restaurant calls bypass — they
	// represent a batch fetch whose cache key shape would differ.
	if len(restaurantIDs) != 1 {
		return c.restaurantsAvailabilityNetwork(ctx, restaurantIDs, date, hhmm, partySize, forwardDays, forwardMinutes, forwardSlots)
	}
	restID := restaurantIDs[0]
	key := availCacheKey{
		RestID:          restID,
		Date:            date,
		Time:            hhmm,
		PartySize:       partySize,
		ForwardMinutes:  forwardMinutes,
		BackwardMinutes: forwardMinutes,
	}

	// Cache check (skip when noCache).
	if !noCache {
		if hit := loadAvailCache(key, RestaurantsAvailabilityHash); hit != nil && hit.Fresh {
			return hit.Entry.Response, nil
		}
	}

	// Singleflight dedupe — key includes noCache so cache-bypass callers
	// don't share a flight with cache-allowed callers (semantics differ
	// on cache-write side and on what they consider acceptable).
	sfKey := fmt.Sprintf("avail:%d:%s:%s:%d:%d:%d:%t",
		restID, date, hhmm, partySize, forwardMinutes, forwardMinutes, noCache)
	v, err, _ := c.availSF.Do(sfKey, func() (any, error) {
		// Fire network call (which goes through do429Aware → AdaptiveLimiter).
		// On BotDetectionError after the in-do429Aware 750ms retry, sleep 5s
		// (ctx-aware) and retry once more before surfacing.
		resp, nerr := c.restaurantsAvailabilityNetwork(ctx, restaurantIDs, date, hhmm, partySize, forwardDays, forwardMinutes, forwardSlots)
		if nerr != nil {
			if _, isBot := IsBotDetection(nerr); isBot {
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				resp, nerr = c.restaurantsAvailabilityNetwork(ctx, restaurantIDs, date, hhmm, partySize, forwardDays, forwardMinutes, forwardSlots)
			}
			if nerr != nil {
				// U5 stale-cache fallback: when the network is genuinely
				// blocked (BotDetectionError after both retries), serve
				// the cached entry IF one exists within the 24h hard cap.
				// Tag with `source: "cache_fallback"` so consumers can
				// distinguish this from a fresh fetch. `Stale` is set
				// when the entry is past TTL.
				if _, isBot := IsBotDetection(nerr); isBot {
					if hit := loadAvailCache(key, RestaurantsAvailabilityHash); hit != nil {
						return enrichWithCacheMetadata(hit.Entry.Response, hit.Entry.CachedAt, !hit.Fresh), nil
					}
				}
				return nil, nerr
			}
		}
		// Always write cache on success — even when noCache=true. A user
		// asking for fresh data is also willing to update what's cached.
		saveAvailCache(key, RestaurantsAvailabilityHash, resp)
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RestaurantAvailability), nil
}

// enrichWithCacheMetadata stamps Source="cache_fallback", Stale, and
// CachedAt onto each row of a cached response so JSON consumers can see
// the data came from the U5 stale-fallback path rather than a fresh
// fetch. Mutates a copy — the cache entry itself isn't modified.
func enrichWithCacheMetadata(resp []RestaurantAvailability, cachedAt time.Time, stale bool) []RestaurantAvailability {
	out := make([]RestaurantAvailability, len(resp))
	for i, r := range resp {
		r.CachedAt = cachedAt
		r.Stale = stale
		r.Source = "cache_fallback"
		out[i] = r
	}
	return out
}

// restaurantsAvailabilityNetwork is the bare network call that
// RestaurantsAvailability wraps. Kept private so callers go through the
// cache+singleflight+retry envelope rather than bypassing it.
func (c *Client) restaurantsAvailabilityNetwork(ctx context.Context, restaurantIDs []int, date, hhmm string, partySize, forwardDays, forwardMinutes, forwardSlots int) ([]RestaurantAvailability, error) {
	if forwardDays <= 0 {
		forwardDays = 1
	}
	if forwardMinutes <= 0 {
		forwardMinutes = 150
	}
	if forwardSlots <= 0 {
		forwardSlots = 5
	}
	if hhmm == "" {
		hhmm = "19:00"
	}
	if partySize <= 0 {
		partySize = 2
	}
	// Variable shape captured live from the OT consumer frontend (May 2026):
	// onlyPop / requireTimes / useCBR / privilegedAccess flags drive feature
	// gating; the older v3 shape (useNewVersion / forwardSlots / etc.) is
	// gone. forwardDays=0 + forwardMinutes/backwardMinutes describes a
	// time WINDOW around the requested time on a single day, not a multi-day
	// span; the page hits this endpoint per-day when scanning a window.
	// `restaurantAvailabilityTokens` and `loyaltyRedemptionTiers` are arrays
	// the gateway requires to be present (empty arrays accepted).
	// `attributionToken` and `correlationId` are analytics; safe to leave
	// blank — server treats absence as an anonymous request.
	body := map[string]any{
		"operationName": "RestaurantsAvailability",
		"variables": map[string]any{
			"onlyPop": false,
			// forwardDays=0 means "single day" in the new gateway —
			// forwardMinutes/backwardMinutes describe a time window
			// on the requested date only. Multi-day scans loop the
			// caller's `forwardDays` outside this function.
			"forwardDays":      0,
			"requireTimes":     false,
			"requireTypes":     []string{"Standard"},
			"useCBR":           false,
			"privilegedAccess": []string{"UberOneDiningProgram"},
			"restaurantIds":    restaurantIDs,
			"date":             date,
			"time":             hhmm,
			"partySize":        partySize,
			// "NA" (North America) — the gateway validates against a
			// known region enum; "us" rejects with HTTP 400.
			"databaseRegion":               "NA",
			"restaurantAvailabilityTokens": []string{},
			"loyaltyRedemptionTiers":       []string{},
			"attributionToken":             "",
			// correlationId is a per-request UUID the gateway logs.
			// Empty string sometimes 400s; a fresh UUID always passes.
			"correlationId":   newUUID(),
			"forwardMinutes":  forwardMinutes,
			"backwardMinutes": forwardMinutes,
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": RestaurantsAvailabilityHash,
			},
		},
	}
	_ = forwardSlots // accepted for signature parity; new gateway uses time window instead
	parsed, err := c.gqlCall(ctx, "RestaurantsAvailability", body)
	if err != nil {
		return nil, err
	}
	type respShape struct {
		Data struct {
			Availability []RestaurantAvailability `json:"availability"`
		} `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	var r respShape
	if err := json.Unmarshal(parsed, &r); err != nil {
		return nil, fmt.Errorf("parsing RestaurantsAvailability response: %w", err)
	}
	if len(r.Errors) > 0 {
		// Surface PersistedQueryNotFound with a clear hint — the cached
		// hash has drifted past whatever OT's bundle currently expects.
		first := r.Errors[0]
		if strings.Contains(strings.ToUpper(first.Message), "PERSISTED") || first.Extensions.Code == "PERSISTED_QUERY_NOT_FOUND" {
			return nil, fmt.Errorf("opentable: persisted-query hash drifted (RestaurantsAvailability returned %q); cached hash %s no longer accepted by upstream — the cached value will need to be refreshed in a follow-up release", first.Message, RestaurantsAvailabilityHash[:16])
		}
		return nil, fmt.Errorf("opentable RestaurantsAvailability: %s", first.Message)
	}
	return r.Data.Availability, nil
}

// RestaurantIDFromQuery resolves a free-text query (or a slug like
// `le-bernardin-new-york`) to a restaurant ID via Autocomplete. Picks the
// first result whose lowercase name contains the lowercase query
// (slug-dashes converted to spaces). Returns 0 if no match — caller
// surfaces a "couldn't resolve" error.
func (c *Client) RestaurantIDFromQuery(ctx context.Context, query string, lat, lng float64) (id int, name, slug string, err error) {
	q := strings.ReplaceAll(strings.ToLower(query), "-", " ")
	q = strings.TrimSpace(q)
	if q == "" {
		return 0, "", "", fmt.Errorf("empty query")
	}
	results, err := c.Autocomplete(ctx, q, lat, lng)
	if err != nil {
		return 0, "", "", err
	}
	for _, r := range results {
		nameLower := strings.ToLower(r.Name)
		if r.Type != "Restaurant" {
			continue
		}
		if !strings.Contains(nameLower, q) && !strings.Contains(q, nameLower) {
			// Some autocomplete responses lead with token-prefix matches.
			// If the user passed a multi-word query and the result name
			// matches the first significant token, accept.
			tokens := strings.Fields(q)
			if len(tokens) == 0 || !strings.Contains(nameLower, tokens[0]) {
				continue
			}
		}
		idInt := 0
		fmt.Sscanf(r.ID, "%d", &idInt)
		if idInt == 0 {
			continue
		}
		return idInt, r.Name, r.URLSlug, nil
	}
	return 0, "", "", fmt.Errorf("no opentable restaurant matched %q", query)
}

// gqlCall posts a GraphQL request with the persisted-query envelope. On
// PersistedQueryNotFound (a 400 with that errors[].extensions.code), it
// could re-bootstrap the hash from a homepage scrape — for v1 we surface
// the error so the user can run `doctor --refresh-hashes`.
func (c *Client) gqlCall(ctx context.Context, opname string, body any) ([]byte, error) {
	js, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling GraphQL body: %w", err)
	}
	u := Origin + GraphQLPath + "?optype=query&opname=" + url.QueryEscape(opname)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(js)))
	if err != nil {
		return nil, fmt.Errorf("building GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("X-CSRF-Token", c.CSRF())
	req.Header.Set("Origin", Origin)
	// Referer must match a page we actually browsed — Akamai cross-checks the
	// Referer against recent navigations on this session. The bootstrap page
	// works for every operation that doesn't require a venue-specific page.
	req.Header.Set("Referer", Origin+bootstrapPath)
	// apollographql-client-name is what real Chrome sends; some Akamai rules
	// flag GraphQL requests that arrive without it as bot traffic.
	req.Header.Set("apollographql-client-name", "fe-search")
	req.Header.Set("apollographql-client-version", "0.0.1")
	req.Header.Set("x-query-timeout", "10000")
	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", opname, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", opname, err)
	}
	// Note: 403 retry happens inside `do429Aware` (single 750ms attempt).
	// `do429Aware` returns either a 200 response on success/retry-success,
	// or a typed `*BotDetectionError` after retry exhausted — never a 403
	// response with a nil error. The `RestaurantsAvailability` wrapper
	// adds a SECOND retry attempt at 5s on top of the BotDetectionError.
	// This block exists as defense-in-depth: if a future change in
	// `do429Aware` were to surface a raw 403 here, surface it as
	// BotDetectionError so callers' downstream handling stays correct.
	if resp.StatusCode == 403 {
		return nil, &BotDetectionError{
			URL:    u,
			Status: 403,
			Streak: 1,
			Until:  time.Now().Add(1 * time.Minute),
			Reason: "GraphQL " + opname + " returned 403 (unexpected — do429Aware should have converted to BotDetectionError before reaching gqlCall)",
		}
	}
	if resp.StatusCode != 200 {
		// PersistedQueryNotFound is a 400 with text/plain "Bad Request" or a
		// JSON `errors[].extensions.code === "PERSISTED_QUERY_NOT_FOUND"`.
		// Surface it with a hint so callers know to refresh hashes.
		hint := ""
		if resp.StatusCode == 400 {
			hint = " (likely a stale persisted-query hash; run `doctor --refresh-hashes` if this is recurring)"
		}
		if resp.StatusCode == 409 {
			hint = " (Apollo persisted-query gateway: body operationName must match the URL opname AND match the hash's registered name)"
		}
		return nil, fmt.Errorf("opentable %s returned HTTP %d%s: %s", opname, resp.StatusCode, hint, truncate(string(data), 200))
	}
	return data, nil
}

// newUUID generates an RFC-4122 v4 UUID for the GraphQL correlationId.
// crypto/rand is used because the gateway logs these and we don't want
// CLI invocations to collide. Errors fall back to a deterministic value
// so the request still goes out — the gateway validates shape, not
// uniqueness.
func newUUID() string {
	var b [16]byte
	if _, err := cryptoRand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// throttleRateMin and throttleRateMax bound the TRG_OT_THROTTLE_RATE
// override so a misconfigured value can't disable rate limiting entirely
// (rate=0 would division-by-zero in AdaptiveLimiter) or set rates so high
// that bursts hit Akamai instantly. 0.01 ≈ 100s spacing (paranoid mode);
// 5.0 = 200ms spacing (private-proxy or test environments).
const (
	throttleRateMin     = 0.01
	throttleRateMax     = 5.0
	throttleRateDefault = 0.5
)

// readThrottleRate returns the AdaptiveLimiter initial rate from
// TRG_OT_THROTTLE_RATE. Returns the default on unset, unparseable, or
// out-of-range values, with a stderr warning so misconfigurations are
// noticed.
func readThrottleRate() float64 {
	v := strings.TrimSpace(os.Getenv("TRG_OT_THROTTLE_RATE"))
	if v == "" {
		return throttleRateDefault
	}
	r, err := strconv.ParseFloat(v, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TRG_OT_THROTTLE_RATE=%q is not a valid number; using default %.2f\n", v, throttleRateDefault)
		return throttleRateDefault
	}
	if r < throttleRateMin || r > throttleRateMax {
		fmt.Fprintf(os.Stderr, "TRG_OT_THROTTLE_RATE=%.4f out of range [%.2f, %.2f]; using default %.2f\n",
			r, throttleRateMin, throttleRateMax, throttleRateDefault)
		return throttleRateDefault
	}
	return r
}
