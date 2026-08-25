// Package tock wraps the consumer-side Tock surface. Tock has no public REST
// API — only `/api/business/<int>` and `/api/patron` are accessible to
// authenticated consumers; everything else flows through SSR-rendered HTML
// where the SPA's Redux state ($REDUX_STATE) carries the data we need.
//
// We use enetx/surf for the Chrome TLS fingerprint that clears Cloudflare
// (`cf-mitigated: challenge`).
package tock

// PATCH: cross-network-source-clients — see .printing-press-patches.json for the change-set rationale.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"

	"github.com/enetx/surf"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/jseval"
)

const (
	Origin    = "https://www.exploretock.com"
	APIPrefix = "/api"

	defaultTimeout = 30 * time.Second
)

// Client is a Surf-backed Tock client.
type Client struct {
	http    *http.Client
	session *auth.Session
	limiter *cliutil.AdaptiveLimiter
	origin  string

	// cancelViaUI is a narrow test seam for the HTTP-first cancel dispatcher.
	// Production callers leave it nil and use chromeCancelReservation.
	cancelViaUI func(context.Context, CancelRequest) (*CancelResponse, error)
	// cancelClickDispatch is a narrow test seam for the trusted-click mouse
	// dispatch, letting fixtures inject a mid-dispatch CDP failure.
	// Production callers leave it nil.
	cancelClickDispatch func(ctx context.Context, x, y float64) error
	// cancelUIPhaseTimeout shortens host-gated fixtures without changing the
	// production deadline.
	cancelUIPhaseTimeout time.Duration
}

func (c *Client) baseOrigin() string {
	if c.origin != "" {
		return strings.TrimRight(c.origin, "/")
	}
	return Origin
}

// New creates a Surf-backed Tock client with optional session cookies.
func New(s *auth.Session) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}
	tockURL, _ := auth.CookieURLFor(auth.NetworkTock)
	if s != nil && tockURL != nil {
		jar.SetCookies(tockURL, s.HTTPCookies(auth.NetworkTock))
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
		// Tock's Cloudflare is more permissive than OT's Akamai. 1 req/s
		// floor is comfortable; AdaptiveLimiter ramps up after 10
		// successes and halves on 429.
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	}, nil
}

// do429Aware paces a Tock HTTP request through the adaptive limiter, retries
// once on HTTP 429 with the Retry-After hint, and returns a typed
// `*cliutil.RateLimitError` when retries are exhausted. Empty-on-throttle
// would be indistinguishable from "no data exists" — callers must surface
// this error rather than treating it as a missing venue.
func (c *Client) do429Aware(req *http.Request) (*http.Response, error) {
	return c.do429AwareWithClient(c.http, req)
}

func (c *Client) do429AwareWithClient(httpClient *http.Client, req *http.Request) (*http.Response, error) {
	c.limiter.Wait()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		c.limiter.OnSuccess()
		return resp, nil
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	c.limiter.OnRateLimit()
	wait := cliutil.RetryAfter(resp)
	time.Sleep(wait)
	clonedReq := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		newBody, gerr := req.GetBody()
		if gerr == nil {
			clonedReq.Body = newBody
		}
	}
	c.limiter.Wait()
	resp2, err := httpClient.Do(clonedReq)
	if err != nil {
		return nil, err
	}
	if resp2.StatusCode != http.StatusTooManyRequests {
		c.limiter.OnSuccess()
		return resp2, nil
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	c.limiter.OnRateLimit()
	return nil, &cliutil.RateLimitError{
		URL:        req.URL.String(),
		RetryAfter: cliutil.RetryAfter(resp2),
		Body:       string(body2) + " (initial body: " + string(body) + ")",
	}
}

// LoggedIn reports whether the client has a Tock session cookie.
func (c *Client) LoggedIn() bool {
	return c.session != nil && c.session.LoggedIn(auth.NetworkTock)
}

// Business is a slim view over Tock's /api/business/<id> response.
type Business struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DomainName  string `json:"domainName"`
	TimeZone    string `json:"timeZone"`
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	Country     string `json:"country,omitempty"`
	Cuisine     string `json:"cuisine,omitempty"`
	Description string `json:"description,omitempty"`
	Phone       string `json:"phone,omitempty"`
	WebURL      string `json:"webUrl,omitempty"`
	Address     string `json:"address,omitempty"`
}

