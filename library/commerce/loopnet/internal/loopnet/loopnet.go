// Package loopnet holds the pure extraction and URL logic for the LoopNet
// CLI: turning LoopNet's server-rendered HTML (schema.org JSON-LD plus the
// property-facts table) into typed Go values, and building LoopNet search /
// detail URLs from CLI arguments. It has no I/O and no CLI dependency so it
// can be unit-tested in isolation.
package loopnet

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// Listing is a search-result-grain LoopNet record — the fields available
// from a search page without fetching the listing's detail page.
type Listing struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Address       string  `json:"address,omitempty"`
	City          string  `json:"city,omitempty"`
	State         string  `json:"state,omitempty"`
	Zip           string  `json:"zip,omitempty"`
	PropertyType  string  `json:"property_type,omitempty"`
	ListingType   string  `json:"listing_type,omitempty"`
	Location      string  `json:"location,omitempty"`
	Price         float64 `json:"price,omitempty"`
	PriceText     string  `json:"price_text,omitempty"`
	SizeSqft      float64 `json:"size_sqft,omitempty"`
	Description   string  `json:"description,omitempty"`
	BrokerName    string  `json:"broker_name,omitempty"`
	BrokerCompany string  `json:"broker_company,omitempty"`
	URL           string  `json:"url"`
	ImageURL      string  `json:"image_url,omitempty"`
}

// Property is a detail-grain LoopNet record — the full fact sheet pulled
// from a listing's detail page (schema.org JSON-LD + the facts table).
type Property struct {
	Listing
	PropertySubtype       string   `json:"property_subtype,omitempty"`
	SaleType              string   `json:"sale_type,omitempty"`
	PricePerSqft          float64  `json:"price_per_sqft,omitempty"`
	CapRate               float64  `json:"cap_rate,omitempty"`
	NOI                   float64  `json:"noi,omitempty"`
	YearBuilt             string   `json:"year_built,omitempty"`
	BuildingClass         string   `json:"building_class,omitempty"`
	Zoning                string   `json:"zoning,omitempty"`
	Stories               string   `json:"stories,omitempty"`
	Tenancy               string   `json:"tenancy,omitempty"`
	LotSize               string   `json:"lot_size,omitempty"`
	BuildingFAR           string   `json:"building_far,omitempty"`
	ImprovementAssessment float64  `json:"improvement_assessment,omitempty"`
	LandAssessment        float64  `json:"land_assessment,omitempty"`
	TotalAssessment       float64  `json:"total_assessment,omitempty"`
	ParcelNumbers         []string `json:"parcel_numbers,omitempty"`
	Highlights            []string `json:"highlights,omitempty"`
	BrokerPhone           string   `json:"broker_phone,omitempty"`
	DateModified          string   `json:"date_modified,omitempty"`
	Auction               bool     `json:"auction,omitempty"`
}

// SearchResult is one parsed LoopNet search-results page.
type SearchResult struct {
	Listings     []Listing `json:"listings"`
	TotalResults int       `json:"total_results,omitempty"`
	HasNextPage  bool      `json:"has_next_page,omitempty"`
}

