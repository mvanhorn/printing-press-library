// Package immovlan holds the pure logic behind immovlan-pp-cli: search
// criteria, HTML parsing of Immovlan's server-rendered pages, and the
// small reference tables the commands share. No I/O lives here.
package immovlan

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Deal values as the CLI exposes them; Params maps them to Immovlan's
// transactiontypes slugs.
const (
	DealSale       = "sale"
	DealRent       = "rent"
	DealPublicSale = "public-sale"
	DealColocation = "colocation"
)

var dealSlugs = map[string]string{
	DealSale: "a-vendre", DealRent: "a-louer", DealPublicSale: "en-vente-publique", DealColocation: "en-colocation",
}

var dealAliases = map[string]string{
	"sale": DealSale, "vente": DealSale, "a-vendre": DealSale, "à-vendre": DealSale, "buy": DealSale, "for_sale": DealSale, "te-koop": DealSale,
	"rent": DealRent, "location": DealRent, "a-louer": DealRent, "à-louer": DealRent, "for_rent": DealRent, "te-huur": DealRent, "louer": DealRent,
	"public-sale": DealPublicSale, "public_sale": DealPublicSale, "publicsale": DealPublicSale, "en-vente-publique": DealPublicSale, "vente-publique": DealPublicSale, "notary": DealPublicSale, "auction": DealPublicSale,
	"colocation": DealColocation, "en-colocation": DealColocation, "flatshare": DealColocation, "shared": DealColocation,
}

// NormalizeDeal maps sale/rent/public-sale/colocation and their French,
// Dutch and Immoweb-style spellings to the CLI's canonical deal value.
func NormalizeDeal(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	k = strings.ReplaceAll(k, " ", "-")
	if v, ok := dealAliases[k]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown deal %q (use sale, rent, public-sale or colocation)", s)
}

var typeAliases = map[string]string{
	"maison": "maison", "house": "maison", "huis": "maison", "houses": "maison", "maisons": "maison",
	"appartement": "appartement", "apartment": "appartement", "flat": "appartement", "apartments": "appartement", "appartements": "appartement",
	"terrain": "terrain", "land": "terrain", "grond": "terrain", "plot": "terrain",
	"garage": "garage", "parking": "garage",
	"kot": "kot", "studentroom": "kot", "student-room": "kot",
}

// NormalizeType maps a property type in French, Dutch or English (or an
// Immoweb constant such as HOUSE) to Immovlan's slug.
func NormalizeType(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	k = strings.ReplaceAll(k, "_", "-")
	if v, ok := typeAliases[k]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown property type %q (use maison, appartement, terrain, garage or kot)", s)
}

// EPCBand maps a PEB letter to Immovlan's search band. Bands are also
// accepted as-is.
func EPCBand(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	switch k {
	case "a", "a+", "a++", "excellent":
		return "excellent", nil
	case "b", "c", "good":
		return "good", nil
	case "d", "e", "poor":
		return "poor", nil
	case "f", "g", "bad":
		return "bad", nil
	case "unknown", "?", "none":
		return "unknown", nil
	}
	return "", fmt.Errorf("unknown EPC value %q (use letters A-G or bands excellent/good/poor/bad/unknown)", s)
}

// BandLetters lists the PEB letters an Immovlan band covers.
func BandLetters(band string) []string {
	switch band {
	case "excellent":
		return []string{"A"}
	case "good":
		return []string{"B", "C"}
	case "poor":
		return []string{"D", "E"}
	case "bad":
		return []string{"F", "G"}
	}
	return nil
}

var postcodeRe = regexp.MustCompile(`^(BE-)?([1-9][0-9]{3})$`)

// NormalizePostalCode accepts 1050 or BE-1050 and returns the bare code.
func NormalizePostalCode(s string) string {
	m := postcodeRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if m == nil {
		return ""
	}
	return m[2]
}

