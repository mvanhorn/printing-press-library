// Package eater is a polite HTML-scrape client for Eater "best X in Y"
// map/list pages.
//
// Eater map pages embed schema.org JSON-LD with either an ItemList of
// Restaurant items (proper) or a sequence of inline Restaurant blocks
// (fallback). BestOf handles both shapes.
package eater

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
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// Restaurant is a single Eater listing entry.
type Restaurant struct {
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Description string  `json:"description"`
	ListURL     string  `json:"list_url"`
	ItemURL     string  `json:"item_url,omitempty"`
}

// Client is a thin wrapper around eater.com map pages.
type Client struct {
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client with sensible defaults.
func New(httpClient *http.Client, ua string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{HTTPClient: httpClient, UserAgent: ua}
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
		return nil, fmt.Errorf("eater request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("eater read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	return body, nil
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

// rawNode is a permissive view over schema.org JSON.
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

// BestOf fetches an Eater map page and extracts Restaurant entries from its
// embedded JSON-LD. The input URL is preserved on each entry as ListURL.
func (c *Client) BestOf(ctx context.Context, listURL string) ([]Restaurant, error) {
	if listURL == "" {
		return nil, errors.New("eater: empty list url")
	}
	body, err := c.doGet(ctx, listURL)
	if err != nil {
		return nil, err
	}
	out := parseRestaurantsFromJSONLD(body)
	for i := range out {
		out[i].ListURL = listURL
	}
	return out, nil
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
		"Bar", "BarOrPub", "CafeOrCoffeeShop", "Bakery", "Winery":
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
		ItemURL:     n.URL,
		Address:     extractAddress(n.Address),
	}
	if n.Geo != nil {
		r.Lat = jsonNumber(n.Geo.Latitude)
		r.Lng = jsonNumber(n.Geo.Longitude)
	}
	return r, true
}

// extractAddress flattens a schema.org address that can be a string or a
// PostalAddress object.
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

// firstSentence trims a description to its first sentence, or returns it as-is
// when no sentence terminator is found.
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
