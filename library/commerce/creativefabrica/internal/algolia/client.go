// Package algolia is a hand-written client for Creative Fabrica's public,
// search-only Algolia catalog index. Creative Fabrica has no official API; its
// website search is powered by Algolia, and this package talks to that same
// index directly over standard HTTPS (no Cloudflare, no browser at runtime).
//
// The public app id and search-only key are NOT compiled into this CLI. They
// are resolved at runtime in this order:
//
//  1. CREATIVEFABRICA_ALGOLIA_APP_ID / CREATIVEFABRICA_ALGOLIA_API_KEY env vars
//  2. a cached credentials file under the user config dir
//  3. best-effort auto-discovery from the live site's JavaScript bundle
//
// The key is referer-restricted, so every request sends Origin/Referer headers.
package algolia

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

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
)

const (
	// DefaultAppID is Creative Fabrica's public Algolia application id. An app
	// id is a non-secret identifier (it appears in every catalog request URL),
	// so shipping it as a default is safe; the search key is never shipped.
	DefaultAppID = "KO25I12XZ3"

	// IndexRelevance is the default (relevance-ranked) products index.
	IndexRelevance = "prod_Productsv2"
	// IndexNewest is the newest-first sort replica.
	IndexNewest = "prod_Productsv2_trending_newest"

	siteOrigin  = "https://www.creativefabrica.com"
	siteReferer = "https://www.creativefabrica.com/"
)

// flexString decodes a JSON value that the catalog index sometimes returns as a
// string ("2.99"), sometimes as a boolean (false = "no regular price"), and
// occasionally as a number. It always presents as a Go string.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null" || s == "false":
		*f = ""
	case s == "true":
		*f = "true"
	case len(s) >= 2 && s[0] == '"':
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*f = flexString(v)
	default:
		*f = flexString(strings.Trim(s, `"`))
	}
	return nil
}

// String returns the decoded value.
func (f flexString) String() string { return string(f) }

// flexFloat decodes a JSON value that the catalog index returns as a number,
// a numeric string ("2.99"), false, or null, into a Go float64.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "false" || s == `""` {
		*f = 0
		return nil
	}
	if len(s) >= 2 && s[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		var n float64
		// A malformed numeric string (e.g. "free") legitimately decodes to 0;
		// the parse error is the signal for that, not a failure to surface.
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &n); err != nil {
			n = 0
		}
		*f = flexFloat(n)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexFloat(n)
	return nil
}

// Float returns the decoded value.
func (f flexFloat) Float() float64 { return float64(f) }

// Designer is the nested designer object on a catalog hit.
type Designer struct {
	DesignerID   int    `json:"designerId"`
	DesignerName string `json:"designerName"`
	DesignerURL  string `json:"designerUrl"`
}

// Hit is one catalog product returned by Algolia. Field tags match the live
// index attribute names exactly.
type Hit struct {
	ObjectID            string     `json:"objectID"`
	NameEN              string     `json:"name_en"`
	DescriptionEN       string     `json:"description_en"`
	Type                string     `json:"type"`
	Category            []string   `json:"category"`
	Tags                []string   `json:"tags"`
	Image               string     `json:"image"`
	URL                 string     `json:"url"`
	Designer            Designer   `json:"designer"`
	Price               flexFloat  `json:"price"`
	RegularPrice        flexString `json:"regularPrice"`
	IsFree              bool       `json:"isFree"`
	HasPod              bool       `json:"hasPod"`
	HasPromotions       bool       `json:"hasPromotions"`
	OutsideSubscription bool       `json:"outsideSubscription"`
	IsExclusive         bool       `json:"isExclusive"`
	Popularity          flexFloat  `json:"popularity"`
	Date                int64      `json:"date"`
}

// SearchRequest is a single Algolia query. Zero-value fields are omitted.
type SearchRequest struct {
	IndexName         string
	Query             string
	Page              int
	HitsPerPage       int
	Filters           string
	Facets            []string
	MaxValuesPerFacet int
}

// SearchResult is the per-query slice of an Algolia multi-query response.
type SearchResult struct {
	Hits        []Hit                     `json:"hits"`
	NbHits      int                       `json:"nbHits"`
	NbPages     int                       `json:"nbPages"`
	Page        int                       `json:"page"`
	HitsPerPage int                       `json:"hitsPerPage"`
	Index       string                    `json:"index"`
	Facets      map[string]map[string]int `json:"facets"`
}

// maxAttempts bounds 429 retries before a typed RateLimitError surfaces.
const maxAttempts = 5

