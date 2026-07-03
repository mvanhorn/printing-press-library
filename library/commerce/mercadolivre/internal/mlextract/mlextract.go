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
	CatalogID string  `json:"catalog_id"`
	Name      string  `json:"name"`
	Brand     string  `json:"brand"`
	Seller    string  `json:"seller"`
	Price     float64 `json:"price"`
	// OriginalPrice is the pre-discount ("previous") price when the card
	// advertises one; zero otherwise.
	OriginalPrice float64 `json:"original_price"`
	Currency      string  `json:"currency"`
	Availability  string  `json:"availability"`
	URL           string  `json:"url"`
	RatingValue   float64 `json:"rating_value"`
	RatingCount   int     `json:"rating_count"`
	// ShippingText is a free-form delivery hint scraped from the search card
	// (e.g. "Chegará grátis sábado"). It is NOT a day-count; the authoritative
	// delivery window still comes from ParseProduct on the product page.
	ShippingText string `json:"shipping_text"`
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
	// DeliveryMinDays/DeliveryMaxDays are the total shipping window in days
	// (handlingTime + transitTime), read from offers.shippingDetails. Zero
	// when the page carries no DAY-unit delivery estimate.
	DeliveryMinDays int    `json:"delivery_min_days"`
	DeliveryMaxDays int    `json:"delivery_max_days"`
	URL             string `json:"url"`
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
		"id":             l.CatalogID,
		"catalog_id":     l.CatalogID,
		"name":           l.Name,
		"brand":          l.Brand,
		"seller":         l.Seller,
		"price":          l.Price,
		"original_price": l.OriginalPrice,
		"currency":       l.Currency,
		"availability":   l.Availability,
		"url":            l.URL,
		"rating_value":   l.RatingValue,
		"rating_count":   l.RatingCount,
		"shipping_text":  l.ShippingText,
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
	Type            string              `json:"@type"`
	Availability    string              `json:"availability"`
	Price           flexFloat           `json:"price"`
	LowPrice        flexFloat           `json:"lowPrice"`
	HighPrice       flexFloat           `json:"highPrice"`
	PriceCurrency   string              `json:"priceCurrency"`
	URL             string              `json:"url"`
	ShippingDetails *rawShippingDetails `json:"shippingDetails"`
}

// rawShippingDetails mirrors offers.shippingDetails.deliveryTime, the only
// part of the shipping block the CLI reads (handling + transit windows).
type rawShippingDetails struct {
	DeliveryTime *rawDeliveryTime `json:"deliveryTime"`
}

type rawDeliveryTime struct {
	HandlingTime *rawQuantitativeValue `json:"handlingTime"`
	TransitTime  *rawQuantitativeValue `json:"transitTime"`
}

type rawQuantitativeValue struct {
	MinValue flexFloat `json:"minValue"`
	MaxValue flexFloat `json:"maxValue"`
	UnitCode string    `json:"unitCode"`
}

// resolvedOffer collapses object / array / AggregateOffer shapes into scalars.
type resolvedOffer struct {
	Price           float64
	Currency        string
	Availability    string
	URL             string
	DeliveryMinDays int
	DeliveryMaxDays int
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
	minDays, maxDays := deliveryDays(o.ShippingDetails)
	return resolvedOffer{
		Price:           price,
		Currency:        o.PriceCurrency,
		Availability:    StripAvailability(o.Availability),
		URL:             o.URL,
		DeliveryMinDays: minDays,
		DeliveryMaxDays: maxDays,
	}
}

