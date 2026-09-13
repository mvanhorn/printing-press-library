// Package immo holds the pure, network-free domain logic for immoweb-pp-cli:
// search criteria normalisation, Immoweb URL parsing, listing flattening,
// statistics, scoring and geo helpers. It has no dependency on the store or
// the HTTP client so every function is unit-testable.
package immo

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Criteria is the normalised form of an Immoweb search. It maps 1:1 onto the
// query parameters accepted by /en/search-results, /en/search-results-count
// and /en/search-results-map.
type Criteria struct {
	Types       []string `json:"types,omitempty"`        // HOUSE, APARTMENT, LAND, ...
	Deal        string   `json:"deal,omitempty"`         // FOR_SALE or FOR_RENT
	Communes    []string `json:"communes,omitempty"`     // human names, resolved to postal codes at run time
	PostalCodes []string `json:"postal_codes,omitempty"` // BE-1050 style (or bare 1050)
	Provinces   []string `json:"provinces,omitempty"`
	Districts   []string `json:"districts,omitempty"`
	MinPrice    int      `json:"min_price,omitempty"`
	MaxPrice    int      `json:"max_price,omitempty"`
	MinBedrooms int      `json:"min_bedrooms,omitempty"`
	MaxBedrooms int      `json:"max_bedrooms,omitempty"`
	MinSurface  int      `json:"min_surface,omitempty"`
	MaxSurface  int      `json:"max_surface,omitempty"`
	MinLand     int      `json:"min_land,omitempty"`
	MaxLand     int      `json:"max_land,omitempty"`
	MinYear     int      `json:"min_year,omitempty"`
	EPC         []string `json:"epc,omitempty"`
	Garden      bool     `json:"garden,omitempty"`
	Terrace     bool     `json:"terrace,omitempty"`
	Pool        bool     `json:"pool,omitempty"`
	NewBuild    bool     `json:"new_build,omitempty"`
	LifeAnnuity bool     `json:"life_annuity,omitempty"`
	PublicSale  bool     `json:"public_sale,omitempty"`
	Furnished   bool     `json:"furnished,omitempty"`
	Sort        string   `json:"sort,omitempty"` // relevance, newest, cheapest, most_expensive, postal_code
}

var typeAliases = map[string]string{
	"house": "HOUSE", "houses": "HOUSE", "maison": "HOUSE", "maisons": "HOUSE", "huis": "HOUSE", "woning": "HOUSE",
	"apartment": "APARTMENT", "apartments": "APARTMENT", "appartement": "APARTMENT", "appartements": "APARTMENT", "flat": "APARTMENT", "studio": "APARTMENT",
	"land": "LAND", "terrain": "LAND", "grond": "LAND", "plot": "LAND", "bouwgrond": "LAND",
	"office": "OFFICE", "bureau": "OFFICE", "kantoor": "OFFICE",
	"garage": "GARAGE", "parking": "GARAGE",
	"commercial": "COMMERCIAL", "business": "COMMERCIAL", "commerce": "COMMERCIAL", "handelspand": "COMMERCIAL",
	"industry": "INDUSTRY", "industrie": "INDUSTRY", "industrial": "INDUSTRY",
	"other": "OTHER", "autre": "OTHER", "andere": "OTHER",
}