// SplitCSV splits a comma-separated flag value, trimming blanks.
func SplitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Criteria is one Immovlan search.
type Criteria struct {
	Types       []string `json:"types,omitempty"`
	Deal        string   `json:"deal"`
	PostalCodes []string `json:"postal_codes,omitempty"`
	Towns       []string `json:"towns,omitempty"` // Immovlan slugs such as 1030-schaerbeek
	EPC         []string `json:"epc,omitempty"`   // bands: bad, poor, good, excellent, unknown
	MinPrice    int      `json:"min_price,omitempty"`
	MaxPrice    int      `json:"max_price,omitempty"`
	MinBedrooms int      `json:"min_bedrooms,omitempty"`
	MaxBedrooms int      `json:"max_bedrooms,omitempty"`
	SortBy      string   `json:"sort_by,omitempty"` // price, nrOfBedrooms, totalSurface, zipCode; empty = newest
	SortDir     string   `json:"sort_direction,omitempty"`
}

var sortFields = map[string]string{
	"newest": "", "recent": "", "": "",
	"price": "price", "price_asc": "price", "price-asc": "price", "cheapest": "price",
	"price_desc": "price", "price-desc": "price", "expensive": "price",
	"bedrooms": "nrOfBedrooms", "surface": "totalSurface", "postcode": "zipCode", "zip": "zipCode",
}

// NormalizeSort maps a friendly sort name to Immovlan's sortby/sortdirection.
func NormalizeSort(s string) (field, dir string, err error) {
	k := strings.ToLower(strings.TrimSpace(s))
	f, ok := sortFields[k]
	if !ok {
		return "", "", fmt.Errorf("unknown sort %q (use newest, price, price_desc, bedrooms, surface or postcode)", s)
	}
	if f == "" {
		return "", "", nil
	}
	dir = "ascending"
	if strings.HasSuffix(k, "desc") || k == "expensive" {
		dir = "descending"
	}
	return f, dir, nil
}

// Validate checks the criteria before a request.
func (c Criteria) Validate() error {
	if c.Deal == "" {
		return fmt.Errorf("a deal is required (--deal sale|rent|public-sale|colocation)")
	}
	if _, ok := dealSlugs[c.Deal]; !ok {
		return fmt.Errorf("unknown deal %q", c.Deal)
	}
	if len(c.PostalCodes) == 0 && len(c.Towns) == 0 {
		return fmt.Errorf("a location is required (--postcode 1030,1210 or --commune schaerbeek)")
	}
	if c.MinPrice < 0 || c.MaxPrice < 0 || (c.MaxPrice > 0 && c.MinPrice > c.MaxPrice) {
		return fmt.Errorf("invalid price range")
	}
	if c.MaxBedrooms > 0 && c.MinBedrooms > c.MaxBedrooms {
		return fmt.Errorf("invalid bedroom range")
	}
	return nil
}

// Params renders the query string Immovlan's search page expects.
func (c Criteria) Params() map[string]string {
	p := map[string]string{"transactiontypes": dealSlugs[c.Deal]}
	if len(c.Types) > 0 {
		p["propertytypes"] = strings.Join(c.Types, ",")
	}
	// Immovlan ignores municipals=; towns= accepts bare postcodes and
	// <postcode>-<slug> values (checked live 2026-09-22).
	if towns := append(append([]string{}, c.Towns...), c.PostalCodes...); len(towns) > 0 {
		p["towns"] = strings.Join(towns, ",")
	}
	if len(c.EPC) > 0 {
		p["epcratings"] = strings.Join(c.EPC, ",")
	}
	set := func(k string, v int) {
		if v > 0 {
			p[k] = strconv.Itoa(v)
		}
	}
	set("minprice", c.MinPrice)
	set("maxprice", c.MaxPrice)
	set("minbedrooms", c.MinBedrooms)
	set("maxbedrooms", c.MaxBedrooms)
	if c.SortBy != "" {
		p["sortby"] = c.SortBy
		p["sortdirection"] = c.SortDir
	}
	return p
}

