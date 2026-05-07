// Package michelin is an HTML-scrape client for guide.michelin.com.
//
// guide.michelin.com is fronted by AWS WAF: stdlib HTTP returns 202 with a
// challenge body, and only a Chrome-fingerprint TLS client clears the gate.
// Because the wider CLI does not yet bundle a browser-impersonation
// transport, this package is intentionally pluggable: callers may swap in
// any http.RoundTripper (for example, a Surf-backed transport) by setting
// http.Client.Transport. When the gate trips, methods return ErrChallenged
// so the caller can decide whether to retry through that transport.
//
// JSON-LD parsing follows the same shape as the eater and timeout packages:
// schema.org Restaurant entries are pulled from
// <script type="application/ld+json"> blocks.
package michelin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

const (
	defaultBaseURL   = "https://guide.michelin.com"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// ErrChallenged signals that an AWS WAF challenge intercepted the request.
// The caller should retry through a Chrome-fingerprint transport (e.g. Surf).
var ErrChallenged = errors.New("michelin: AWS-WAF challenge — retry through a Chrome-fingerprint transport")

// Restaurant is a single Michelin Guide listing.
type Restaurant struct {
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Description string  `json:"description"`
	URL         string  `json:"url,omitempty"`
}

// Client is a thin wrapper around guide.michelin.com pages.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client. The caller may set httpClient.Transport to a
// browser-impersonating RoundTripper (for example, Surf) to clear the WAF.
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

// wafActionRe matches the AWS-WAF inline body marker used on challenge pages.
var wafActionRe = regexp.MustCompile(`(?i)(?:<title>\s*Action\s+Required|aws-waf-token|awswafCookie)`)

// isChallenged classifies a response as a WAF challenge.
func isChallenged(status int, headers http.Header, body []byte) bool {
	if headers.Get("x-amzn-waf-action") != "" {
		return true
	}
	if status == http.StatusAccepted {
		// 202 is Michelin/AWS-WAF's challenge response code on guide.michelin.com.
		return true
	}
	if wafActionRe.Match(body) {
		return true
	}
	return false
}

func (c *Client) doGet(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("michelin request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("michelin read body: %w", err)
	}
	if isChallenged(resp.StatusCode, resp.Header, body) {
		return nil, ErrChallenged
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	return body, nil
}

// RestaurantsForRegion fetches a region listings page and returns its embedded
// schema.org Restaurant entries.
func (c *Client) RestaurantsForRegion(ctx context.Context, regionSlug string) ([]Restaurant, error) {
	if regionSlug == "" {
		return nil, errors.New("michelin: empty region slug")
	}
	full := c.BaseURL + "/en/" + regionSlug + "/restaurants"
	body, err := c.doGet(ctx, full)
	if err != nil {
		return nil, err
	}
	out := parseRestaurantsFromJSONLD(body)
	for i := range out {
		if out[i].URL == "" {
			out[i].URL = full
		}
	}
	return out, nil
}

// Restaurant fetches a single restaurant page and returns its primary entry.
func (c *Client) Restaurant(ctx context.Context, fullURL string) (*Restaurant, error) {
	if fullURL == "" {
		return nil, errors.New("michelin: empty url")
	}
	body, err := c.doGet(ctx, fullURL)
	if err != nil {
		return nil, err
	}
	out := parseRestaurantsFromJSONLD(body)
	if len(out) == 0 {
		return nil, errors.New("michelin: no Restaurant JSON-LD found")
	}
	r := out[0]
	if r.URL == "" {
		r.URL = fullURL
	}
	return &r, nil
}

var jsonLDRe = regexp.MustCompile(`(?s)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

func extractJSONLD(html []byte) [][]byte {
	matches := jsonLDRe.FindAllSubmatch(html, -1)
	out := make([][]byte, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

type rawNode struct {
	Type            interface{}     `json:"@type"`
	Graph           []rawNode       `json:"@graph"`
	ItemListElement json.RawMessage `json:"itemListElement"`
	Name            string          `json:"name"`
	URL             string          `json:"url"`
	Description     string          `json:"description"`
	Address         json.RawMessage `json:"address"`
	Geo             *struct {
		Latitude  json.RawMessage `json:"latitude"`
		Longitude json.RawMessage `json:"longitude"`
	} `json:"geo"`
	Item *rawNode `json:"item"`
}

func parseRestaurantsFromJSONLD(html []byte) []Restaurant {
	var out []Restaurant
	for _, blob := range extractJSONLD(html) {
		blob = []byte(strings.TrimSpace(string(blob)))
		if len(blob) == 0 {
			continue
		}
		switch blob[0] {
		case '{':
			var n rawNode
			if err := json.Unmarshal(blob, &n); err != nil {
				continue
			}
			out = append(out, walkNode(n)...)
		case '[':
			var arr []rawNode
			if err := json.Unmarshal(blob, &arr); err != nil {
				continue
			}
			for _, n := range arr {
				out = append(out, walkNode(n)...)
			}
		}
	}
	return out
}

func walkNode(n rawNode) []Restaurant {
	var out []Restaurant
	if isRestaurantType(n.Type) {
		if r, ok := nodeToRestaurant(n); ok {
			out = append(out, r)
		}
	}
	for _, g := range n.Graph {
		out = append(out, walkNode(g)...)
	}
	if len(n.ItemListElement) > 0 {
		out = append(out, walkItemListElement(n.ItemListElement)...)
	}
	return out
}

func walkItemListElement(raw json.RawMessage) []Restaurant {
	var out []Restaurant
	var arr []rawNode
	if err := json.Unmarshal(raw, &arr); err != nil {
		return out
	}
	for _, el := range arr {
		if el.Item != nil {
			out = append(out, walkNode(*el.Item)...)
			continue
		}
		out = append(out, walkNode(el)...)
	}
	return out
}

func isRestaurantType(t interface{}) bool {
	switch v := t.(type) {
	case string:
		return restaurantTypeName(v)
	case []interface{}:
		for _, it := range v {
			if s, ok := it.(string); ok && restaurantTypeName(s) {
				return true
			}
		}
	}
	return false
}

func restaurantTypeName(s string) bool {
	switch s {
	case "Restaurant", "FoodEstablishment", "LocalBusiness",
		"Bar", "BarOrPub", "CafeOrCoffeeShop":
		return true
	}
	return false
}

func nodeToRestaurant(n rawNode) (Restaurant, bool) {
	if strings.TrimSpace(n.Name) == "" {
		return Restaurant{}, false
	}
	r := Restaurant{
		Name:        n.Name,
		Description: firstSentence(n.Description),
		URL:         n.URL,
		Address:     extractAddress(n.Address),
	}
	if n.Geo != nil {
		r.Lat = jsonNumber(n.Geo.Latitude)
		r.Lng = jsonNumber(n.Geo.Longitude)
	}
	return r, true
}

func extractAddress(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		StreetAddress   string `json:"streetAddress"`
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		PostalCode      string `json:"postalCode"`
		AddressCountry  string `json:"addressCountry"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	parts := []string{}
	for _, p := range []string{obj.StreetAddress, obj.AddressLocality, obj.AddressRegion, obj.PostalCode, obj.AddressCountry} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

func jsonNumber(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return 0
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, ".!?"); i > 0 && i < len(s)-1 {
		return strings.TrimSpace(s[:i+1])
	}
	return s
}
