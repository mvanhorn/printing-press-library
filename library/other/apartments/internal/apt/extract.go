package apt

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/cliutil"

	"golang.org/x/net/html"
)

// Placard summarises one listing card from a search-results page. The
// SearchSlug field is populated by callers (the slug they searched
// against) — extraction itself never sets it.
type Placard struct {
	URL        string  `json:"url"`
	PropertyID string  `json:"property_id"`
	Title      string  `json:"title,omitempty"`
	Beds       int     `json:"beds,omitempty"`
	Baths      float64 `json:"baths,omitempty"`
	MaxRent    int     `json:"max_rent,omitempty"`
	SearchSlug string  `json:"search_slug,omitempty"`
}

// Address mirrors the schema.org PostalAddress fields we read from
// listing detail pages.
type Address struct {
	StreetAddress string `json:"street_address,omitempty"`
	City          string `json:"city,omitempty"`
	State         string `json:"state,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
}

// FloorPlan is one rentable plan within a single listing. apartments.com
// pages may carry several (Studio / 1 BR / 2 BR ranges).
type FloorPlan struct {
	Name    string  `json:"name,omitempty"`
	Beds    int     `json:"beds,omitempty"`
	Baths   float64 `json:"baths,omitempty"`
	Sqft    int     `json:"sqft,omitempty"`
	RentMin int     `json:"rent_min,omitempty"`
	RentMax int     `json:"rent_max,omitempty"`
}

// PetPolicy is a best-effort capture: apartments.com doesn't expose a
// stable schema.org or microdata target for the pet block, so most
// fields are zero unless callers compute them from amenities text.
type PetPolicy struct {
	AllowsCats  bool `json:"allows_cats,omitempty"`
	AllowsDogs  bool `json:"allows_dogs,omitempty"`
	PetRent     int  `json:"pet_rent,omitempty"`
	PetDeposit  int  `json:"pet_deposit,omitempty"`
	PetFee      int  `json:"pet_fee,omitempty"`
	WeightLimit int  `json:"weight_limit,omitempty"`
}

// LeaseInfo carries minimum/maximum lease length when present.
type LeaseInfo struct {
	MinMonths int `json:"min_months,omitempty"`
	MaxMonths int `json:"max_months,omitempty"`
}

// Listing is the extracted shape of one apartments.com detail page.
type Listing struct {
	URL         string      `json:"url"`
	PropertyID  string      `json:"property_id"`
	Title       string      `json:"title,omitempty"`
	Address     Address     `json:"address"`
	Beds        int         `json:"beds,omitempty"`
	Baths       float64     `json:"baths,omitempty"`
	MaxRent     int         `json:"max_rent,omitempty"`
	Sqft        int         `json:"sqft,omitempty"`
	Amenities   []string    `json:"amenities,omitempty"`
	FloorPlans  []FloorPlan `json:"floor_plans,omitempty"`
	Photos      []string    `json:"photos,omitempty"`
	Phone       string      `json:"phone,omitempty"`
	AvailableAt string      `json:"available_at,omitempty"`
	PetPolicy   PetPolicy   `json:"pet_policy"`
	LeaseInfo   LeaseInfo   `json:"lease_info"`
}

// ID extraction shared by both parsers: returns the last non-empty path
// segment of a URL. ListingURLToPropertyID is the public alias.
func lastPathSegment(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return ""
}

// ListingURLToPropertyID returns the apartments.com property-id slug for
// a listing URL: the last non-empty path segment. Empty on malformed
// input.
func ListingURLToPropertyID(u string) string { return lastPathSegment(u) }

// attr looks up an attribute on a node, case-insensitive. The empty
// string is returned for a missing attribute.
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// hasAttrSubstring reports whether n's attribute `key` contains the
// substring `needle` (case-insensitive). Used for class checks since
// apartments.com mixes class names like "placardTitle js-placardTitle".
func hasAttrSubstring(n *html.Node, key, needle string) bool {
	v := attr(n, key)
	if v == "" {
		return false
	}
	return strings.Contains(strings.ToLower(v), strings.ToLower(needle))
}

// hasClassAtom reports whether n's class attribute contains `needle`
// as a whole class atom (delimited by whitespace), case-insensitive.
// Use this for the canonical "placard" check; substring match is too
// loose because "placards" / "placardContainer" trigger false positives.
func hasClassAtom(n *html.Node, needle string) bool {
	v := strings.ToLower(attr(n, "class"))
	if v == "" {
		return false
	}
	needleLower := strings.ToLower(needle)
	for _, atom := range strings.Fields(v) {
		if atom == needleLower {
			return true
		}
	}
	return false
}

// nodeText concatenates the text content of an HTML subtree.
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
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

// closestWithDataAttr walks up from n looking for the nearest ancestor
// (inclusive of n itself) that carries a non-empty `key` attribute.
func closestWithDataAttr(n *html.Node, key string) *html.Node {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Type != html.ElementNode {
			continue
		}
		if attr(cur, key) != "" {
			return cur
		}
	}
	return nil
}

// firstDescendantWithDataAttr finds the first descendant of n with the
// given attribute set to a non-empty value. Returns nil when none.
func firstDescendantWithDataAttr(n *html.Node, key string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && attr(n, key) != "" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

// parseBedsValue maps the loose data-beds attribute (which may be
// "Studio", "0", "2", "2.0") to an int. "Studio" => 0.
func parseBedsValue(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if strings.EqualFold(v, "studio") {
		return 0
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return int(f)
	}
	return 0
}

func parseFloatAttr(v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return 0
}

func parseIntAttr(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	// Trim a leading '$' / commas, handle ranges like "1500-2200" by
	// taking the upper bound (max-rent semantics).
	v = strings.ReplaceAll(v, ",", "")
	v = strings.TrimPrefix(v, "$")
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = strings.TrimSpace(v[idx+1:])
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return int(f)
	}
	return 0
}

// resolveURL turns a possibly-relative href into an absolute URL using
// baseURL as the reference.
func resolveURL(href, baseURL string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// ParsePlacards walks a search-results HTML page and extracts placard
// summaries. Apartments.com renders each card as an `<article>` element
// whose class contains "placard"; URL + property ID live on data-url /
// data-listingid, and beds / price are inner text in `.bedTextBox` and
// `.priceTextBox`. We accept any element whose class contains "placard"
// (not just <article>) so the parser stays resilient to markup
// reshuffles. Caps at 60 placards per page. The legacy data-beds /
// data-baths / data-maxrent attribute fast-path is preserved for older
// HTML and for unit tests.
func ParsePlacards(htmlBytes []byte, baseURL string) ([]Placard, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, err
	}
	var out []Placard
	seen := map[string]bool{}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(out) >= 60 || n == nil {
			return
		}
		// Match elements whose class atoms include exactly "placard"
		// (apartments.com 2024+ markup) or anchors whose class atoms
		// include "placardtitle" (legacy/unit-test markup). Substring
		// match is too loose: "placards" (the ul wrapper),
		// "placardContainer", "placardCarouselImgCount", etc. all
		// contain the substring "placard" but are not card roots.
		isPlacardCard := n.Type == html.ElementNode && hasClassAtom(n, "placard")
		if isPlacardCard {
			href := attr(n, "data-url")
			if href == "" {
				href = attr(n, "href")
			}
			if href == "" {
				if a := firstAnchorWithHref(n); a != nil {
					href = attr(a, "href")
				}
			}
			abs := resolveURL(href, baseURL)
			if abs != "" && !seen[abs] {
				seen[abs] = true
				p := Placard{
					URL:        abs,
					PropertyID: lastPathSegment(abs),
				}
				if pid := attr(n, "data-listingid"); pid != "" {
					p.PropertyID = pid
				}
				p.Title = cliutil.CleanText(attr(n, "title"))
				if p.Title == "" {
					p.Title = innerTextByClassSubstr(n, "js-placardtitle")
				}
				if p.Title == "" {
					p.Title = cliutil.CleanText(attr(n, "data-streetaddress"))
				}
				// Legacy attribute fast-path (also exercised by tests).
				dataHost := n
				if attr(n, "data-beds") == "" && attr(n, "data-maxrent") == "" {
					if anc := closestWithDataAttr(n, "data-beds"); anc != nil {
						dataHost = anc
					} else if anc := closestWithDataAttr(n, "data-maxrent"); anc != nil {
						dataHost = anc
					}
				}
				p.Beds = parseBedsValue(attr(dataHost, "data-beds"))
				p.Baths = parseFloatAttr(attr(dataHost, "data-baths"))
				p.MaxRent = parseIntAttr(attr(dataHost, "data-maxrent"))
				// Modern fallback: read beds and maxRent from inner
				// .bedTextBox / .priceTextBox text. Apartments.com may
				// emit ranges ("1-2 Beds", "$1,199+"), so we handle
				// both upper-bound (max) extraction.
				if p.Beds == 0 {
					if t := innerTextByClassSubstr(n, "bedtextbox"); t != "" {
						p.Beds = parseBedsFromText(t)
					}
				}
				if p.MaxRent == 0 {
					if t := innerTextByClassSubstr(n, "pricetextbox"); t != "" {
						p.MaxRent = parsePriceFromText(t)
					}
				}
				out = append(out, p)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out, nil
}

// firstAnchorWithHref walks n's subtree and returns the first <a>
// carrying a non-empty href.
func firstAnchorWithHref(n *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "a") && attr(n, "href") != "" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

// innerTextByClassSubstr returns the trimmed text of the first
// descendant whose class contains needle (case-insensitive).
func innerTextByClassSubstr(n *html.Node, needle string) string {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Type == html.ElementNode && hasAttrSubstring(n, "class", needle) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if found == nil {
		return ""
	}
	return cliutil.CleanText(strings.TrimSpace(nodeText(found)))
}

// parseBedsFromText pulls a bed count out of text like "2 Beds",
// "Studio", "1-2 Beds". Returns the upper bound for ranges; returns 0
// for studios.
func parseBedsFromText(t string) int {
	low := strings.ToLower(t)
	if strings.Contains(low, "studio") {
		return 0
	}
	// Match the last integer before "bed".
	bed := strings.Index(low, "bed")
	if bed < 0 {
		return 0
	}
	prefix := low[:bed]
	// Find rightmost integer in prefix.
	digits := ""
	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c >= '0' && c <= '9' {
			digits = string(c) + digits
			continue
		}
		if digits != "" {
			break
		}
	}
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}

// parsePriceFromText pulls a max rent from text like "$1,199+",
// "$1,199 - $2,400". Returns the upper bound when a range is present;
// returns the leading number otherwise.
func parsePriceFromText(t string) int {
	clean := strings.ReplaceAll(t, ",", "")
	clean = strings.ReplaceAll(clean, "$", "")
	clean = strings.ReplaceAll(clean, "+", "")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0
	}
	// Range form "1199 - 2400" → take last number.
	if idx := strings.Index(clean, "-"); idx >= 0 {
		clean = strings.TrimSpace(clean[idx+1:])
	}
	// Strip trailing /mo or similar.
	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if (c < '0' || c > '9') && c != '.' {
			clean = clean[:i]
			break
		}
	}
	if clean == "" {
		return 0
	}
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return int(f)
}

// ParseListing walks a listing detail page and extracts schema.org
// microdata + data-* attributes. Best-effort: missing fields stay zero
// rather than erroring.
func ParseListing(htmlBytes []byte, listingURL string) (Listing, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return Listing{}, err
	}
	li := Listing{
		URL:        listingURL,
		PropertyID: lastPathSegment(listingURL),
	}

	// schema.org meta itemprop fields.
	var (
		street, city, state, postal string
		title                       string
	)

	// Walk once, collect everything we care about.
	var (
		amenities  []string
		photos     []string
		dataBeds   string
		dataBaths  string
		dataMax    string
		dataSqft   string
		dataAvail  string
		phoneVal   string
		floorPlans []FloorPlan
	)

	// To avoid duplicate amenities we dedupe via a set.
	amSet := map[string]bool{}

	var inAmenityBlock int
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "meta":
				switch strings.ToLower(attr(n, "itemprop")) {
				case "streetaddress":
					street = cliutil.CleanText(attr(n, "content"))
				case "addresslocality":
					city = cliutil.CleanText(attr(n, "content"))
				case "addressregion":
					state = cliutil.CleanText(attr(n, "content"))
				case "postalcode":
					postal = cliutil.CleanText(attr(n, "content"))
				case "telephone":
					if phoneVal == "" {
						phoneVal = cliutil.CleanText(attr(n, "content"))
					}
				}
			case "title":
				if title == "" {
					title = cliutil.CleanText(nodeText(n))
				}
			case "img":
				src := attr(n, "src")
				if src == "" {
					src = attr(n, "data-src")
				}
				if src != "" && (strings.Contains(src, "apartments.com") || strings.HasPrefix(src, "/")) {
					abs := resolveURL(src, listingURL)
					if abs != "" {
						photos = append(photos, abs)
					}
				}
			}
			// Pick up first non-empty data-* attributes.
			if dataBeds == "" {
				if v := attr(n, "data-beds"); v != "" {
					dataBeds = v
				}
			}
			if dataBaths == "" {
				if v := attr(n, "data-baths"); v != "" {
					dataBaths = v
				}
			}
			if dataMax == "" {
				if v := attr(n, "data-maxrent"); v != "" {
					dataMax = v
				}
			}
			if dataSqft == "" {
				if v := attr(n, "data-sqft-min"); v != "" {
					dataSqft = v
				}
			}
			if dataAvail == "" {
				if v := attr(n, "data-availability"); v != "" {
					dataAvail = v
				}
			}
			// Floor plan rows: any element carrying data-rent-min OR
			// data-rent-max + a data-beds-min (the apartments.com
			// "all" floor-plan tab structure).
			if attr(n, "data-rent-min") != "" || attr(n, "data-rent-max") != "" ||
				attr(n, "data-beds-min") != "" {
				fp := FloorPlan{
					Name:    cliutil.CleanText(attr(n, "data-name")),
					Beds:    parseBedsValue(attr(n, "data-beds-min")),
					Baths:   parseFloatAttr(attr(n, "data-baths-min")),
					Sqft:    parseIntAttr(attr(n, "data-sqft-min")),
					RentMin: parseIntAttr(attr(n, "data-rent-min")),
					RentMax: parseIntAttr(attr(n, "data-rent-max")),
				}
				if fp.Name == "" {
					fp.Name = cliutil.CleanText(attr(n, "data-fp-name"))
				}
				// Only keep if at least one numeric signal landed.
				if fp.Beds > 0 || fp.Sqft > 0 || fp.RentMin > 0 || fp.RentMax > 0 {
					floorPlans = append(floorPlans, fp)
				}
			}

			// Amenity tracking: enter when we see a class containing
			// "amenitiesList" / "specsList" / "amenityGroup".
			classV := strings.ToLower(attr(n, "class"))
			isAmenityHost := strings.Contains(classV, "amenitieslist") ||
				strings.Contains(classV, "specslist") ||
				strings.Contains(classV, "amenitygroup")
			if isAmenityHost {
				inAmenityBlock++
			}
			if inAmenityBlock > 0 && strings.EqualFold(n.Data, "li") {
				txt := cliutil.CleanText(strings.TrimSpace(nodeText(n)))
				if txt != "" && !amSet[txt] {
					amSet[txt] = true
					amenities = append(amenities, txt)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if isAmenityHost {
				inAmenityBlock--
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	li.Title = title
	li.Address = Address{
		StreetAddress: street,
		City:          city,
		State:         state,
		PostalCode:    postal,
	}
	li.Beds = parseBedsValue(dataBeds)
	li.Baths = parseFloatAttr(dataBaths)
	li.MaxRent = parseIntAttr(dataMax)
	li.Sqft = parseIntAttr(dataSqft)
	li.AvailableAt = dataAvail
	li.Phone = phoneVal
	li.Amenities = amenities
	li.Photos = dedupeStrings(photos)
	if floorPlans == nil {
		floorPlans = []FloorPlan{}
	}
	li.FloorPlans = floorPlans

	// Best-effort pet hints from amenities text. Apartments.com's pet
	// block isn't a stable schema.org target, so we infer Allows*Cats /
	// Allows*Dogs from amenity text and leave fees zero.
	for _, am := range amenities {
		l := strings.ToLower(am)
		if strings.Contains(l, "cat") {
			li.PetPolicy.AllowsCats = true
		}
		if strings.Contains(l, "dog") {
			li.PetPolicy.AllowsDogs = true
		}
	}

	return li, nil
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// firstDescendantWithDataAttr is used by callers that walk into the
// floor-plan block. Currently unused at package level but exposed via
// internal helpers above; keeping it package-private avoids surprising
// downstream callers.
var _ = firstDescendantWithDataAttr