// BusinessByID fetches `/api/business/<id>` and returns the unwrapped
// Business object. The Tock envelope is `{"result": {...}}`.
func (c *Client) BusinessByID(ctx context.Context, id int) (*Business, error) {
	url := fmt.Sprintf("%s%s/business/%d", Origin, APIPrefix, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building Tock business request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("calling tock /business/%d: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("tock: business id %d not found", id)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tock /business/%d returned HTTP %d", id, resp.StatusCode)
	}
	var env struct {
		Result Business `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("parsing tock business response: %w", err)
	}
	return &env.Result, nil
}

// Patron is a slim view over Tock's /api/patron response.
type Patron struct {
	ID               int    `json:"id"`
	Email            string `json:"email"`
	FirstName        string `json:"firstName,omitempty"`
	LastName         string `json:"lastName,omitempty"`
	Phone            string `json:"phone,omitempty"`
	ZipCode          string `json:"zipCode,omitempty"`
	Status           string `json:"status,omitempty"`
	UUID             string `json:"uuid,omitempty"`
	IsoCountryCode   string `json:"isoCountryCode,omitempty"`
	PhoneCountryCode string `json:"phoneCountryCode,omitempty"`
}

// CurrentPatron returns the authenticated user's Tock profile. Returns
// 404 when no session is attached.
func (c *Client) CurrentPatron(ctx context.Context) (*Patron, error) {
	if !c.LoggedIn() {
		return nil, errors.New("tock: not authenticated; run `auth login --chrome` to import session cookies")
	}
	url := Origin + APIPrefix + "/patron"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building tock patron request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("calling tock /patron: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 401 {
		return nil, errors.New("tock: not authenticated (cookies expired or invalid); re-run `auth login --chrome`")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tock /patron returned HTTP %d", resp.StatusCode)
	}
	var env struct {
		Result Patron `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("parsing tock patron response: %w", err)
	}
	return &env.Result, nil
}

// reduxStateAnchor anchors on `$REDUX_STATE = ` so we can manually walk
// balanced braces. Tock's state is 200+ KB; a regex on the JSON body is
// not viable.
var reduxStateAnchor = regexp.MustCompile(`\$REDUX_STATE\s*=\s*`)

// FetchReduxState fetches a Tock HTML page and extracts $REDUX_STATE as
// parsed JSON. This is the primary read-path: SSR-rendered Redux carries
// `availability`, `business`, `calendar.offerings`, `search`, `purchase`,
// `patron`, `walkinWaitlist`, `loyaltyProgram`, etc.
//
// Tock SSR uses an unquoted-key JS object literal in some places (for
// `undefined` values); we strip those to JSON-parseable form before unmarshaling.
func (c *Client) FetchReduxState(ctx context.Context, path string) (map[string]any, error) {
	url := Origin + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building tock SSR request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.do429Aware(req)
	if err != nil {
		return nil, fmt.Errorf("fetching tock %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("tock: %s not found (404)", path)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tock %s returned HTTP %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tock %s body: %w", path, err)
	}
	state, err := parseReduxStateBody(body)
	if err != nil {
		return nil, fmt.Errorf("tock %s: %w", path, err)
	}
	return state, nil
}

// parseReduxStateBody extracts and parses the SSR-emitted $REDUX_STATE assignment
// from a Tock HTML body. Primary path uses goja (Tock's state contains JS-only
// constructs like `undefined` and function expressions); regex+strip is a
// fallback for builds where goja's stringify trips JSON.
func parseReduxStateBody(body []byte) (map[string]any, error) {
	jsonBody, err := jseval.ExtractObjectLiteral(body, reduxStateAnchor)
	if err != nil {
		return nil, fmt.Errorf("$REDUX_STATE not found: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(jsonBody, &state); err != nil {
		legacy, lerr := extractReduxState(body)
		if lerr == nil {
			cleaned := stripJSUndefined(legacy)
			if err2 := json.Unmarshal(cleaned, &state); err2 == nil {
				return state, nil
			}
		}
		return nil, fmt.Errorf("parsing $REDUX_STATE: %w", err)
	}
	return state, nil
}

// extractReduxState locates `$REDUX_STATE = {...}` and returns the JSON body
// using balanced-brace walking (string-aware, escape-aware). Sized for Tock's
// 200+ KB state which a regex cannot match correctly.
func extractReduxState(body []byte) ([]byte, error) {
	loc := reduxStateAnchor.FindIndex(body)
	if loc == nil {
		return nil, fmt.Errorf("anchor not found")
	}
	i := loc[1]
	for i < len(body) && body[i] != '{' {
		i++
	}
	if i >= len(body) {
		return nil, fmt.Errorf("no JSON body after anchor")
	}
	depth := 0
	inString := false
	escape := false
	for j := i; j < len(body); j++ {
		ch := body[j]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}
		if inString {
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return body[i : j+1], nil
			}
		}
	}
	return nil, fmt.Errorf("unbalanced braces")
}