// NormalizeType maps a user-facing property type (EN/FR/NL, any case) to the
// Immoweb enum. Upper-case enum values pass through.
func NormalizeType(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	if k == "" {
		return "", fmt.Errorf("empty property type")
	}
	if v, ok := typeAliases[k]; ok {
		return v, nil
	}
	up := strings.ToUpper(k)
	for _, v := range typeAliases {
		if v == up {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown property type %q (use house, apartment, land, office, garage, commercial, industry or other)", s)
}

// NormalizeTypes splits a comma list and normalises each entry, de-duplicated.
func NormalizeTypes(csv string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, part := range SplitCSV(csv) {
		v, err := NormalizeType(part)
		if err != nil {
			return nil, err
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, nil
}

// NormalizeDeal maps sale/rent synonyms (EN/FR/NL and Immoweb slugs) to
// FOR_SALE / FOR_RENT.
func NormalizeDeal(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sale", "sell", "buy", "for-sale", "for_sale", "forsale", "vente", "a-vendre", "à-vendre", "acheter", "te-koop", "koop", "kopen":
		return "FOR_SALE", nil
	case "rent", "rental", "let", "for-rent", "for_rent", "forrent", "location", "louer", "a-louer", "à-louer", "te-huur", "huur", "huren":
		return "FOR_RENT", nil
	case "":
		return "", fmt.Errorf("empty deal")
	}
	return "", fmt.Errorf("unknown deal %q (use sale or rent)", s)
}

// NormalizeSort maps friendly sort names to Immoweb orderBy values.
func NormalizeSort(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "-", "_"))) {
	case "", "relevance", "relevant":
		return "relevance", nil
	case "newest", "new", "recent", "latest":
		return "newest", nil
	case "cheapest", "price", "price_asc", "cheap":
		return "cheapest", nil
	case "most_expensive", "expensive", "price_desc":
		return "most_expensive", nil
	case "postal_code", "postcode", "zip":
		return "postal_code", nil
	}
	return "", fmt.Errorf("unknown sort %q (use relevance, newest, cheapest, most-expensive or postal-code)", s)
}

var postcodeRe = regexp.MustCompile(`^(?:BE-)?([0-9]{4})$`)

// NormalizePostalCode returns the BE-1234 form, or "" when s is not a
// Belgian postal code.
func NormalizePostalCode(s string) string {
	m := postcodeRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if m == nil {
		return ""
	}
	return "BE-" + m[1]
}

// BarePostalCode strips a BE- prefix: "BE-1050" -> "1050".
func BarePostalCode(s string) string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "BE-")
}

// SplitCSV splits on commas, trims, drops empties.
func SplitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Params renders the criteria as Immoweb query parameters. Communes must be
// resolved into PostalCodes before calling Params; unresolved commune names
// are not sent.
func (c Criteria) Params() map[string]string {
	p := map[string]string{"countries": "BE"}
	if len(c.Types) > 0 {
		p["propertyTypes"] = strings.Join(c.Types, ",")
	}
	if c.Deal != "" {
		p["transactionTypes"] = c.Deal
	}
	if len(c.PostalCodes) > 0 {
		p["postalCodes"] = strings.Join(c.PostalCodes, ",")
	}
	if len(c.Provinces) > 0 {
		p["provinces"] = strings.ToUpper(strings.Join(c.Provinces, ","))
	}
	if len(c.Districts) > 0 {
		p["districts"] = strings.ToUpper(strings.Join(c.Districts, ","))
	}
	setInt := func(k string, v int) {
		if v > 0 {
			p[k] = strconv.Itoa(v)
		}
	}
	setInt("minPrice", c.MinPrice)
	setInt("maxPrice", c.MaxPrice)
	setInt("minBedroomCount", c.MinBedrooms)
	setInt("maxBedroomCount", c.MaxBedrooms)
	setInt("minSurface", c.MinSurface)
	setInt("maxSurface", c.MaxSurface)
	setInt("minLandSurface", c.MinLand)
	setInt("maxLandSurface", c.MaxLand)
	setInt("minConstructionYear", c.MinYear)
	if len(c.EPC) > 0 {
		p["epcScores"] = strings.ToUpper(strings.Join(c.EPC, ","))
	}
	setBool := func(k string, v bool) {
		if v {
			p[k] = "true"
		}
	}
	setBool("hasGarden", c.Garden)
	setBool("hasTerrace", c.Terrace)
	setBool("hasSwimmingPool", c.Pool)
	setBool("isNewlyBuilt", c.NewBuild)
	setBool("isALifeAnnuitySale", c.LifeAnnuity)
	setBool("isAPublicSale", c.PublicSale)
	setBool("isFurnished", c.Furnished)
	if c.Sort != "" {
		p["orderBy"] = c.Sort
	}
	return p
}

