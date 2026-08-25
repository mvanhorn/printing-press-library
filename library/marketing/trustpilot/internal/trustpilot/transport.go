package trustpilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultUserAgent is a desktop Chrome-shaped UA we attach to every replay
// request; the WAF cares more about the cookie than the UA, but matching the
// UA from the browser-sniff harvest reduces drift between harvest and replay.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// BaseURL is the Trustpilot host; agent-browser captures and JSON-API replays
// both go here.
const BaseURL = "https://www.trustpilot.com"

// Client is a cookie-replay HTTP client for the Trustpilot Next.js JSON API.
// The Chrome-harvested aws-waf-token cookie + the x-nextjs-data header are
// what turn 403/308 responses into 200s; both are non-negotiable.
type Client struct {
	HTTP    *http.Client
	Session Session
}

// NewClient builds a Client around the supplied Session. Caller is
// responsible for ensuring the Session is fresh (see Session.IsFresh).
func NewClient(s Session) *Client {
	if s.UserAgent == "" {
		s.UserAgent = DefaultUserAgent
	}
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Session: s,
	}
}

// FetchPage hits the /_next/data/<buildId>/review/<domain>.json endpoint
// with the supplied filters. Returns the decoded page on success, or a typed
// error so callers can distinguish "cookie expired" from "no such page".
func (c *Client) FetchPage(ctx context.Context, domain string, filters PageFilters) (ReviewsPage, error) {
	if c.Session.ReviewsBuildID == "" {
		return ReviewsPage{}, fmt.Errorf("session has no reviews build id; run auth login first")
	}
	if domain == "" {
		return ReviewsPage{}, fmt.Errorf("domain required")
	}
	u, err := url.Parse(fmt.Sprintf("%s/_next/data/%s/review/%s.json", BaseURL, c.Session.ReviewsBuildID, domain))
	if err != nil {
		return ReviewsPage{}, err
	}
	q := u.Query()
	q.Set("businessUnit", domain)
	// Trustpilot's Next.js handler soft-redirects "page=1" to the canonical
	// no-query form (returns 200 with __N_REDIRECT in pageProps instead of
	// real data). Only attach &page= when it would actually move us off page 1.
	if filters.Page > 1 {
		q.Set("page", strconv.Itoa(filters.Page))
	}
	if filters.Stars > 0 && filters.Stars <= 5 {
		q.Set("stars", strconv.Itoa(filters.Stars))
	}
	if filters.Language != "" {
		q.Set("languages", filters.Language)
	}
	switch filters.Sort {
	case "recency", "relevance":
		q.Set("sort", filters.Sort)
	}
	if filters.DateWindow != "" {
		q.Set("date", filters.DateWindow)
	}
	if filters.VerifiedOnly {
		q.Set("verified", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ReviewsPage{}, err
	}
	c.applyHeaders(req, fmt.Sprintf("%s/review/%s", BaseURL, domain))

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ReviewsPage{}, fmt.Errorf("fetching reviews: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		return ReviewsPage{}, &CookieExpiredError{Status: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusNotFound {
		// PATCH: Next.js soft redirects on 404 indicate unsupported filters, not stale build ids.
		var redirectEnvelope struct {
			PageProps struct {
				NRedirect string `json:"__N_REDIRECT"`
			} `json:"pageProps"`
		}
		if err := json.Unmarshal(body, &redirectEnvelope); err == nil && redirectEnvelope.PageProps.NRedirect != "" {
			return ReviewsPage{}, &FilterUnsupportedError{ParamHint: filterParamHint(filters)}
		}
		return ReviewsPage{}, &BuildIDStaleError{Status: resp.StatusCode, Domain: domain}
	}
	if resp.StatusCode/100 != 2 {
		return ReviewsPage{}, fmt.Errorf("trustpilot reviews returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var envelope struct {
		PageProps json.RawMessage `json:"pageProps"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ReviewsPage{}, fmt.Errorf("decoding reviews envelope: %w", err)
	}
	return ParseReviewsPage(domain, envelope.PageProps)
}

// FetchSearch hits the /_next/data/<searchBuild>/search.json endpoint.
func (c *Client) FetchSearch(ctx context.Context, query string) ([]SearchHit, error) {
	if c.Session.SearchBuildID == "" {
		return nil, fmt.Errorf("session has no search build id; run auth login first")
	}
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	// PATCH(greptile P2 PR#588): propagate url.Parse and http.NewRequestWithContext
	// errors instead of nil-dereferencing on the next u.Query() / req.Header.Set call.
	u, err := url.Parse(fmt.Sprintf("%s/_next/data/%s/search.json", BaseURL, c.Session.SearchBuildID))
	if err != nil {
		return nil, fmt.Errorf("parsing search url: %w", err)
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building search request: %w", err)
	}
	c.applyHeaders(req, fmt.Sprintf("%s/search?query=%s", BaseURL, url.QueryEscape(query)))

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching search: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		return nil, &CookieExpiredError{Status: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, &BuildIDStaleError{Status: resp.StatusCode}
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("trustpilot search returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var envelope struct {
		PageProps json.RawMessage `json:"pageProps"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decoding search envelope: %w", err)
	}
	return ParseSearchPage(envelope.PageProps)
}

// PageFilters captures every query knob exposed by the Trustpilot review
// endpoint.
type PageFilters struct {
	Page         int    // 1-indexed
	Stars        int    // 0 means unfiltered, 1..5 for a specific bin
	Language     string // ISO 639-1
	Sort         string // "" | "recency" | "relevance"
	DateWindow   string // "" | "last30days" | "last3months" | "last6months" | "last12months"
	VerifiedOnly bool
}

func (c *Client) applyHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", c.Session.UserAgent)
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", referer)
	req.Header.Set("x-nextjs-data", "1") // forces JSON response instead of HTML redirect
	if c.Session.CookieJar != "" {
		req.Header.Set("Cookie", c.Session.CookieJar)
	} else if c.Session.AWSWAFToken != "" {
		req.Header.Set("Cookie", "aws-waf-token="+c.Session.AWSWAFToken)
	}
}

// CookieExpiredError signals that the WAF cookie no longer authorizes the
// caller; the session should be re-harvested before retrying.
type CookieExpiredError struct{ Status int }

func (e *CookieExpiredError) Error() string {
	return fmt.Sprintf("aws-waf-token rejected (HTTP %d); re-run auth login", e.Status)
}

// BuildIDStaleError signals an HTTP 404 from a Next.js data endpoint. On the
// reviews endpoint (Domain set) the likelier cause is that no Trustpilot
// review page exists for that domain -- review pages are keyed by domain, so
// a company name or a lookalike domain 404s here; a stale build id is the
// rarer alternative. On the search endpoint (Domain empty) the path has no
// domain component, so a 404 is the stale-build-id signal itself. One type
// covers both because the retry helpers key on it to trigger a re-harvest.
type BuildIDStaleError struct {
	Status int
	Domain string
}

func (e *BuildIDStaleError) Error() string {
	if e.Domain != "" {
		return fmt.Sprintf(
			"no Trustpilot review page found for %q (HTTP %d) - find the canonical domain with: trustpilot-pp-cli search '<company name>'; if the domain is correct, the build id may be stale - re-run auth login",
			e.Domain, e.Status,
		)
	}
	return fmt.Sprintf("Next.js build id rejected (HTTP %d); re-run auth login to refresh", e.Status)
}

type FilterUnsupportedError struct{ ParamHint string }

func (e *FilterUnsupportedError) Error() string {
	return fmt.Sprintf("trustpilot rejected the request (likely an unsupported filter parameter%s); try omitting it or using --local on synced data", paramSuffix(e.ParamHint))
}

func paramSuffix(h string) string {
	if h == "" {
		return ""
	}
	return ": " + h
}

func filterParamHint(filters PageFilters) string {
	switch {
	case filters.DateWindow != "":
		return "date=" + filters.DateWindow
	case filters.Language != "":
		return "languages=" + filters.Language
	case filters.Stars > 0:
		return "stars=" + strconv.Itoa(filters.Stars)
	case filters.VerifiedOnly:
		return "verified=true"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// EncodeCookieJar takes a "k=v; k=v" header value and returns it unchanged
// after stripping whitespace. Exists so future logic (filtering out
// non-essential cookies, etc.) has a single funnel.
func EncodeCookieJar(jar string) string {
	parts := strings.Split(jar, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}
