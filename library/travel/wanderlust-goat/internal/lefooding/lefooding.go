// Package lefooding scrapes Le Fooding's English search and restaurant pages
// (https://lefooding.com). The site embeds schema.org Restaurant JSON-LD on
// detail pages, so we prefer parsing those blocks over walking the DOM.
package lefooding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://lefooding.com"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// Restaurant is a flattened editorial entry from Le Fooding.
type Restaurant struct {
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	City         string  `json:"city"`
	Neighborhood string  `json:"neighborhood,omitempty"`
	Description  string  `json:"description,omitempty"`
	URL          string  `json:"url"`
	Lat          float64 `json:"lat,omitempty"`
	Lng          float64 `json:"lng,omitempty"`
}

// Client is a no-auth Le Fooding scraper.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New constructs a Client. nil http client gets a 10-second default; empty UA
// gets the project default.
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

// jsonLDRe matches every <script type="application/ld+json"> block in a page.
var jsonLDRe = regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>([\s\S]+?)</script>`)

// cardRe pulls cards off the search results page. The selector is intentionally
// loose: Le Fooding's markup has shifted between Drupal templates over the
// years, so we match any <article>...</article> that contains an <a href="/en/">
// pointing to a non-search detail page.
var cardLinkRe = regexp.MustCompile(`(?is)<a[^>]+href=["'](/en/[^"'#?]+)["'][^>]*>([\s\S]+?)</a>`)

// tagStripRe collapses inline tags so we can extract bare text.
var tagStripRe = regexp.MustCompile(`<[^>]+>`)

// whitespaceRe collapses runs of whitespace into a single space.
var whitespaceRe = regexp.MustCompile(`\s+`)

func (c *Client) get(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lefooding request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lefooding read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", fullURL, resp.StatusCode)
	}
	return body, nil
}

// Search runs a Le Fooding text search and returns up to ~30 restaurant cards.
// We first try /en/search?q=<q>; if that returns no cards, we retry with
// ?query=<q> for older template versions.
func (c *Client) Search(ctx context.Context, query string) ([]Restaurant, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("lefooding search: empty query")
	}
	base := strings.TrimRight(c.BaseURL, "/")

	tryParam := func(name string) ([]byte, error) {
		v := url.Values{}
		v.Set(name, query)
		return c.get(ctx, fmt.Sprintf("%s/en/search?%s", base, v.Encode()))
	}

	body, err := tryParam("q")
	if err != nil {
		return nil, err
	}
	results := parseSearchCards(string(body), base)
	if len(results) == 0 {
		// Fall back to ?query=<q> for older Le Fooding templates.
		alt, altErr := tryParam("query")
		if altErr == nil {
			results = parseSearchCards(string(alt), base)
		}
	}
	return results, nil
}

// parseSearchCards walks the search HTML, finds anchor cards under /en/ and
// pulls a name + short description + city/address out of each. We deliberately
// avoid loading a real HTML parser and rely on the consistent card structure.
func parseSearchCards(html, base string) []Restaurant {
	matches := cardLinkRe.FindAllStringSubmatch(html, -1)
	out := make([]Restaurant, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		href := m[1]
		// Filter out search/listing/category links — we only want detail pages.
		if strings.HasPrefix(href, "/en/search") ||
			strings.HasPrefix(href, "/en/category") ||
			strings.HasPrefix(href, "/en/article") ||
			strings.HasPrefix(href, "/en/guide") ||
			href == "/en/" {
			continue
		}
		if seen[href] {
			continue
		}
		seen[href] = true

		inner := m[2]
		// Le Fooding cards put the name in <h3> and city in a sibling <span>;
		// neither is separated by a comma in the source, so we insert a "|"
		// between adjacent block-level tags before stripping. That gives us a
		// stable separator to split name vs. city downstream.
		text := stripAndCollapseWithSep(inner, "|")
		if text == "" {
			continue
		}
		// Card text typically reads: "Name | City | — One-line description".
		name, city, desc := splitCardText(text)
		if name == "" {
			continue
		}
		fullURL := base + href
		out = append(out, Restaurant{
			Name:        name,
			City:        city,
			Description: desc,
			URL:         fullURL,
		})
		if len(out) >= 30 {
			break
		}
	}
	return out
}

func splitCardText(text string) (name, city, desc string) {
	// Try splitting on em-dash first (Le Fooding uses U+2014 between city and
	// description on most cards), then ASCII " - ", then return the whole text
	// as the name.
	if i := strings.Index(text, "—"); i >= 0 {
		left := strings.TrimSpace(text[:i])
		desc = strings.TrimSpace(text[i+len("—"):])
		name, city = splitNameCity(left)
		return
	}
	if i := strings.Index(text, " - "); i >= 0 {
		left := strings.TrimSpace(text[:i])
		desc = strings.TrimSpace(text[i+len(" - "):])
		name, city = splitNameCity(left)
		return
	}
	name, city = splitNameCity(text)
	return
}

// splitNameCity tries comma first, then the "|" block-tag separator we
// inserted, then a single space; if nothing splits cleanly the whole string is
// treated as the name.
func splitNameCity(s string) (string, string) {
	s = strings.TrimSpace(strings.Trim(s, "|"))
	for _, sep := range []string{",", "|"} {
		if i := strings.LastIndex(s, sep); i >= 0 {
			name := strings.TrimSpace(strings.Trim(s[:i], "|"))
			city := strings.TrimSpace(strings.Trim(s[i+len(sep):], "|"))
			return name, city
		}
	}
	return strings.TrimSpace(s), ""
}

// Restaurant fetches a single Le Fooding restaurant page. We prefer JSON-LD
// (schema.org Restaurant) when it is present and fall back to lightweight HTML
// scraping otherwise.
func (c *Client) Restaurant(ctx context.Context, fullURL string) (*Restaurant, error) {
	if strings.TrimSpace(fullURL) == "" {
		return nil, fmt.Errorf("lefooding restaurant: empty url")
	}
	body, err := c.get(ctx, fullURL)
	if err != nil {
		return nil, err
	}
	r := parseRestaurantPage(string(body))
	r.URL = fullURL
	return r, nil
}

// jsonLDRestaurant is a permissive view over schema.org Restaurant. Le Fooding
// emits both flat strings and PostalAddress objects in `address`, so we accept
// both shapes via json.RawMessage.
type jsonLDRestaurant struct {
	Type        any             `json:"@type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Address     json.RawMessage `json:"address"`
	URL         string          `json:"url"`
	Geo         *struct {
		Latitude  any `json:"latitude"`
		Longitude any `json:"longitude"`
	} `json:"geo"`
}