// stripJSUndefined replaces standalone `undefined` keywords with `null` so
// the Redux state is JSON-parseable. Tock's bundler emits `undefined` in
// many contexts: object values (`:undefined`), array slots (`[undefined,...]`),
// after commas (`,undefined`), and at the start (`{a:undefined,...`). We use
// a word-boundary-ish replacement that respects JSON's neighboring tokens.
var jsUndefinedRE = regexp.MustCompile(`(?:^|[^a-zA-Z_$])undefined(?:$|[^a-zA-Z0-9_$])`)

func stripJSUndefined(b []byte) []byte {
	// Replace all occurrences in an iterate-and-rewrite loop so we capture
	// overlapping matches (e.g., `[undefined,undefined]` where the comma
	// is shared between the two matches). The simple ReplaceAllFunc preserves
	// the boundary characters from the regex.
	return jsUndefinedRE.ReplaceAllFunc(b, func(match []byte) []byte {
		// Replace the `undefined` token with `null`, keeping the boundary chars.
		out := make([]byte, 0, len(match))
		for i := 0; i < len(match); {
			if i+9 <= len(match) && string(match[i:i+9]) == "undefined" {
				out = append(out, []byte("null")...)
				i += 9
				continue
			}
			out = append(out, match[i])
			i++
		}
		return out
	})
}

// VenueAvailability fetches `/<venue>/search?date=YYYY-MM-DD&size=N&time=HH:MM`
// and extracts the offerings + availability bits the SPA renders SSR.
func (c *Client) VenueAvailability(ctx context.Context, slug string, date string, size int, t string) (map[string]any, error) {
	if t == "" {
		t = "20:00"
	}
	if size <= 0 {
		size = 2
	}
	path := fmt.Sprintf("/%s/search?date=%s&size=%d&time=%s",
		strings.TrimPrefix(slug, "/"), date, size, t)
	state, err := c.FetchReduxState(ctx, path)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if cal, ok := state["calendar"].(map[string]any); ok {
		out["calendar"] = cal
	}
	if av, ok := state["availability"].(map[string]any); ok {
		out["availability"] = av
	}
	// Business identity is at app.config.business, not the top-level
	// business slice (which is just request-tracking metadata).
	if app, ok := state["app"].(map[string]any); ok {
		if cfg, ok := app["config"].(map[string]any); ok {
			if biz, ok := cfg["business"].(map[string]any); ok && len(biz) > 0 {
				out["business"] = biz
			}
		}
	}
	return out, nil
}

// VenueDetail fetches `/<venue>` and extracts business + offerings from the
// SSR Redux state.
//
// Business identity (id, name, city, cuisine, etc.) lives at
// `state.app.config.business` after SSR hydration; the top-level
// `state.business` slice is just request-tracking metadata. We surface the
// hydrated business object as the canonical `business` field on the return,
// and additionally include `state.app.activeAuth.businessId` so callers can
// hit `/api/business/<id>` for the full REST detail when needed.
func (c *Client) VenueDetail(ctx context.Context, slug string) (map[string]any, error) {
	state, err := c.FetchReduxState(ctx, "/"+strings.TrimPrefix(slug, "/"))
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if app, ok := state["app"].(map[string]any); ok {
		if cfg, ok := app["config"].(map[string]any); ok {
			if biz, ok := cfg["business"].(map[string]any); ok && len(biz) > 0 {
				out["business"] = biz
			}
		}
		if activeAuth, ok := app["activeAuth"].(map[string]any); ok {
			out["active_auth"] = activeAuth
		}
	}
	if cal, ok := state["calendar"].(map[string]any); ok {
		out["calendar"] = cal
	}
	if pkg, ok := state["packageState"].(map[string]any); ok {
		out["packages"] = pkg
	}
	return out, nil
}
