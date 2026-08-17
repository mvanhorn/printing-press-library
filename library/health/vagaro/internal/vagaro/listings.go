// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Metro-level business discovery from the SSR /listings/{service}/{location}
// page. The page embeds a schema.org ItemList as application/ld+json; each
// itemListElement.item is a LocalBusiness carrying the rating, price range,
// and address that the per-business websiteapi surface does not expose.

package vagaro

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
)

// ldJSONScriptRE captures the body of every <script type="application/ld+json">
// block. Non-greedy so multiple blocks on one page are captured separately.
var ldJSONScriptRE = regexp.MustCompile(`(?is)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)

// priceTextRE pulls the first decimal money value out of a "$52.00" / "$52" /
// "From $52.00" style price string.
var priceTextRE = regexp.MustCompile(`\d+(?:\.\d{1,2})?`)

// ListingBusiness is a metro business row parsed from the listings JSON-LD.
type ListingBusiness struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	URL         string  `json:"url,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	PriceRange  string  `json:"price_range,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	ReviewCount int     `json:"review_count,omitempty"`
	City        string  `json:"city,omitempty"`
	State       string  `json:"state,omitempty"`
	Address     string  `json:"address,omitempty"`
	Category    string  `json:"category,omitempty"`
}

// jsonLDAddress mirrors the schema.org PostalAddress shape.
type jsonLDAddress struct {
	StreetAddress   string `json:"streetAddress"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
	PostalCode      string `json:"postalCode"`
}

// jsonLDRating mirrors the schema.org AggregateRating shape. ratingValue and
// ratingCount arrive as either JSON numbers or numeric strings, so both are
// decoded as json.Number.
type jsonLDRating struct {
	RatingValue json.Number `json:"ratingValue"`
	RatingCount json.Number `json:"ratingCount"`
	ReviewCount json.Number `json:"reviewCount"`
}

// jsonLDBusiness is a schema.org LocalBusiness node.
type jsonLDBusiness struct {
	Type            string        `json:"@type"`
	Name            string        `json:"name"`
	URL             string        `json:"url"`
	Telephone       string        `json:"telephone"`
	PriceRange      string        `json:"priceRange"`
	Address         jsonLDAddress `json:"address"`
	AggregateRating jsonLDRating  `json:"aggregateRating"`
}

// Listings fetches the metro listings page for a service/location and parses
// the embedded JSON-LD into business rows. The location slug is advisory: the
// server geo-locates by the caller's IP, so results reflect the caller's metro
// regardless of the slug passed.
func (c *Client) Listings(ctx context.Context, service, location string) ([]ListingBusiness, error) {
	path := "/listings/" + strings.Trim(service, "/") + "/" + strings.Trim(location, "/")
	html, err := c.fetchHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	return ParseListings(html), nil
}

// ParseListings extracts LocalBusiness rows from the listings SSR HTML. It
// walks every ld+json block, descends ItemList.itemListElement[].item, and
// also accepts a bare array or a lone LocalBusiness object. De-duplicated by
// slug in first-seen order.
func ParseListings(html string) []ListingBusiness {
	out := make([]ListingBusiness, 0)
	seen := map[string]struct{}{}
	for _, m := range ldJSONScriptRE.FindAllStringSubmatch(html, -1) {
		block := strings.TrimSpace(m[1])
		if block == "" {
			continue
		}
		for _, b := range businessesFromLDBlock(block) {
			row := listingFromLD(b)
			if row.Slug == "" {
				continue
			}
			if _, ok := seen[row.Slug]; ok {
				continue
			}
			seen[row.Slug] = struct{}{}
			out = append(out, row)
		}
	}
	return out
}

// businessesFromLDBlock decodes one ld+json block into LocalBusiness nodes.
// The block is walked recursively so every observed nesting is handled: a bare
// array wrapping an ItemList, an ItemList object, ListItem wrappers, and lone
// LocalBusiness objects.
func businessesFromLDBlock(block string) []jsonLDBusiness {
	var out []jsonLDBusiness
	collectLDBusinesses(json.RawMessage(block), 0, &out)
	return out
}

// collectLDBusinesses recursively descends a JSON-LD value, appending every
// LocalBusiness leaf (an object carrying both name and url). Arrays and the
// itemListElement / item wrappers are traversed; depth is bounded defensively.
func collectLDBusinesses(raw json.RawMessage, depth int, out *[]jsonLDBusiness) {
	if depth > 6 {
		return
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return
	}
	switch trimmed[0] {
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(trimmed, &arr) == nil {
			for _, el := range arr {
				collectLDBusinesses(el, depth+1, out)
			}
		}
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(trimmed, &obj) != nil {
			return
		}
		if ile, ok := obj["itemListElement"]; ok {
			collectLDBusinesses(ile, depth+1, out)
			return
		}
		if item, ok := obj["item"]; ok {
			collectLDBusinesses(item, depth+1, out)
			return
		}
		var b jsonLDBusiness
		if json.Unmarshal(trimmed, &b) == nil && b.Name != "" && b.URL != "" {
			*out = append(*out, b)
		}
	}
}

func listingFromLD(b jsonLDBusiness) ListingBusiness {
	row := ListingBusiness{
		Name:       cliutil.CleanText(b.Name),
		URL:        strings.TrimSpace(b.URL),
		Slug:       slugFromURL(b.URL),
		Phone:      cliutil.CleanText(b.Telephone),
		PriceRange: strings.TrimSpace(b.PriceRange),
		City:       cliutil.CleanText(b.Address.AddressLocality),
		State:      strings.ToUpper(cliutil.CleanText(b.Address.AddressRegion)),
		Address:    joinAddress(b.Address),
	}
	if v, err := b.AggregateRating.RatingValue.Float64(); err == nil {
		row.Rating = v
	}
	row.ReviewCount = firstInt(b.AggregateRating.RatingCount, b.AggregateRating.ReviewCount)
	return row
}

func firstInt(nums ...json.Number) int {
	for _, n := range nums {
		if n == "" {
			continue
		}
		if v, err := n.Int64(); err == nil && v != 0 {
			return int(v)
		}
	}
	return 0
}

func joinAddress(a jsonLDAddress) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{a.StreetAddress, a.AddressLocality, a.AddressRegion, a.PostalCode} {
		if s := cliutil.CleanText(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// slugFromURL extracts the business slug (last non-empty path segment) from a
// Vagaro profile URL. Query strings and trailing slashes are stripped.
func slugFromURL(rawURL string) string {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return ""
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	return strings.TrimSpace(u)
}

// ParsePriceTextCents parses a price string like "$52.00", "52", or
// "From $52.00" into integer cents. Returns ok=false when no numeric value is
// present (e.g. "Free", "Varies", empty).
func ParsePriceTextCents(s string) (int, bool) {
	m := priceTextRE.FindString(s)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return int(f*100 + 0.5), true
}