var (
	reListingID = regexp.MustCompile(`/Listing/(?:[^/]+/)?(\d+)/?`)
	reMoney     = regexp.MustCompile(`\$\s?([0-9][0-9,]*(?:\.[0-9]+)?)`)
	rePercent   = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	reSqft      = regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]+)?)\s*SF\b`)
	reDigits    = regexp.MustCompile(`[0-9][0-9,]*(?:\.[0-9]+)?`)
	reWS        = regexp.MustCompile(`\s+`)
	reParcel    = regexp.MustCompile(`[0-9][0-9-]+[0-9]`)
)

// distressKeywords are case-insensitive phrases in a listing's free-text
// description or sale type that signal a motivated seller. Matched
// deterministically — this is string search, not classification.
var distressKeywords = []string{
	"price reduced", "price reduction", "reduced price", "priced to sell",
	"motivated seller", "motivated owner", "must sell", "must be sold",
	"bring all offers", "bring offers", "all offers considered",
	"below market", "below appraisal", "below assessed",
	"distressed", "foreclosure", "bank owned", "bank-owned", "reo ",
	"short sale", "auction", "ten-x", "ten x", "bankruptcy",
	"estate sale", "liquidation", "as-is", "as is sale", "quick close",
	"owner financing", "seller financing", "value add", "value-add",
}

// ListingIDFromURL pulls the numeric LoopNet listing id out of a
// /Listing/.../<id>/ URL. Returns "" when no id is present.
func ListingIDFromURL(u string) string {
	m := reListingID.FindStringSubmatch(u)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// ParseMoney extracts the first dollar amount from a string, returning 0
// when none is present ("Price Upon Request" -> 0).
func ParseMoney(s string) float64 {
	m := reMoney.FindStringSubmatch(s)
	if len(m) == 2 {
		return toFloat(m[1])
	}
	return 0
}

// ParsePercent extracts the first percentage from a string (e.g. a cap rate).
func ParsePercent(s string) float64 {
	m := rePercent.FindStringSubmatch(s)
	if len(m) == 2 {
		return toFloat(m[1])
	}
	return 0
}

// ParseSqft extracts a building size in square feet from a string like
// "1,057 SF Office Building".
func ParseSqft(s string) float64 {
	m := reSqft.FindStringSubmatch(s)
	if len(m) == 2 {
		return toFloat(m[1])
	}
	return 0
}

func toFloat(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0
	}
	return f
}

func cleanText(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(s, " "))
}

// HasDistressSignal reports whether free text carries a motivated-seller
// keyword, and returns the first matched keyword.
func HasDistressSignal(text string) (bool, string) {
	lower := strings.ToLower(text)
	for _, kw := range distressKeywords {
		if strings.Contains(lower, kw) {
			return true, strings.TrimSpace(kw)
		}
	}
	return false, ""
}

// --- JSON-LD extraction -----------------------------------------------------

// ExtractJSONLD parses every <script type="application/ld+json"> block in
// the document and returns each decoded value. A block is dropped silently
// if it is not valid JSON.
func ExtractJSONLD(htmlBytes []byte) []any {
	doc, err := xhtml.Parse(strings.NewReader(string(htmlBytes)))
	if err != nil {
		return nil
	}
	var blocks []any
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, "script") {
			if strings.EqualFold(attr(n, "type"), "application/ld+json") {
				if text := rawText(n); strings.TrimSpace(text) != "" {
					var v any
					if json.Unmarshal([]byte(text), &v) == nil {
						blocks = append(blocks, v)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return blocks
}

// IsChallengePage reports whether an HTML body is an Akamai bot-challenge
// sensor page rather than real LoopNet content. Real LoopNet pages are large
// (>100 KB) and always carry schema.org JSON-LD; the sensor page is a few KB
// with neither. A small page with no JSON-LD is treated as a challenge.
func IsChallengePage(htmlBytes []byte) bool {
	if len(htmlBytes) >= 20000 {
		return false
	}
	return !strings.Contains(string(htmlBytes), "application/ld+json")
}

// ParseSearchHTML parses a LoopNet search-results page into a SearchResult.
// Listings come from the schema.org CollectionPage JSON-LD block; the total
// result count comes from JSON-LD numberOfItems or the .total-results-digits
// element; HasNextPage is true when a NextPage control is present.
func ParseSearchHTML(htmlBytes []byte) (*SearchResult, error) {
	res := &SearchResult{}
	for _, block := range ExtractJSONLD(htmlBytes) {
		obj, ok := block.(map[string]any)
		if !ok {
			continue
		}
		mainEntity, ok := obj["mainEntity"].(map[string]any)
		if !ok {
			continue
		}
		items, ok := mainEntity["itemListElement"].([]any)
		if !ok {
			continue
		}
		for _, raw := range items {
			item, ok := raw.(map[string]any)
			if !ok || !typeContains(item["@type"], "RealEstateListing") {
				continue
			}
			l := listingFromJSONLD(item)
			if l.URL == "" && l.Name == "" {
				continue
			}
			res.Listings = append(res.Listings, l)
		}
		if n, ok := mainEntity["numberOfItems"].(float64); ok && int(n) > res.TotalResults {
			res.TotalResults = int(n)
		}
	}
	if res.TotalResults == 0 {
		res.TotalResults = totalResultsFromHTML(htmlBytes)
	}
	res.HasNextPage = strings.Contains(string(htmlBytes), `data-automation-id="NextPage"`)
	if len(res.Listings) == 0 {
		return res, fmt.Errorf("no listings found on search page (no RealEstateListing JSON-LD)")
	}
	return res, nil
}

func listingFromJSONLD(item map[string]any) Listing {
	l := Listing{
		Name:        cleanText(asString(item["name"])),
		Description: cleanText(asString(item["description"])),
		URL:         asString(item["url"]),
		ImageURL:    imageURL(item["image"]),
	}
	l.ID = ListingIDFromURL(l.URL)
	l.Address = l.Name
	l.SizeSqft = ParseSqft(l.Description)
	for _, off := range asSlice(item["offers"]) {
		offer, ok := off.(map[string]any)
		if !ok {
			continue
		}
		if p := asFloat(offer["price"]); p > 0 && l.Price == 0 {
			l.Price = p
		}
		if by, ok := offer["offeredBy"].(map[string]any); ok {
			if l.BrokerName == "" {
				l.BrokerName = cleanText(asString(by["name"]))
			}
			if org, ok := by["worksFor"].(map[string]any); ok && l.BrokerCompany == "" {
				l.BrokerCompany = cleanText(asString(org["name"]))
			}
		}
	}
	if l.Price > 0 {
		l.PriceText = formatMoney(l.Price)
	} else if strings.Contains(strings.ToLower(l.Description), "upon request") {
		l.PriceText = "Price Upon Request"
	}
	return l
}

// ParseDetailHTML parses a LoopNet listing detail page into a Property,
// combining the RealEstateListing/Product JSON-LD block with the
// data-fact-type facts table (for tax assessments, parcels, FAR, cap rate).
func ParseDetailHTML(htmlBytes []byte) (*Property, error) {
	var ld map[string]any
	for _, block := range ExtractJSONLD(htmlBytes) {
		obj, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if typeContains(obj["@type"], "RealEstateListing") {
			ld = obj
			break
		}
	}
	if ld == nil {
		return nil, fmt.Errorf("no RealEstateListing JSON-LD found on detail page")
	}

	p := &Property{}
	p.Name = cleanText(asString(ld["name"]))
	p.Description = cleanText(asString(ld["description"]))
	p.URL = asString(ld["url"])
	p.ID = ListingIDFromURL(p.URL)
	p.ImageURL = imageURL(ld["image"])
	p.DateModified = asString(ld["dateModified"])

	if loc, ok := ld["contentLocation"].(map[string]any); ok {
		if addr, ok := loc["address"].(map[string]any); ok {
			p.Address = cleanText(asString(addr["streetAddress"]))
			p.City = cleanText(asString(addr["addressLocality"]))
			p.State = cleanText(asString(addr["addressRegion"]))
			p.Zip = cleanText(asString(addr["postalCode"]))
		}
	}
	if p.Address == "" {
		p.Address = p.Name
	}
	for _, off := range asSlice(ld["offers"]) {
		offer, ok := off.(map[string]any)
		if !ok {
			continue
		}
		if v := asFloat(offer["price"]); v > 0 && p.Price == 0 {
			p.Price = v
		}
	}
	for _, prov := range asSlice(ld["provider"]) {
		agent, ok := prov.(map[string]any)
		if !ok {
			continue
		}
		if p.BrokerName == "" {
			p.BrokerName = cleanText(asString(agent["name"]))
		}
		if org, ok := agent["memberOf"].(map[string]any); ok && p.BrokerCompany == "" {
			p.BrokerCompany = cleanText(asString(org["name"]))
		}
		if p.BrokerPhone == "" {
			p.BrokerPhone = cleanText(asString(agent["telephone"]))
		}
	}
	// additionalProperty[] is a list of {name, value} schema.org PropertyValue.
	for _, ap := range asSlice(ld["additionalProperty"]) {
		pv, ok := ap.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(cleanText(asString(pv["name"])))
		val := propertyValue(pv["value"])
		switch {
		case name == "price per sf" && p.PricePerSqft == 0:
			p.PricePerSqft = ParseMoney(val)
		case name == "sale type":
			p.SaleType = val
		case name == "property type" && p.PropertyType == "":
			p.PropertyType = val
		case name == "property subtype" && p.PropertySubtype == "":
			p.PropertySubtype = val
		case name == "building class":
			p.BuildingClass = val
		case name == "zoning":
			p.Zoning = val
		case name == "year built", name == "year built/renovated":
			p.YearBuilt = val
		case name == "no. stories", name == "stories":
			p.Stories = val
		case name == "tenancy":
			p.Tenancy = val
		case name == "lot size":
			p.LotSize = val
		case strings.Contains(name, "rentable building area"), name == "building size":
			if p.SizeSqft == 0 {
				p.SizeSqft = ParseSqft(val)
			}
		case name == "cap rate" && p.CapRate == 0:
			p.CapRate = ParsePercent(val)
		}
	}

	// The facts table fills in what JSON-LD omits: cap rate / NOI on
	// investment listings, FAR, parcel numbers, and tax assessments.
	facts := parseFactTable(htmlBytes)
	if p.CapRate == 0 {
		p.CapRate = ParsePercent(facts["CapRate"])
	}
	if v := ParseMoney(facts["NOI"]); v > 0 {
		p.NOI = v
	}
	p.BuildingFAR = cleanText(facts["BuildingFAR"])
	p.ImprovementAssessment = ParseMoney(facts["ImprovementAssessment"])
	p.LandAssessment = ParseMoney(facts["LandAssessment"])
	p.TotalAssessment = ParseMoney(facts["TotalAssessment"])
	if pn := facts["ParcelNumber"]; pn != "" {
		p.ParcelNumbers = reParcel.FindAllString(pn, -1)
	}
	if p.YearBuilt == "" {
		p.YearBuilt = digitsOnly(facts["YearBuiltRenovated"])
	}

	if p.Price > 0 {
		p.PriceText = formatMoney(p.Price)
	} else if strings.Contains(strings.ToLower(p.Description), "upon request") {
		p.PriceText = "Price Upon Request"
	}
	if p.SizeSqft == 0 {
		p.SizeSqft = ParseSqft(p.Description)
	}
	auctionText := strings.ToLower(p.SaleType + " " + p.URL + " " + p.Description)
	p.Auction = strings.Contains(auctionText, "auction") || strings.Contains(auctionText, "ten-x")
	return p, nil
}

// parseFactTable collects every [data-fact-type] cell on a detail page and,
// per fact type, keeps the cell text that carries the value (a $ amount, a
// percentage, or digits) rather than the label cell.
func parseFactTable(htmlBytes []byte) map[string]string {
	doc, err := xhtml.Parse(strings.NewReader(string(htmlBytes)))
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			if ft := attr(n, "data-fact-type"); ft != "" {
				txt := cleanText(rawText(n))
				if txt != "" && betterFactValue(out[ft], txt) {
					out[ft] = txt
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

// betterFactValue reports whether candidate is a more value-like cell text
// than current — preferring cells that contain a number/$/%.
func betterFactValue(current, candidate string) bool {
	if current == "" {
		return true
	}
	curHas := reDigits.MatchString(current)
	candHas := reDigits.MatchString(candidate)
	if candHas && !curHas {
		return true
	}
	if !candHas && curHas {
		return false
	}
	// Both or neither carry digits: prefer the shorter (value cells like
	// "$47,344" beat label+value mashes like "Improvements Assessment ...").
	return len(candidate) < len(current)
}

// --- helpers ----------------------------------------------------------------

func typeContains(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, want)
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.EqualFold(s, want) {
				return true
			}
		}
	}
	return false
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		return toFloat(n)
	}
	return 0
}

func asSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		return []any{t}
	}
	return nil
}

// propertyValue renders a schema.org PropertyValue "value" field, which may
// be a scalar, a string, or a single-element array.
func propertyValue(v any) string {
	switch t := v.(type) {
	case string:
		return cleanText(t)
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%.2f", t), ".00")
	case []any:
		var parts []string
		for _, e := range t {
			if s := asString(e); s != "" {
				parts = append(parts, cleanText(s))
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func imageURL(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return asString(t["url"])
	case []any:
		for _, e := range t {
			if s := imageURL(e); s != "" {
				return s
			}
		}
	}
	return ""
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func rawText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			b.WriteString(node.Data)
			b.WriteString(" ")
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func digitsOnly(s string) string {
	m := reDigits.FindString(s)
	return m
}

func formatMoney(f float64) string {
	n := int64(f)
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return "$" + string(out)
}

func totalResultsFromHTML(htmlBytes []byte) int {
	s := string(htmlBytes)
	idx := strings.Index(s, "total-results-digits")
	if idx < 0 {
		return 0
	}
	window := s[idx:min(idx+200, len(s))]
	m := reDigits.FindString(window)
	if m == "" {
		return 0
	}
	return int(toFloat(m))
}