// Key is a stable identity for the search (sorted lists, no sort order); the
// store uses it to tell a re-saved search with reordered lists from a real
// criteria change.
func (c Criteria) Key() string {
	sorted := func(v []string) []string {
		out := append([]string(nil), v...)
		sort.Strings(out)
		return out
	}
	c.Types, c.PostalCodes, c.Towns, c.EPC = sorted(c.Types), sorted(c.PostalCodes), sorted(c.Towns), sorted(c.EPC)
	c.SortBy, c.SortDir = "", ""
	p := c.Params()
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+p[k])
	}
	return strings.Join(parts, "&")
}

// ParseSearchURL turns an immovlan.be search URL into criteria.
func ParseSearchURL(raw string) (Criteria, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || !strings.Contains(u.Host, "immovlan.be") {
		return Criteria{}, fmt.Errorf("not an immovlan.be URL: %q", raw)
	}
	q := u.Query()
	var c Criteria
	for _, t := range SplitCSV(q.Get("transactiontypes")) {
		if d, err := NormalizeDeal(t); err == nil && c.Deal == "" {
			c.Deal = d
		}
	}
	for _, t := range SplitCSV(q.Get("propertytypes")) {
		if v, err := NormalizeType(t); err == nil {
			c.Types = append(c.Types, v)
		}
	}
	c.Towns = SplitCSV(q.Get("towns"))
	// Legacy URLs only: the live site ignores municipals=, but old bookmarks carry it.
	for _, pc := range SplitCSV(q.Get("municipals")) {
		if v := NormalizePostalCode(pc); v != "" {
			c.PostalCodes = append(c.PostalCodes, v)
		}
	}
	for _, e := range SplitCSV(q.Get("epcratings")) {
		if b, err := EPCBand(e); err == nil {
			c.EPC = append(c.EPC, b)
		}
	}
	atoi := func(k string) int { n, _ := strconv.Atoi(q.Get(k)); return n }
	c.MinPrice, c.MaxPrice, c.MinBedrooms, c.MaxBedrooms = atoi("minprice"), atoi("maxprice"), atoi("minbedrooms"), atoi("maxbedrooms")
	c.SortBy, c.SortDir = q.Get("sortby"), q.Get("sortdirection")
	if c.Deal == "" {
		// /fr/immobilier/maison/a-vendre/schaerbeek style paths
		for _, seg := range strings.Split(u.Path, "/") {
			if d, err := NormalizeDeal(seg); err == nil {
				c.Deal = d
			} else if v, err := NormalizeType(seg); err == nil {
				c.Types = append(c.Types, v)
			}
		}
	}
	return c, nil
}

// Fold lowercases and strips accents and punctuation for matching.
func Fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(foldRune(r))
		case r == ' ' || r == '-' || r == '\'' || r == '_' || r == '.':
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func foldRune(r rune) rune {
	switch r {
	case 'à', 'â', 'ä', 'á', 'ã':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'î', 'ï', 'í':
		return 'i'
	case 'ô', 'ö', 'ó', 'õ':
		return 'o'
	case 'ù', 'û', 'ü', 'ú':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	}
	return r
}

// TownsPostcodes returns the postcodes embedded in <postcode>-<slug> town values.
func TownsPostcodes(towns []string) []string {
	out := []string{}
	for _, t := range towns {
		if pc := NormalizePostalCode(strings.SplitN(t, "-", 2)[0]); pc != "" {
			out = append(out, pc)
		}
	}
	return out
}

// TownSlug builds Immovlan's <postcode>-<slug> town value from a postcode
// and a locality name ("1030", "Schaerbeek" → "1030-schaerbeek").
func TownSlug(postcode, name string) string {
	return postcode + "-" + strings.ReplaceAll(Fold(name), " ", "-")
}
