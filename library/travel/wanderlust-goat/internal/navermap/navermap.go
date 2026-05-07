// Package navermap is a thin client for Naver Map's instantSearch JSON
// endpoint (https://map.naver.com/p/api/search/instantSearch). The endpoint
// usually requires a logged-in session cookie; without one, it returns 401/403
// and the caller should treat the source as unavailable.
package navermap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

const (
	defaultBaseURL   = "https://map.naver.com/p/api/search/instantSearch"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// ErrUnauthenticated signals that the instantSearch endpoint refused the
// request because we have no session cookie. Orchestrators should record the
// source as unavailable rather than treating this as a hard failure.
var ErrUnauthenticated = errors.New("navermap: instantSearch requires a session cookie — set NAVER_MAP_COOKIE")

// Place is a flattened Naver Map search hit.
type Place struct {
	Name     string  `json:"name"`
	Address  string  `json:"address"`
	Category string  `json:"category"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	ID       string  `json:"id"`
}

// Client is a Naver Map client. Cookie, when set, is sent verbatim as the
// Cookie header — typically populated from $NAVER_MAP_COOKIE.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Cookie     string
}

// New constructs a Client.
func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpClient,
		UserAgent:  ua,
	}
}

// instantSearchEnvelope is the documented-by-observation shape of a successful
// instantSearch response. The endpoint returns a top-level array of category
// groups; we flatten the "place" group into Place values. Coordinates are
// returned in millionths of a degree (e.g. 37.5665° → 37566500).
type instantSearchEnvelope struct {
	Place []rawPlace `json:"place"`
	// Some response variants nest under "items":
	Items []rawPlace `json:"items"`
}

type rawPlace struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Address  string `json:"address"`
	Category string `json:"category"`
	// Naver historically used these field names.
	X any `json:"x"` // longitude in WGS84 degrees (string or float)
	Y any `json:"y"` // latitude in WGS84 degrees (string or float)
}

// Search calls instantSearch and flattens the place group. Empty query is
// rejected before any HTTP call; 401/403 is mapped to ErrUnauthenticated.
func (c *Client) Search(ctx context.Context, query string) ([]Place, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("navermap search: empty query")
	}
	v := url.Values{}
	v.Set("query", query)
	v.Set("type", "all")
	v.Set("searchCoord", "")
	full := c.BaseURL + "?" + v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://map.naver.com/")
	if c.Cookie != "" {
		req.Header.Set("Cookie", c.Cookie)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("navermap request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("navermap read body: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthenticated
	default:
		return nil, fmt.Errorf("%s returned %d: %s", full, resp.StatusCode, httperr.Snippet(body))
	}

	var env instantSearchEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("navermap parse json: %w", err)
	}
	raws := env.Place
	if len(raws) == 0 {
		raws = env.Items
	}

	out := make([]Place, 0, len(raws))
	for _, rp := range raws {
		name := rp.Name
		if name == "" {
			name = rp.Title
		}
		if name == "" {
			continue
		}
		out = append(out, Place{
			Name:     name,
			Address:  rp.Address,
			Category: rp.Category,
			Lat:      coordAsFloat(rp.Y),
			Lng:      coordAsFloat(rp.X),
			ID:       rp.ID,
		})
	}
	return out, nil
}

// coordAsFloat accepts either a JSON number or a stringified number.
func coordAsFloat(v any) float64 {
	switch tv := v.(type) {
	case float64:
		return tv
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(tv), 64)
		if err == nil {
			return f
		}
	}
	return 0
}