// Validate reports criteria that Immoweb cannot answer.
func (c Criteria) Validate() error {
	if len(c.Types) == 0 {
		return fmt.Errorf("a property type is required (--type house, apartment, land, ...)")
	}
	if c.Deal == "" {
		return fmt.Errorf("a deal is required (--deal sale or --deal rent)")
	}
	if c.MinPrice > 0 && c.MaxPrice > 0 && c.MinPrice > c.MaxPrice {
		return fmt.Errorf("--min-price %d is greater than --max-price %d", c.MinPrice, c.MaxPrice)
	}
	for _, e := range c.EPC {
		switch strings.ToUpper(e) {
		case "A++", "A+", "A", "B", "C", "D", "E", "F", "G":
		default:
			return fmt.Errorf("unknown EPC label %q (use A, B, C, D, E, F or G)", e)
		}
	}
	return nil
}

// Key returns a stable identifier for the criteria (used to group local
// search runs). Order-insensitive for list fields.
func (c Criteria) Key() string {
	sorted := func(in []string) []string {
		out := append([]string(nil), in...)
		sort.Strings(out)
		return out
	}
	c.Types, c.PostalCodes, c.Provinces, c.Districts, c.EPC = sorted(c.Types), sorted(c.PostalCodes), sorted(c.Provinces), sorted(c.Districts), sorted(c.EPC)
	p := c.Params()
	delete(p, "orderBy")
	if len(c.Communes) > 0 {
		names := append([]string(nil), c.Communes...)
		for i := range names {
			names[i] = Fold(names[i])
		}
		sort.Strings(names)
		p["communes"] = strings.Join(names, ",")
	}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(p[k])
		b.WriteByte('&')
	}
	return b.String()
}

