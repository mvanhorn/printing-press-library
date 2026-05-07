// Package tabelog scrapes Tabelog's English restaurant pages
// (https://tabelog.com/en/...). Per-restaurant pages embed schema.org
// Restaurant JSON-LD (name, address, geo, AggregateRating); we prefer parsing
// those blocks over walking the DOM.
package tabelog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://tabelog.com"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second

	// MinHighQualityRating is the Tabelog rating floor for "high-quality"
	// callers. Anything at or above this score is kept; everything below is
	// dropped from RestaurantsByPrefecture's filtered list.
	MinHighQualityRating = 3.5
)

// Restaurant is a flattened Tabelog entry. NameLocal preserves the Japanese
// title when the page emits both English and Japanese names.
type Restaurant struct {
	Name        string  `json:"name"`
	NameLocal   string  `json:"name_local,omitempty"`
	URL         string  `json:"url"`
	Address     string  `json:"address"`
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"rating_count"`
	Lat         float64 `json:"lat,omitempty"`
	Lng         float64 `json:"lng,omitempty"`
	Cuisine     string  `json:"cuisine,omitempty"`
	PriceRange  string  `json:"price_range,omitempty"`
}

// Client is a no-auth Tabelog scraper.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
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

func (c *Client) get(ctx context.Context, full string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en;q=0.9,ja;q=0.8")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tabelog request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tabelog read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", full, resp.StatusCode)
	}
	return body, nil
}

// RestaurantsByPrefecture fetches a Tabelog listing for a prefecture +
// optional cuisine slug and returns only restaurants whose rating meets the
// MinHighQualityRating bar (>= 3.5). Empty cuisineSlug fetches the prefecture
// landing page.
func (c *Client) RestaurantsByPrefecture(ctx context.Context, prefectureSlug, cuisineSlug string) ([]Restaurant, error) {
	if strings.TrimSpace(prefectureSlug) == "" {
		return nil, fmt.Errorf("tabelog: empty prefecture slug")
	}
	base := strings.TrimRight(c.BaseURL, "/")
	var full string
	if strings.TrimSpace(cuisineSlug) == "" {
		full = fmt.Sprintf("%s/en/%s/", base, prefectureSlug)
	} else {
		full = fmt.Sprintf("%s/en/%s/cuisine/%s/", base, prefectureSlug, cuisineSlug)
	}
	body, err := c.get(ctx, full)
	if err != nil {
		return nil, err
	}
	cards := parseListingCards(string(body))
	out := make([]Restaurant, 0, len(cards))
	for _, r := range cards {
		if r.Rating > 0 && r.Rating < MinHighQualityRating {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// Restaurant fetches a single Tabelog restaurant page and decodes JSON-LD.
func (c *Client) Restaurant(ctx context.Context, fullURL string) (*Restaurant, error) {
	if strings.TrimSpace(fullURL) == "" {
		return nil, fmt.Errorf("tabelog: empty url")
	}
	body, err := c.get(ctx, fullURL)
	if err != nil {
		return nil, err
	}
	r := parseRestaurantPage(string(body))
	if r.URL == "" {
		r.URL = fullURL
	}
	return r, nil
}

var (
	jsonLDRe = regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>([\s\S]+?)</script>`)

	// Card on a listing page: <a class="…list-rst__rst-name-target…" href="…">Name</a>.
	cardLinkRe = regexp.MustCompile(`(?is)<a[^>]+class=["'][^"']*list-rst__rst-name-target[^"']*["'][^>]+href=["']([^"']+)["'][^>]*>([\s\S]+?)</a>`)

	// Rating per card: <span class="…list-rst__rating-val…">3.62</span>.
	cardRatingRe = regexp.MustCompile(`(?is)<span[^>]+class=["'][^"']*list-rst__rating-val[^"']*["'][^>]*>([\s\S]+?)</span>`)

	// Address per card: <p class="…list-rst__area-genre…">Tokyo / Sushi</p>.
	cardAreaRe = regexp.MustCompile(`(?is)<(?:p|span)[^>]+class=["'][^"']*list-rst__area-genre[^"']*["'][^>]*>([\s\S]+?)</(?:p|span)>`)

	// Wrapper that pairs the link with its sibling rating/area in the same row.
	rowRe = regexp.MustCompile(`(?is)<li[^>]+class=["'][^"']*list-rst[^"']*["'][^>]*>([\s\S]+?)</li>`)

	tagStripRe   = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

