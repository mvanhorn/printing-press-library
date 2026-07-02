// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

// Package mlextract turns raw Mercado Livre HTML/JSON-LD payloads into the
// structured rows the CLI stores and compares. It is pure logic: no network,
// no store, no cobra. Everything is grounded on the fixtures in testdata/.
package mlextract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/cliutil"
	"golang.org/x/net/html"
)

// Listing is one search-result row extracted from the search page @graph.
// JSON tags match the store's listings-table columns so a slice of these can
// be marshaled straight into UpsertBatch after StoreJSON adds the id.
type Listing struct {
	CatalogID    string  `json:"catalog_id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	Availability string  `json:"availability"`
	URL          string  `json:"url"`
	RatingValue  float64 `json:"rating_value"`
	RatingCount  int     `json:"rating_count"`
}

// ProductDetail is a single product page's JSON-LD Product, flattened.
type ProductDetail struct {
	CatalogID    string  `json:"catalog_id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	Availability string  `json:"availability"`
	RatingValue  float64 `json:"rating_value"`
	RatingCount  int     `json:"rating_count"`
	ReviewCount  int     `json:"review_count"`
	URL          string  `json:"url"`
}

// Attribute is one technical-spec row (th/td pair) from the product page.
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StoreJSON renders a Listing as the store record shape, adding the "id"
// field the generic upsert path keys on ({"id":..., "catalog_id":..., ...}).
func (l Listing) StoreJSON() json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"id":           l.CatalogID,
		"catalog_id":   l.CatalogID,
		"name":         l.Name,
		"brand":        l.Brand,
		"price":        l.Price,
		"currency":     l.Currency,
		"availability": l.Availability,
		"url":          l.URL,
		"rating_value": l.RatingValue,
		"rating_count": l.RatingCount,
	})
	return b
}

// StoreJSON renders a ProductDetail as the store record shape (adds "id").
func (p ProductDetail) StoreJSON() json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"id":           p.CatalogID,
		"catalog_id":   p.CatalogID,
		"name":         p.Name,
		"brand":        p.Brand,
		"price":        p.Price,
		"currency":     p.Currency,
		"availability": p.Availability,
		"rating_value": p.RatingValue,
		"rating_count": p.RatingCount,
		"review_count": p.ReviewCount,
		"url":          p.URL,
	})
	return b
}

// --- flexible JSON scalars -------------------------------------------------

// flexFloat unmarshals a JSON number OR a JSON-encoded numeric string.
// schema.org offers frequently ship price as "1499.90" (a string); a plain
// float64 field would silently zero on those payloads.
type flexFloat struct {
	val float64
	set bool
}

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		return nil
	}
	s = strings.Trim(s, `"`)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Tolerate comma decimals (pt-BR): "1.499,90" -> "1499.90".
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // tolerate junk rather than failing the whole decode
	}
	f.val, f.set = v, true
	return nil
}

// flexBrand unmarshals brand as either {"name":"Asus"} or the bare string "Asus".
type flexBrand struct {
	Name string
}

func (b *flexBrand) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '{' {
		var obj struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil
		}
		b.Name = obj.Name
		return nil
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		b.Name = s
	}
	return nil
}

type rawRating struct {
	RatingValue flexFloat `json:"ratingValue"`
	RatingCount flexFloat `json:"ratingCount"`
	ReviewCount flexFloat `json:"reviewCount"`
}

type rawOffer struct {
	Type          string    `json:"@type"`
	Availability  string    `json:"availability"`
	Price         flexFloat `json:"price"`
	LowPrice      flexFloat `json:"lowPrice"`
	HighPrice     flexFloat `json:"highPrice"`
	PriceCurrency string    `json:"priceCurrency"`
	URL           string    `json:"url"`
}

// resolvedOffer collapses object / array / AggregateOffer shapes into scalars.
type resolvedOffer struct {
	Price        float64
	Currency     string
	Availability string
	URL          string
}

// parseOffers accepts the "offers" value in any of the three shapes the ML
// JSON-LD emits: a single Offer object, an array of Offers (lowest price
// wins), or an AggregateOffer with lowPrice/highPrice.
func parseOffers(raw json.RawMessage) resolvedOffer {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return resolvedOffer{}
	}
	if trimmed[0] == '[' {
		var arr []rawOffer
		if err := json.Unmarshal(trimmed, &arr); err != nil || len(arr) == 0 {
			return resolvedOffer{}
		}
		best := offerToResolved(arr[0])
		for _, o := range arr[1:] {
			r := offerToResolved(o)
			if r.Price > 0 && (best.Price == 0 || r.Price < best.Price) {
				best = r
			}
		}
		return best
	}
	var o rawOffer
	if err := json.Unmarshal(trimmed, &o); err != nil {
		return resolvedOffer{}
	}
	return offerToResolved(o)
}

func offerToResolved(o rawOffer) resolvedOffer {
	price := 0.0
	switch {
	case o.Price.set:
		price = o.Price.val
	case o.LowPrice.set:
		price = o.LowPrice.val
	case o.HighPrice.set:
		price = o.HighPrice.val
	}
	return resolvedOffer{
		Price:        price,
		Currency:     o.PriceCurrency,
		Availability: StripAvailability(o.Availability),
		URL:          o.URL,
	}
}