// deliveryDays sums the handling and transit windows into a total (min, max)
// day range. Each component contributes only when its unitCode is "DAY";
// otherwise it adds nothing, so a non-DAY payload yields 0/0.
func deliveryDays(sd *rawShippingDetails) (int, int) {
	if sd == nil || sd.DeliveryTime == nil {
		return 0, 0
	}
	minDays, maxDays := 0, 0
	add := func(q *rawQuantitativeValue) {
		if q == nil || q.UnitCode != "DAY" {
			return
		}
		minDays += int(q.MinValue.val)
		maxDays += int(q.MaxValue.val)
	}
	add(sd.DeliveryTime.HandlingTime)
	add(sd.DeliveryTime.TransitTime)
	return minDays, maxDays
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

// ParseSearchListings is the primary search-page entry point. Real Mercado
// Livre search HTML (fetched over HTTP) carries NO JSON-LD @graph and no DOM
// poly-cards — those are JS-rendered. The results live only in a nordic/RSC
// SSR blob embedded as a JS string inside a `_n.ctx.s.q("<escaped-json>")`
// call. ParseSearchListings extracts the largest such blob, unescapes it, and
// scans it for `"polycard":{...}` objects (see parsePolycardBlob).
//
// If the polycard path yields zero listings (e.g. an older/test page that
// still ships a JSON-LD @graph), it falls back to parseSearchListingsLD,
// which parses the @graph via ParseSearchGraph and overlays DOM poly-card
// sellers. Callers that only need the JSON-LD fields can keep using
// ParseSearchGraph directly.
func ParseSearchListings(htmlBytes []byte) ([]Listing, error) {
	if blob, ok := extractNordicBlob(htmlBytes); ok {
		listings, err := parsePolycardBlob(blob)
		if err == nil && len(listings) > 0 {
			return listings, nil
		}
	}
	return parseSearchListingsLD(htmlBytes)
}

// parseSearchListingsLD is the JSON-LD fallback path: it parses the search
// page @graph (price/brand/rating/url/catalog_id via ParseSearchGraph) and
// overlays the seller name scraped from the DOM poly-cards. Sellers are merged
// onto listings by catalog_id, falling back to index order when a listing
// carries no catalog_id.
func parseSearchListingsLD(htmlBytes []byte) ([]Listing, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("mlextract: parsing search page: %w", err)
	}

	graphJSON := findLDJSONGraph(doc)
	var listings []Listing
	if len(graphJSON) > 0 {
		listings, err = ParseSearchGraph(graphJSON)
		if err != nil {
			return nil, err
		}
	}

	cards := parsePolyCards(doc)
	mergeSellers(listings, cards)
	return listings, nil
}

// --- polycard SSR-blob parsing ---------------------------------------------

// nordicBlobRE captures the JS-string argument (including its surrounding
// double quotes and escaped chars) of a `_n.ctx.s.q("...")` call.
var nordicBlobRE = regexp.MustCompile(`_n\.ctx\.s\.q\(("(?:\\.|[^"\\])*")\)`)