func parseListingCards(html string) []Restaurant {
	out := make([]Restaurant, 0, 16)
	for _, m := range rowRe.FindAllStringSubmatch(html, -1) {
		row := m[1]
		var r Restaurant
		if lm := cardLinkRe.FindStringSubmatch(row); len(lm) == 3 {
			r.URL = strings.TrimSpace(lm[1])
			r.Name = stripAndCollapse(lm[2])
		}
		if r.Name == "" || r.URL == "" {
			continue
		}
		if rm := cardRatingRe.FindStringSubmatch(row); len(rm) == 2 {
			r.Rating = parseFloatLoose(stripAndCollapse(rm[1]))
		}
		if am := cardAreaRe.FindStringSubmatch(row); len(am) == 2 {
			area := stripAndCollapse(am[1])
			// Tabelog's list-rst__area-genre is "Area / Cuisine".
			if i := strings.Index(area, "/"); i >= 0 {
				r.Address = strings.TrimSpace(area[:i])
				r.Cuisine = strings.TrimSpace(area[i+1:])
			} else {
				r.Address = area
			}
		}
		out = append(out, r)
	}
	return out
}

// jsonLDRestaurant is the permissive view we apply to every JSON-LD block; we
// cherry-pick whichever block is the Restaurant.
type jsonLDRestaurant struct {
	Type            any             `json:"@type"`
	Name            string          `json:"name"`
	AlternateName   any             `json:"alternateName"`
	URL             string          `json:"url"`
	Address         json.RawMessage `json:"address"`
	PriceRange      string          `json:"priceRange"`
	ServesCuisine   any             `json:"servesCuisine"`
	AggregateRating *struct {
		RatingValue any `json:"ratingValue"`
		ReviewCount any `json:"reviewCount"`
		RatingCount any `json:"ratingCount"`
	} `json:"aggregateRating"`
	Geo *struct {
		Latitude  any `json:"latitude"`
		Longitude any `json:"longitude"`
	} `json:"geo"`
}

type postalAddress struct {
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
	PostalCode      string `json:"postalCode"`
}

func parseRestaurantPage(html string) *Restaurant {
	r := &Restaurant{}

	for _, m := range jsonLDRe.FindAllStringSubmatch(html, -1) {
		raw := strings.TrimSpace(m[1])
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue
		}
		for _, b := range flattenJSONLD(v) {
			if !typeMatches(b["@type"], "Restaurant", "FoodEstablishment", "LocalBusiness") {
				continue
			}
			raw2, _ := json.Marshal(b)
			var jr jsonLDRestaurant
			if err := json.Unmarshal(raw2, &jr); err != nil {
				continue
			}
			if r.Name == "" {
				r.Name = jr.Name
			}
			if r.NameLocal == "" {
				r.NameLocal = stringOrFirst(jr.AlternateName)
			}
			if r.URL == "" {
				r.URL = jr.URL
			}
			if r.Address == "" {
				r.Address = decodeAddress(jr.Address)
			}
			if r.PriceRange == "" {
				r.PriceRange = jr.PriceRange
			}
			if r.Cuisine == "" {
				r.Cuisine = stringOrFirst(jr.ServesCuisine)
			}
			if jr.AggregateRating != nil {
				if r.Rating == 0 {
					r.Rating = numberAsFloat(jr.AggregateRating.RatingValue)
				}
				if r.RatingCount == 0 {
					if c := numberAsInt(jr.AggregateRating.ReviewCount); c > 0 {
						r.RatingCount = c
					} else {
						r.RatingCount = numberAsInt(jr.AggregateRating.RatingCount)
					}
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
	return r
}

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

func decodeAddress(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var p postalAddress
	if err := json.Unmarshal(raw, &p); err == nil {
		parts := []string{}
		for _, x := range []string{p.StreetAddress, p.AddressLocality, p.AddressRegion, p.PostalCode} {
			if x != "" {
				parts = append(parts, x)
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func stringOrFirst(v any) string {
	switch tv := v.(type) {
	case string:
		return tv
	case []any:
		if len(tv) > 0 {
			if s, ok := tv[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

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

func numberAsInt(v any) int {
	switch tv := v.(type) {
	case float64:
		return int(tv)
	case string:
		// Tabelog occasionally emits "1,234" — strip thousands separators.
		s := strings.ReplaceAll(strings.TrimSpace(tv), ",", "")
		i, err := strconv.Atoi(s)
		if err == nil {
			return i
		}
	}
	return 0
}

func parseFloatLoose(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func stripAndCollapse(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