// Fold lower-cases, strips accents and collapses separators so "Liège",
// "liege" and "LIEGE" compare equal and "Saint-Gilles" == "saint gilles".
func Fold(s string) string {
	t := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	lastSpace := false
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r == '-' || r == '_' || r == '\'' || unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			lastSpace = true
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

var slugTypes = map[string][]string{
	"maison": {"HOUSE"}, "house": {"HOUSE"}, "huis": {"HOUSE"},
	"appartement": {"APARTMENT"}, "apartment": {"APARTMENT"},
	"maison-et-appartement": {"HOUSE", "APARTMENT"}, "house-and-apartment": {"HOUSE", "APARTMENT"}, "huis-en-appartement": {"HOUSE", "APARTMENT"},
	"terrain": {"LAND"}, "land": {"LAND"}, "grond": {"LAND"}, "terrain-a-batir": {"LAND"}, "bouwgrond": {"LAND"},
	"bureau": {"OFFICE"}, "office": {"OFFICE"}, "kantoor": {"OFFICE"},
	"garage":   {"GARAGE"},
	"commerce": {"COMMERCIAL"}, "business": {"COMMERCIAL"}, "handelspand": {"COMMERCIAL"},
	"industrie": {"INDUSTRY"}, "industry": {"INDUSTRY"},
	"autre": {"OTHER"}, "other": {"OTHER"}, "andere": {"OTHER"},
}

var slugDeals = map[string]string{
	"a-vendre": "FOR_SALE", "for-sale": "FOR_SALE", "te-koop": "FOR_SALE",
	"a-louer": "FOR_RENT", "for-rent": "FOR_RENT", "te-huur": "FOR_RENT",
}

// ParseSearchURL turns an Immoweb search URL (FR/NL/EN, search page or
// search-results JSON URL) into Criteria. Path segments carry type, deal and
// optionally locality/postal code; query parameters carry the rest.
func ParseSearchURL(raw string) (Criteria, error) {
	var c Criteria
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return c, fmt.Errorf("parsing URL: %w", err)
	}
	if u.Host != "" && !strings.HasSuffix(strings.ToLower(u.Host), "immoweb.be") {
		return c, fmt.Errorf("%s is not an immoweb.be URL", u.Host)
	}
	segs := []string{}
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segs = append(segs, strings.ToLower(s))
		}
	}
	for _, s := range segs {
		if t, ok := slugTypes[s]; ok && len(c.Types) == 0 {
			c.Types = append([]string(nil), t...)
			continue
		}
		if d, ok := slugDeals[s]; ok {
			c.Deal = d
			continue
		}
		if pc := NormalizePostalCode(s); pc != "" {
			c.PostalCodes = appendUnique(c.PostalCodes, pc)
		}
	}
	q := u.Query()
	if v := q.Get("propertyTypes"); v != "" {
		c.Types = SplitCSV(strings.ToUpper(v))
	}
	if v := q.Get("transactionTypes"); v != "" {
		c.Deal = strings.ToUpper(v)
	}
	if v := q.Get("postalCodes"); v != "" {
		for _, pc := range SplitCSV(v) {
			if n := NormalizePostalCode(pc); n != "" {
				c.PostalCodes = appendUnique(c.PostalCodes, n)
			}
		}
	}
	for _, v := range SplitCSV(q.Get("provinces")) {
		if p, err := NormalizeProvince(v); err == nil {
			c.Provinces = appendUnique(c.Provinces, p)
		} else {
			c.Provinces = appendUnique(c.Provinces, strings.ToUpper(v))
		}
	}
	if v := q.Get("districts"); v != "" {
		c.Districts = SplitCSV(v)
	}
	atoi := func(k string) int {
		n, _ := strconv.Atoi(q.Get(k))
		return n
	}
	c.MinPrice = atoi("minPrice")
	c.MaxPrice = atoi("maxPrice")
	c.MinBedrooms = atoi("minBedroomCount")
	c.MaxBedrooms = atoi("maxBedroomCount")
	c.MinSurface = atoi("minSurface")
	c.MaxSurface = atoi("maxSurface")
	c.MinLand = atoi("minLandSurface")
	c.MaxLand = atoi("maxLandSurface")
	c.MinYear = atoi("minConstructionYear")
	if v := q.Get("epcScores"); v != "" {
		c.EPC = SplitCSV(v)
	}
	b := func(k string) bool { return strings.EqualFold(q.Get(k), "true") }
	c.Garden = b("hasGarden")
	c.Terrace = b("hasTerrace")
	c.Pool = b("hasSwimmingPool")
	c.NewBuild = b("isNewlyBuilt")
	c.LifeAnnuity = b("isALifeAnnuitySale")
	c.PublicSale = b("isAPublicSale")
	c.Furnished = b("isFurnished")
	if v := q.Get("orderBy"); v != "" {
		c.Sort = v
	}
	if len(c.Types) == 0 && c.Deal == "" {
		return c, fmt.Errorf("could not find a property type or deal in %q; paste an Immoweb search URL such as https://www.immoweb.be/fr/recherche/maison/a-vendre", raw)
	}
	return c, nil
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// provinceAliases maps French, Dutch, English and German province names
// (folded) to Immoweb's values. Immoweb silently ignores an unknown
// province and searches the whole country, so unknown names are refused.
var provinceAliases = map[string]string{
	"antwerp": "ANTWERP", "anvers": "ANTWERP", "antwerpen": "ANTWERP",
	"limburg": "LIMBURG", "limbourg": "LIMBURG",
	"east flanders": "EAST_FLANDERS", "flandre orientale": "EAST_FLANDERS", "oost vlaanderen": "EAST_FLANDERS",
	"west flanders": "WEST_FLANDERS", "flandre occidentale": "WEST_FLANDERS", "west vlaanderen": "WEST_FLANDERS",
	"flemish brabant": "FLEMISH_BRABANT", "brabant flamand": "FLEMISH_BRABANT", "vlaams brabant": "FLEMISH_BRABANT",
	"walloon brabant": "WALLOON_BRABANT", "brabant wallon": "WALLOON_BRABANT", "waals brabant": "WALLOON_BRABANT",
	"brussels": "BRUSSELS", "bruxelles": "BRUSSELS", "brussel": "BRUSSELS", "bruxelles capitale": "BRUSSELS",
	"hainaut": "HAINAUT", "henegouwen": "HAINAUT",
	"liege": "LIEGE", "luik": "LIEGE", "luttich": "LIEGE",
	"luxembourg": "LUXEMBOURG", "luxemburg": "LUXEMBOURG",
	"namur": "NAMUR", "namen": "NAMUR",
}

// NormalizeProvince maps a province name in any national language (or an
// Immoweb value such as WALLOON_BRABANT) to Immoweb's value.
func NormalizeProvince(s string) (string, error) {
	f := Fold(s)
	if v, ok := provinceAliases[f]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown province %q (use one of: antwerp, limburg, east_flanders, west_flanders, flemish_brabant, walloon_brabant, brussels, hainaut, liege, luxembourg, namur; French and Dutch names work too)", s)
}