// extractNordicBlob finds every `_n.ctx.s.q("...")` call in the HTML,
// JSON-unescapes each quoted argument into its real string, and returns the
// LARGEST one (the product data blob). ok is false when no blob is present.
func extractNordicBlob(htmlBytes []byte) (string, bool) {
	matches := nordicBlobRE.FindAllSubmatch(htmlBytes, -1)
	var best string
	for _, m := range matches {
		var s string
		if err := json.Unmarshal(m[1], &s); err != nil {
			continue
		}
		if len(s) > len(best) {
			best = s
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// polycard mirrors one "polycard":{...} object inside the unescaped blob.
type polycard struct {
	Metadata   polycardMetadata    `json:"metadata"`
	Components []polycardComponent `json:"components"`
}

type polycardMetadata struct {
	ID            string `json:"id"`
	ProductID     string `json:"product_id"`
	UserProductID string `json:"user_product_id"`
	URL           string `json:"url"`
}

// polycardComponent is a discriminated union keyed on Type; only the field
// matching Type is populated by the blob.
type polycardComponent struct {
	Type            string             `json:"type"`
	Title           *polycardTitle     `json:"title"`
	Seller          *polycardSeller    `json:"seller"`
	ReviewCompacted *polycardReview    `json:"review_compacted"`
	Price           *polycardPrice     `json:"price"`
	ShippingV2      []polycardShipping `json:"shipping_v2"`
}

type polycardTitle struct {
	Text string `json:"text"`
}

type polycardSeller struct {
	Text   string          `json:"text"`
	Values []polycardValue `json:"values"`
}

type polycardValue struct {
	Key   string         `json:"key"`
	Type  string         `json:"type"`
	Label *polycardLabel `json:"label"`
}

type polycardLabel struct {
	Text string `json:"text"`
}

type polycardReview struct {
	AltText string `json:"alt_text"`
}

type polycardPrice struct {
	CurrentPrice *polycardPriceValue  `json:"current_price"`
	PriceLabels  []polycardPriceLabel `json:"price_labels"`
}

type polycardPriceValue struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type polycardPriceLabel struct {
	Values []polycardPriceLabelValue `json:"values"`
}

type polycardPriceLabelValue struct {
	Key   string              `json:"key"`
	Price *polycardPriceValue `json:"price"`
}

type polycardShipping struct {
	Values []polycardShippingValue `json:"values"`
}

type polycardShippingValue struct {
	Key  string        `json:"key"`
	Pill *polycardPill `json:"pill"`
}

type polycardPill struct {
	Text string `json:"text"`
}

const polycardMarker = `"polycard":`

// parsePolycardBlob scans an already-UNESCAPED nordic blob for each
// `"polycard":{...}` object, extracts the brace-balanced object after the
// marker, and maps it to a Listing. It does not attempt to parse the wider RSC
// reference graph.
func parsePolycardBlob(blob string) ([]Listing, error) {
	var out []Listing
	for idx := 0; ; {
		p := strings.Index(blob[idx:], polycardMarker)
		if p < 0 {
			break
		}
		start := idx + p + len(polycardMarker)
		obj, next, ok := extractBalancedObject(blob, start)
		if !ok {
			idx = start
			continue
		}
		idx = next
		var pc polycard
		if err := json.Unmarshal([]byte(obj), &pc); err != nil {
			continue
		}
		l := polycardToListing(pc)
		if l.CatalogID == "" && l.Name == "" {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

// extractBalancedObject returns the brace-balanced JSON object that begins at
// (or just after leading whitespace from) index from. It tracks in-string
// state and backslash escapes so that `{`/`}` inside quoted string values do
// not corrupt the brace count. It returns the object text, the index just past
// its closing brace, and ok=false when no balanced object is found.
func extractBalancedObject(s string, from int) (string, int, bool) {
	i := from
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		break
	}
	if i >= len(s) || s[i] != '{' {
		return "", from, false
	}
	depth := 0
	inStr := false
	esc := false
	for j := i; j < len(s); j++ {
		c := s[j]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[i : j+1], j + 1, true
			}
		}
	}
	return "", from, false
}

var ratingAltRE = regexp.MustCompile(`Classifica\x{00e7}\x{00e3}o\s+([0-9]+(?:[.,][0-9]+)?)\s+de\s+5`)

// polycardToListing maps one parsed polycard object onto a Listing.
func polycardToListing(pc polycard) Listing {
	l := Listing{Currency: "BRL"}

	id := strings.TrimSpace(pc.Metadata.ProductID)
	if id == "" {
		id = strings.TrimSpace(pc.Metadata.ID)
	}
	if id == "" {
		id = CatalogIDFromURL(pc.Metadata.URL)
	}
	l.CatalogID = id

	for _, c := range pc.Components {
		switch c.Type {
		case "title":
			if c.Title != nil {
				l.Name = cliutil.CleanText(c.Title.Text)
			}
		case "seller":
			if c.Seller != nil {
				l.Seller = cliutil.CleanText(polycardSellerLabel(c.Seller))
			}
		case "review_compacted":
			if c.ReviewCompacted != nil {
				l.RatingValue = ratingFromAltText(c.ReviewCompacted.AltText)
			}
		case "price":
			if c.Price != nil {
				if c.Price.CurrentPrice != nil {
					l.Price = c.Price.CurrentPrice.Value
					if cur := strings.TrimSpace(c.Price.CurrentPrice.Currency); cur != "" {
						l.Currency = cur
					}
				}
				l.OriginalPrice = polycardPreviousPrice(c.Price)
			}
		case "shipping_v2":
			l.ShippingText = polycardShippingText(c.ShippingV2)
		}
	}

	// Canonical catalog URL when a real product_id is present; otherwise fall
	// back to the metadata URL only when it is itself a product URL.
	if strings.TrimSpace(pc.Metadata.ProductID) != "" && l.CatalogID != "" {
		l.URL = "https://www.mercadolivre.com.br/p/" + l.CatalogID
	} else if u := normalizeProductURL(pc.Metadata.URL); u != "" {
		l.URL = u
	}
	return l
}

// polycardSellerLabel returns the seller's label text: the first values[] entry
// with key=="label", falling back to the first entry that carries a label.
func polycardSellerLabel(s *polycardSeller) string {
	for _, v := range s.Values {
		if v.Key == "label" && v.Label != nil {
			return v.Label.Text
		}
	}
	for _, v := range s.Values {
		if v.Label != nil {
			return v.Label.Text
		}
	}
	return ""
}

// ratingFromAltText pulls the numeric rating out of a review_compacted alt_text
// like "Classificação 4.4 de 5 estrelas...". Returns 0 when absent.
func ratingFromAltText(alt string) float64 {
	m := ratingAltRE.FindStringSubmatch(alt)
	if m == nil {
		return 0
	}
	s := strings.ReplaceAll(m[1], ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// polycardPreviousPrice returns the pre-discount price advertised in the price
// component's price_labels (values[].key=="previous_price"), or 0.
func polycardPreviousPrice(p *polycardPrice) float64 {
	for _, lbl := range p.PriceLabels {
		for _, v := range lbl.Values {
			if v.Key == "previous_price" && v.Price != nil {
				return v.Price.Value
			}
		}
	}
	return 0
}

// polycardShippingText returns the first non-empty shipping pill text (a
// free-form delivery hint), or "".
func polycardShippingText(arr []polycardShipping) string {
	for _, s := range arr {
		for _, v := range s.Values {
			if v.Pill != nil && strings.TrimSpace(v.Pill.Text) != "" {
				return cliutil.CleanText(v.Pill.Text)
			}
		}
	}
	return ""
}

// normalizeProductURL returns a usable https product URL when u points at a
// /p/ or /up/ catalog page (adding a scheme when the blob omits one), else "".
func normalizeProductURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if !strings.Contains(u, "/p/") && !strings.Contains(u, "/up/") {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	return u
}

// polyCard is one DOM search card: the catalog id from its title href and the
// seller label.
type polyCard struct {
	catalogID string
	seller    string
}

// findLDJSONGraph returns the text of the first <script type="application/
// ld+json"> whose body carries an "@graph" array (the search page graph). If no
// script mentions @graph, it returns the first ld+json script body.
func findLDJSONGraph(doc *html.Node) []byte {
	var first []byte
	var out []byte
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if out != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" && scriptIsLDJSON(n) {
			body := []byte(nodeText(n))
			if first == nil {
				first = body
			}
			if bytes.Contains(body, []byte("@graph")) {
				out = body
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if out != nil {
		return out
	}
	return first
}

func scriptIsLDJSON(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "type" && strings.EqualFold(strings.TrimSpace(a.Val), "application/ld+json") {
			return true
		}
	}
	return false
}

// parsePolyCards walks the DOM for div.poly-card nodes, pulling the catalog id
// (from the poly-component__title href) and the poly-component__seller text.
func parsePolyCards(doc *html.Node) []polyCard {
	var out []polyCard
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "poly-card") {
			out = append(out, polyCard{
				catalogID: CatalogIDFromURL(polyCardHref(n)),
				seller:    cliutil.CleanText(polyCardSeller(n)),
			})
			// Do not descend into a card we already captured.
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

// polyCardHref returns the href of the first a.poly-component__title under card.
func polyCardHref(card *html.Node) string {
	var href string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if href != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "poly-component__title") {
			for _, a := range n.Attr {
				if a.Key == "href" {
					href = a.Val
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	return href
}

// polyCardSeller returns the text of the first .poly-component__seller element
// under card.
func polyCardSeller(card *html.Node) string {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && hasClass(n, "poly-component__seller") {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	if found == nil {
		return ""
	}
	return nodeText(found)
}

// mergeSellers overlays the poly-card seller onto each listing: by catalog_id
// first, then by index order for listings without a matching id.
func mergeSellers(listings []Listing, cards []polyCard) {
	sellerByID := make(map[string]string, len(cards))
	for _, c := range cards {
		if c.catalogID != "" && c.seller != "" {
			sellerByID[c.catalogID] = c.seller
		}
	}
	for i := range listings {
		if listings[i].CatalogID != "" {
			if s, ok := sellerByID[listings[i].CatalogID]; ok {
				listings[i].Seller = s
				continue
			}
		}
		if i < len(cards) {
			listings[i].Seller = cards[i].seller
		}
	}
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
		CatalogID:       catalogID,
		Name:            cliutil.CleanText(pn.Name),
		Brand:           cliutil.CleanText(pn.Brand.Name),
		Price:           offer.Price,
		Currency:        offer.Currency,
		Availability:    offer.Availability,
		DeliveryMinDays: offer.DeliveryMinDays,
		DeliveryMaxDays: offer.DeliveryMaxDays,
		URL:             offer.URL,
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
