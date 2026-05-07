// Package atlasobscura is a polite HTML-scrape client for atlasobscura.com.
//
// Atlas Obscura serves HTML to plain stdlib HTTP with a normal browser-like
// User-Agent. Listings and entry pages embed schema.org Place data in
// <script type="application/ld+json"> blocks; that is the cleanest extraction
// path. When JSON-LD parse fails, City() falls back to a regex that pulls
// js-LinkCard__link anchors from the HTML.
package atlasobscura

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
	defaultBaseURL   = "https://www.atlasobscura.com"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// Entry is a single Atlas Obscura location.
type Entry struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	ImageURL    string  `json:"image_url,omitempty"`
}

// Client is a thin wrapper around atlasobscura.com HTML pages.
type Client struct {
	BaseURL    string
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
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: httpClient,
		UserAgent:  ua,
	}
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
		return nil, fmt.Errorf("atlasobscura request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("atlasobscura read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	return body, nil
}

// jsonLDRe finds every <script type="application/ld+json">...</script> block.
var jsonLDRe = regexp.MustCompile(`(?s)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// extractJSONLD returns each script block's raw JSON payload.
func extractJSONLD(html []byte) [][]byte {
	matches := jsonLDRe.FindAllSubmatch(html, -1)
	out := make([][]byte, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// City fetches a city listing page and returns its embedded Place entries.
// Falls back to a link-card regex when JSON-LD has no usable items.
func (c *Client) City(ctx context.Context, citySlug string) ([]Entry, error) {
	if citySlug == "" {
		return nil, errors.New("atlasobscura: empty city slug")
	}
	full := c.BaseURL + "/things-to-do/" + citySlug
	body, err := c.doGet(ctx, full)
	if err != nil {
		return nil, err
	}
	if entries := parsePlacesFromJSONLD(body); len(entries) > 0 {
		return entries, nil
	}
	return parseLinkCardFallback(body), nil
}

// Entry fetches a single entry page and returns the embedded Place data.
func (c *Client) Entry(ctx context.Context, entryURL string) (*Entry, error) {
	if entryURL == "" {
		return nil, errors.New("atlasobscura: empty entry url")
	}
	body, err := c.doGet(ctx, entryURL)
	if err != nil {
		return nil, err
	}
	entries := parsePlacesFromJSONLD(body)
	if len(entries) == 0 {
		return nil, errors.New("atlasobscura: no Place JSON-LD found")
	}
	e := entries[0]
	if e.URL == "" {
		e.URL = entryURL
	}
	return &e, nil
}

// rawNode holds a permissive view of a JSON-LD object used for type sniffing.
type rawNode struct {
	Type             interface{}     `json:"@type"`
	Graph            []rawNode       `json:"@graph"`
	ItemListElement  json.RawMessage `json:"itemListElement"`
	Name             string          `json:"name"`
	URL              string          `json:"url"`
	Description      string          `json:"description"`
	ShortDescription string          `json:"shortDescription"`
	Image            json.RawMessage `json:"image"`
	Geo              *struct {
		Latitude  json.RawMessage `json:"latitude"`
		Longitude json.RawMessage `json:"longitude"`
	} `json:"geo"`
	// Item shape inside ItemList elements.
	Item *rawNode `json:"item"`
}

// parsePlacesFromJSONLD walks every JSON-LD block and returns Place-like nodes.
func parsePlacesFromJSONLD(html []byte) []Entry {
	var out []Entry
	for _, blob := range extractJSONLD(html) {
		// JSON-LD blocks may be a single object or an array of objects.
		blob = []byte(strings.TrimSpace(string(blob)))
		if len(blob) == 0 {
			continue
		}
		switch blob[0] {
		case '{':
			var node rawNode
			if err := json.Unmarshal(blob, &node); err != nil {
				continue
			}
			out = append(out, walkNode(node)...)
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

// walkNode pulls Place entries from a node, descending into @graph and
// itemListElement collections as needed.
func walkNode(n rawNode) []Entry {
	var out []Entry
	if isPlaceType(n.Type) {
		if e, ok := nodeToEntry(n); ok {
			out = append(out, e)
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

// walkItemListElement handles both array-of-objects and array-of-objects-with-item shapes.
func walkItemListElement(raw json.RawMessage) []Entry {
	var out []Entry
	var arr []rawNode
	if err := json.Unmarshal(raw, &arr); err != nil {
		return out
	}
	for _, el := range arr {
		if el.Item != nil {
			out = append(out, walkNode(*el.Item)...)
			continue
		}
		// Inline element treated as the item itself.
		out = append(out, walkNode(el)...)
	}
	return out
}

// isPlaceType returns true for any schema.org type the scrape clients treat as
// a location of interest.
func isPlaceType(t interface{}) bool {
	switch v := t.(type) {
	case string:
		return placeTypeName(v)
	case []interface{}:
		for _, it := range v {
			if s, ok := it.(string); ok && placeTypeName(s) {
				return true
			}
		}
	}
	return false
}

func placeTypeName(s string) bool {
	switch s {
	case "Place", "TouristAttraction", "LandmarksOrHistoricalBuildings",
		"Museum", "Restaurant", "LocalBusiness", "FoodEstablishment", "Bar":
		return true
	}
	return false
}

// nodeToEntry flattens a Place-like node into an Entry. Returns false when the
// node carries no usable name.
func nodeToEntry(n rawNode) (Entry, bool) {
	if strings.TrimSpace(n.Name) == "" {
		return Entry{}, false
	}
	desc := n.ShortDescription
	if desc == "" {
		desc = n.Description
	}
	e := Entry{
		Title:       n.Name,
		URL:         n.URL,
		Description: desc,
		ImageURL:    extractFirstImage(n.Image),
	}
	if n.Geo != nil {
		e.Lat = jsonNumber(n.Geo.Latitude)
		e.Lng = jsonNumber(n.Geo.Longitude)
	}
	return e, true
}

// extractFirstImage returns the first URL string from a schema.org image field,
// which may be a string, an array of strings, or an ImageObject.
func extractFirstImage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return extractFirstImage(arr[0])
	}
	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.URL
	}
	return ""
}

// jsonNumber parses a schema.org numeric field that may be encoded as a string
// or as a JSON number.
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

// linkCardRe matches the bottom-rail card anchors AO renders for places. The
// title is in the inner text; href is captured. This is the JSON-LD-failure
// fallback only.
var linkCardRe = regexp.MustCompile(`(?s)<a[^>]*class=["'][^"']*js-LinkCard__link[^"']*["'][^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)

// titleStripTags removes leftover tags from a captured anchor's inner HTML.
var titleStripTags = regexp.MustCompile(`<[^>]+>`)

func parseLinkCardFallback(html []byte) []Entry {
	matches := linkCardRe.FindAllSubmatch(html, -1)
	out := make([]Entry, 0, len(matches))
	for _, m := range matches {
		href := string(m[1])
		title := strings.TrimSpace(titleStripTags.ReplaceAllString(string(m[2]), ""))
		if href == "" || title == "" {
			continue
		}
		if !strings.HasPrefix(href, "http") {
			href = defaultBaseURL + href
		}
		out = append(out, Entry{Title: title, URL: href})
	}
	return out
}