type postalAddress struct {
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
	PostalCode      string `json:"postalCode"`
	AddressCountry  any    `json:"addressCountry"`
}

func parseRestaurantPage(html string) *Restaurant {
	r := &Restaurant{}

	for _, m := range jsonLDRe.FindAllStringSubmatch(html, -1) {
		raw := strings.TrimSpace(m[1])
		// Some pages emit `[ {...}, {...} ]` arrays; handle both shapes.
		var anyVal any
		if err := json.Unmarshal([]byte(raw), &anyVal); err != nil {
			continue
		}
		blocks := flattenJSONLD(anyVal)
		for _, b := range blocks {
			if !typeMatches(b["@type"], "Restaurant", "FoodEstablishment", "LocalBusiness") {
				continue
			}
			// Re-marshal and decode into the typed view for ergonomic field access.
			raw2, _ := json.Marshal(b)
			var jr jsonLDRestaurant
			if err := json.Unmarshal(raw2, &jr); err != nil {
				continue
			}
			if r.Name == "" {
				r.Name = jr.Name
			}
			if r.Description == "" {
				r.Description = stripAndCollapse(jr.Description)
			}
			if r.Address == "" || r.City == "" {
				street, city := decodeAddress(jr.Address)
				if r.Address == "" {
					r.Address = street
				}
				if r.City == "" {
					r.City = city
				}
			}
			if jr.Geo != nil {
				if r.Lat == 0 {
					r.Lat = numberAsFloat(jr.Geo.Latitude)
				}
				if r.Lng == 0 {
					r.Lng = numberAsFloat(jr.Geo.Longitude)
				}
			}
		}
	}

	// Fallbacks for fields JSON-LD didn't supply.
	if r.Name == "" {
		r.Name = stripAndCollapse(extractFirstTag(html, "h1"))
	}
	if r.Description == "" {
		r.Description = stripAndCollapse(extractFirstTag(html, "p"))
	}
	if r.Neighborhood == "" {
		r.Neighborhood = extractMetaContent(html, "neighborhood")
	}
	return r
}