// Client queries the Creative Fabrica Algolia catalog.
type Client struct {
	HTTP    *http.Client
	Creds   Creds
	BaseURL string // override host; empty => https://{appID}-dsn.algolia.net
	DryRun  bool
	limiter *cliutil.AdaptiveLimiter
}

// New builds a client with the given timeout and per-second rate cap (0 uses a
// conservative default). Credentials are resolved lazily on the first request
// so commands that only --dry-run never touch the network.
func New(timeout time.Duration, ratePerSec ...float64) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	rate := 8.0
	if len(ratePerSec) > 0 && ratePerSec[0] > 0 {
		rate = ratePerSec[0]
	}
	return &Client{HTTP: &http.Client{Timeout: timeout}, limiter: cliutil.NewAdaptiveLimiter(rate)}
}

func (c *Client) host() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	appID := c.Creds.AppID
	if appID == "" {
		appID = DefaultAppID
	}
	return "https://" + appID + "-dsn.algolia.net"
}

// EnsureCreds resolves credentials once and caches them on the client.
func (c *Client) EnsureCreds(ctx context.Context) error {
	if c.Creds.APIKey != "" {
		return nil
	}
	creds, err := ResolveCreds(ctx, c.HTTP)
	if err != nil {
		return err
	}
	c.Creds = creds
	return nil
}

// Search runs one or more Algolia queries and returns one result per request,
// in order.
func (c *Client) Search(ctx context.Context, reqs ...SearchRequest) ([]SearchResult, error) {
	if len(reqs) == 0 {
		return nil, fmt.Errorf("no search requests")
	}
	if err := c.EnsureCreds(ctx); err != nil {
		return nil, err
	}

	type algoliaReq struct {
		IndexName         string   `json:"indexName"`
		Query             string   `json:"query"`
		Page              int      `json:"page"`
		HitsPerPage       int      `json:"hitsPerPage,omitempty"`
		Filters           string   `json:"filters,omitempty"`
		Facets            []string `json:"facets,omitempty"`
		MaxValuesPerFacet int      `json:"maxValuesPerFacet,omitempty"`
	}
	body := struct {
		Requests []algoliaReq `json:"requests"`
	}{}
	for _, r := range reqs {
		if r.IndexName == "" {
			r.IndexName = IndexRelevance
		}
		body.Requests = append(body.Requests, algoliaReq{
			IndexName:         r.IndexName,
			Query:             r.Query,
			Page:              r.Page,
			HitsPerPage:       r.HitsPerPage,
			Filters:           r.Filters,
			Facets:            r.Facets,
			MaxValuesPerFacet: r.MaxValuesPerFacet,
		})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("x-algolia-application-id", c.Creds.AppID)
	q.Set("x-algolia-api-key", c.Creds.APIKey)
	q.Set("x-algolia-agent", "creativefabrica-pp-cli")
	endpoint := c.host() + "/1/indexes/*/queries?" + q.Encode()

	var data []byte
	var lastStatus int
	var retryAfter time.Duration
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if c.limiter != nil {
			c.limiter.Wait()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", siteOrigin)
		req.Header.Set("Referer", siteReferer)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("calling catalog search: %w", err)
		}
		data, _ = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		lastStatus = resp.StatusCode
		retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			if c.limiter != nil {
				c.limiter.OnRateLimit()
			}
			if attempt < maxAttempts-1 {
				// Back off before retrying: honor a server Retry-After when
				// present, else exponential (0.5s, 1s, 2s, 4s) capped at 5s.
				// The shared public search key is rate-limited, so a brief
				// wait lets a transient throttle clear instead of failing.
				wait := retryAfter
				if wait <= 0 {
					wait = time.Duration(500<<attempt) * time.Millisecond
					if wait > 5*time.Second {
						wait = 5 * time.Second
					}
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
			}
			continue // retry until maxAttempts
		}
		if c.limiter != nil {
			c.limiter.OnSuccess()
		}
		break
	}
	if lastStatus == http.StatusTooManyRequests {
		return nil, &cliutil.RateLimitError{URL: endpoint, RetryAfter: retryAfter, Body: snippet(data)}
	}
	if lastStatus == http.StatusForbidden {
		return nil, fmt.Errorf("catalog search returned 403 (the public search key may have rotated; run 'creativefabrica-pp-cli doctor' or set CREATIVEFABRICA_ALGOLIA_API_KEY)")
	}
	if lastStatus < 200 || lastStatus >= 300 {
		return nil, fmt.Errorf("catalog search returned HTTP %d: %s", lastStatus, snippet(data))
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parsing catalog response: %w", err)
	}
	return out.Results, nil
}

// parseRetryAfter parses a Retry-After header given as integer seconds.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