// --- exported extraction ---------------------------------------------------

type searchGraphEnvelope struct {
	Graph []json.RawMessage `json:"@graph"`
}

type rawProductNode struct {
	Type            string          `json:"@type"`
	Name            string          `json:"name"`
	SKU             string          `json:"sku"`
	Brand           flexBrand       `json:"brand"`
	AggregateRating *rawRating      `json:"aggregateRating"`
	Offers          json.RawMessage `json:"offers"`
}

// ParseSearchGraph parses the search page @graph, keeping only @type=="Product"
// nodes and mapping each to a Listing. SearchResultsPage and any other node
// type is skipped.
func ParseSearchGraph(raw []byte) ([]Listing, error) {
	var env searchGraphEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("mlextract: decoding search @graph: %w", err)
	}
	var out []Listing
	for _, node := range env.Graph {
		var pn rawProductNode
		if err := json.Unmarshal(node, &pn); err != nil {
			continue
		}
		if pn.Type != "Product" {
			continue
		}
		offer := parseOffers(pn.Offers)
		l := Listing{
			CatalogID:    CatalogIDFromURL(offer.URL),
			Name:         cliutil.CleanText(pn.Name),
			Brand:        cliutil.CleanText(pn.Brand.Name),
			Price:        offer.Price,
			Currency:     offer.Currency,
			Availability: offer.Availability,
			URL:          offer.URL,
		}
		if pn.AggregateRating != nil {
			l.RatingValue = pn.AggregateRating.RatingValue.val
			l.RatingCount = int(pn.AggregateRating.RatingCount.val)
		}
		out = append(out, l)
	}
	return out, nil
}

// ParseProduct parses a single product-page JSON-LD Product node.
func ParseProduct(raw []byte) (*ProductDetail, error) {
	var pn rawProductNode
	if err := json.Unmarshal(raw, &pn); err != nil {
		return nil, fmt.Errorf("mlextract: decoding product: %w", err)
	}
	offer := parseOffers(pn.Offers)
	catalogID := CatalogIDFromURL(offer.URL)
	if catalogID == "" {
		catalogID = strings.TrimSpace(pn.SKU)
	}
	p := &ProductDetail{
		CatalogID:    catalogID,
		Name:         cliutil.CleanText(pn.Name),
		Brand:        cliutil.CleanText(pn.Brand.Name),
		Price:        offer.Price,
		Currency:     offer.Currency,
		Availability: offer.Availability,
		URL:          offer.URL,
	}
	if pn.AggregateRating != nil {
		p.RatingValue = pn.AggregateRating.RatingValue.val
		p.RatingCount = int(pn.AggregateRating.RatingCount.val)
		p.ReviewCount = int(pn.AggregateRating.ReviewCount.val)
	}
	return p, nil
}

// ParseSpecTable extracts th/td attribute pairs from the product technical
// spec table. Rows are identified by the andes-table__row class on <tr>.
func ParseSpecTable(htmlBytes []byte) ([]Attribute, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("mlextract: parsing spec table: %w", err)
	}
	var out []Attribute
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" && hasClass(n, "andes-table__row") {
			name := firstCellText(n, "th")
			value := firstCellText(n, "td")
			if name != "" {
				out = append(out, Attribute{Name: name, Value: value})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out, nil
}

// firstCellText returns the CleanText of the first descendant element named
// tag (th or td) under row.
func firstCellText(row *html.Node, tag string) string {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	if found == nil {
		return ""
	}
	return cliutil.CleanText(nodeText(found))
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, f := range strings.Fields(a.Val) {
				if f == class {
					return true
				}
			}
		}
	}
	return false
}

// --- shared helpers --------------------------------------------------------

var catalogSegmentRE = regexp.MustCompile(`(?i)/(?:p|up)/(MLB[A-Z0-9]+)`)

// CatalogIDFromURL pulls the catalog id from a Mercado Livre product URL. It
// matches the segment after /p/ or /up/ (e.g. .../p/MLB51764304 ->
// MLB51764304, .../up/MLBU3664751795 -> MLBU3664751795). Falls back to the
// last path segment when no /p//up/ marker is present.
func CatalogIDFromURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if m := catalogSegmentRE.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	// Fallback: last path segment, stripped of query/fragment.
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

// StripAvailability turns "https://schema.org/InStock" into "InStock". A bare
// token passes through unchanged.
func StripAvailability(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

var leadingNumberRE = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// ExtractLeadingNumber pulls the first numeric token out of a free-form spec
// value string ("700 W" -> 700, "2560 px x 1600 px" -> 2560, "1 TB" -> 1).
// Returns ok=false when the string carries no number. A comma decimal
// separator is normalized to a dot.
func ExtractLeadingNumber(s string) (float64, bool) {
	m := leadingNumberRE.FindString(s)
	if m == "" {
		return 0, false
	}
	if strings.Contains(m, ",") && !strings.Contains(m, ".") {
		m = strings.ReplaceAll(m, ",", ".")
	} else {
		// A comma alongside a dot is a thousands separator in this locale;
		// drop it ("1,499.90" -> "1499.90").
		m = strings.ReplaceAll(m, ",", "")
	}
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