// flattenJSONLD walks the heterogeneous shape of a JSON-LD block and returns a
// flat list of `{...}` objects. Pages may use `@graph: [...]` arrays.
func flattenJSONLD(v any) []map[string]any {
	switch tv := v.(type) {
	case map[string]any:
		if g, ok := tv["@graph"].([]any); ok {
			out := make([]map[string]any, 0, len(g))
			for _, sub := range g {
				out = append(out, flattenJSONLD(sub)...)
			}
			return out
		}
		return []map[string]any{tv}
	case []any:
		var out []map[string]any
		for _, sub := range tv {
			out = append(out, flattenJSONLD(sub)...)
		}
		return out
	}
	return nil
}

// typeMatches accepts either a string or []any for @type and returns true if
// any of the wanted types appears.
func typeMatches(got any, wanted ...string) bool {
	is := func(s string) bool {
		for _, w := range wanted {
			if strings.EqualFold(s, w) {
				return true
			}
		}
		return false
	}
	switch tv := got.(type) {
	case string:
		return is(tv)
	case []any:
		for _, x := range tv {
			if s, ok := x.(string); ok && is(s) {
				return true
			}
		}
	}
	return false
}

func decodeAddress(raw json.RawMessage) (street, city string) {
	if len(raw) == 0 {
		return
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, ""
	}
	var p postalAddress
	if err := json.Unmarshal(raw, &p); err == nil {
		parts := []string{}
		if p.StreetAddress != "" {
			parts = append(parts, p.StreetAddress)
		}
		if p.PostalCode != "" {
			parts = append(parts, p.PostalCode)
		}
		street = strings.TrimSpace(strings.Join(parts, ", "))
		city = p.AddressLocality
		return
	}
	return
}

// numberAsFloat handles JSON-LD Geo coordinates that may arrive as numbers or
// quoted strings (Le Fooding has emitted both).
func numberAsFloat(v any) float64 {
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

func stripAndCollapse(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// stripAndCollapseWithSep replaces every HTML tag with sep before collapsing
// runs of whitespace. Use this when the tag boundary itself carries meaning
// (e.g. <h3>Name</h3><span>City</span> should split on tag boundary, not on
// the absence of inline whitespace).
func stripAndCollapseWithSep(s, sep string) string {
	s = tagStripRe.ReplaceAllString(s, sep)
	// Collapse runs of the separator and surrounding whitespace.
	pattern := regexp.MustCompile(`\s*` + regexp.QuoteMeta(sep) + `(?:\s*` + regexp.QuoteMeta(sep) + `)*\s*`)
	s = pattern.ReplaceAllString(s, sep)
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.Trim(s, sep+" "))
}

func extractFirstTag(html, tag string) string {
	re := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `[^>]*>([\s\S]+?)</` + regexp.QuoteMeta(tag) + `>`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractMetaContent looks for <meta property="…neighborhood…" content="…"> or
// <meta name="…neighborhood…" content="…">. Used as a low-cost fallback.
func extractMetaContent(html, key string) string {
	re := regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["'][^"']*` + regexp.QuoteMeta(key) + `[^"']*["'][^>]+content=["']([^"']+)["']`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
