// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package zimmo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Criteria is the CLI-side search definition. It serialises to Zimmo's
// filter object and is what saved searches store.
type Criteria struct {
	Statuses   []string `json:"statuses,omitempty"`   // FOR_SALE, TO_RENT, SOLD, RENTED, TAKE_OVER
	Categories []string `json:"categories,omitempty"` // HOUSE, APARTMENT, PLOT, GARAGE, COMMERCIAL, ROOM, OTHER
	PlaceIDs   []int    `json:"place_ids,omitempty"`
	Postcodes  []string `json:"postcodes,omitempty"`
	// Communes are names still to be resolved to PlaceIDs by the caller
	// (geo-api); Filter ignores them.
	Communes   []string     `json:"communes,omitempty"`
	MinPrice   *int         `json:"min_price,omitempty"`
	MaxPrice   *int         `json:"max_price,omitempty"`
	MinBeds    *int         `json:"min_bedrooms,omitempty"`
	MaxBeds    *int         `json:"max_bedrooms,omitempty"`
	MinSurface *int         `json:"min_surface,omitempty"`
	MaxSurface *int         `json:"max_surface,omitempty"`
	MinPlot    *int         `json:"min_plot,omitempty"`
	MinYear    *int         `json:"min_year,omitempty"`
	MaxYear    *int         `json:"max_year,omitempty"`
	EPC        []string     `json:"epc,omitempty"` // A..G letters
	Text       string       `json:"text,omitempty"`
	Codes      []string     `json:"codes,omitempty"`
	NewBuild   *bool        `json:"new_build,omitempty"`
	Polygon    [][2]float64 `json:"polygon,omitempty"` // [lat, lon] ring
	Sort       string       `json:"sort,omitempty"`
	// Raw is a filter decoded from a pasted zimmo.be search URL; fields
	// above are merged over it.
	Raw map[string]any `json:"raw,omitempty"`
}

// Statuses accepted on the CLI, with FR/NL/EN words.
var statusWords = map[string]string{
	"sale": "FOR_SALE", "for-sale": "FOR_SALE", "for_sale": "FOR_SALE", "buy": "FOR_SALE", "a-vendre": "FOR_SALE", "vendre": "FOR_SALE", "te-koop": "FOR_SALE", "koop": "FOR_SALE",
	"rent": "TO_RENT", "to-rent": "TO_RENT", "to_rent": "TO_RENT", "a-louer": "TO_RENT", "louer": "TO_RENT", "te-huur": "TO_RENT", "huur": "TO_RENT",
	"sold": "SOLD", "vendu": "SOLD", "verkocht": "SOLD",
	"rented": "RENTED", "loue": "RENTED", "verhuurd": "RENTED",
	"take-over": "TAKE_OVER", "take_over": "TAKE_OVER", "overname": "TAKE_OVER", "reprise": "TAKE_OVER",
}

var categoryWords = map[string]string{
	"house": "HOUSE", "maison": "HOUSE", "huis": "HOUSE", "villa": "HOUSE", "woning": "HOUSE",
	"apartment": "APARTMENT", "appartement": "APARTMENT", "flat": "APARTMENT", "studio": "APARTMENT", "loft": "APARTMENT", "penthouse": "APARTMENT",
	"plot": "PLOT", "land": "PLOT", "terrain": "PLOT", "grond": "PLOT", "bouwgrond": "PLOT",
	"garage": "GARAGE", "parking": "GARAGE",
	"commercial": "COMMERCIAL", "commerce": "COMMERCIAL", "bien-professionnel": "COMMERCIAL", "bureau": "COMMERCIAL", "handelspand": "COMMERCIAL", "kantoor": "COMMERCIAL", "bedrijfsvastgoed": "COMMERCIAL",
	"room": "ROOM", "kot": "ROOM", "kot-colocation": "ROOM", "kamer": "ROOM", "colocation": "ROOM",
	"other": "OTHER",
}

// ParseStatus maps a CLI word or enum value to Zimmo's status enum.
func ParseStatus(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	up := strings.ToUpper(k)
	switch up {
	case "FOR_SALE", "TO_RENT", "SOLD", "RENTED", "TAKE_OVER":
		return up, nil
	}
	if v, ok := statusWords[Fold(k)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown status %q (use sale, rent, sold, rented or take-over)", s)
}

// ParseCategory maps a CLI word or enum value to Zimmo's category enum.
func ParseCategory(s string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(s))
	up := strings.ToUpper(k)
	switch up {
	case "HOUSE", "APARTMENT", "PLOT", "GARAGE", "COMMERCIAL", "ROOM", "OTHER":
		return up, nil
	}
	if v, ok := categoryWords[Fold(k)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown property type %q (use house, apartment, plot, garage, commercial, room)", s)
}

var epcLetter = regexp.MustCompile(`^[A-G]$`)

// ParseEPC validates a comma-separated list of EPC letters. "A+" and
// "A++" collapse to A because Zimmo's filter only knows A..G.
func ParseEPC(csv string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(csv, ",") {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		p = strings.TrimRight(p, "+")
		if !epcLetter.MatchString(p) {
			return nil, fmt.Errorf("invalid EPC letter %q (use A..G)", p)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// expandEPC adds Zimmo's +/- variants of each letter (D -> D, D_PLUS,
// D_MINUS; A also A_PLUS_PLUS) so a letter filter matches every label.
func expandEPC(letters []string) []string {
	out := make([]string, 0, len(letters)*3)
	for _, l := range letters {
		out = append(out, l, l+"_PLUS", l+"_MINUS")
		if l == "A" {
			out = append(out, "A_PLUS_PLUS")
		}
	}
	return out
}

func rangeFilter(min, max *int) map[string]any {
	if min == nil && max == nil {
		return nil
	}
	r := map[string]any{}
	if min != nil {
		r["min"] = *min
	}
	if max != nil {
		r["max"] = *max
	}
	return map[string]any{"range": r, "unknown": false}
}

// Filter builds Zimmo's filter object.
func (c Criteria) Filter() map[string]any {
	f := map[string]any{}
	for k, v := range c.Raw {
		f[k] = v
	}
	statuses := c.Statuses
	if len(statuses) == 0 {
		if _, ok := f["status"]; !ok {
			statuses = []string{"FOR_SALE"}
		}
	}
	if len(statuses) > 0 {
		// The site shows take-over businesses under "for sale".
		st := append([]string{}, statuses...)
		if len(st) == 1 && st[0] == "FOR_SALE" {
			st = append(st, "TAKE_OVER")
		}
		f["status"] = map[string]any{"in": st}
	}
	if len(c.Categories) > 0 {
		f["category"] = map[string]any{"in": c.Categories}
	}
	if len(c.PlaceIDs) > 0 {
		f["placeId"] = map[string]any{"in": c.PlaceIDs}
	}
	if len(c.Postcodes) > 0 {
		f["postalCode"] = map[string]any{"in": c.Postcodes}
	}
	if r := rangeFilter(c.MinPrice, c.MaxPrice); r != nil {
		f["price"] = r
	}
	if r := rangeFilter(c.MinBeds, c.MaxBeds); r != nil {
		f["bedrooms"] = r
	}
	if r := rangeFilter(c.MinSurface, c.MaxSurface); r != nil {
		f["floorspaceSurface"] = r
	}
	if r := rangeFilter(c.MinPlot, nil); r != nil {
		f["plotSurface"] = r
	}
	if r := rangeFilter(c.MinYear, c.MaxYear); r != nil {
		f["constructionYear"] = r
	}
	if len(c.EPC) > 0 {
		f["energyLabel"] = map[string]any{"in": expandEPC(c.EPC)}
	}
	if c.Text != "" {
		f["text"] = map[string]any{"query": c.Text}
	}
	if len(c.Codes) > 0 {
		f["zimmoCode"] = map[string]any{"in": c.Codes}
	}
	if c.NewBuild != nil {
		f["newConstruction"] = map[string]any{"eq": *c.NewBuild}
	}
	if len(c.Polygon) >= 3 {
		pts := make([][]float64, 0, len(c.Polygon)+1)
		for _, p := range c.Polygon {
			pts = append(pts, []float64{p[0], p[1]})
		}
		if c.Polygon[0] != c.Polygon[len(c.Polygon)-1] {
			pts = append(pts, []float64{c.Polygon[0][0], c.Polygon[0][1]})
		}
		f["polygon"] = map[string]any{"in": []any{map[string]any{"points": pts}}}
	}
	return f
}

// SortModes lists the accepted --sort values.
var SortModes = []string{"newest", "oldest", "price-asc", "price-desc", "epc", "relevance"}

// Sorting builds Zimmo's sorting array. Default is newest first, which
// also gives stable paging for sync.
func (c Criteria) Sorting() ([]map[string]any, error) {
	switch c.Sort {
	case "", "newest":
		return []map[string]any{{"type": "DATE", "order": "DESC"}}, nil
	case "oldest":
		return []map[string]any{{"type": "DATE", "order": "ASC"}}, nil
	case "price-asc", "price":
		return []map[string]any{{"type": "PRICE", "order": "ASC"}}, nil
	case "price-desc":
		return []map[string]any{{"type": "PRICE", "order": "DESC"}}, nil
	case "epc":
		return []map[string]any{{"type": "ENERGY_LABEL", "order": "ASC"}}, nil
	case "relevance":
		return nil, nil
	}
	return nil, fmt.Errorf("unknown --sort %q (use %s)", c.Sort, strings.Join(SortModes, ", "))
}

// Request builds a search request for one page.
func (c Criteria) Request(from, size int) (SearchRequest, error) {
	s, err := c.Sorting()
	if err != nil {
		return SearchRequest{}, err
	}
	return SearchRequest{Paging: Paging{From: from, Size: size}, Sorting: s, Filter: c.Filter()}, nil
}

// Key is a stable hash-friendly representation used by saved searches.
func (c Criteria) Key() string {
	b, _ := json.Marshal(c)
	return string(b)
}

var seoPath = regexp.MustCompile(`^/(?:fr|nl|en)/(?:([a-z0-9-]+?)(?:-(\d{4}))?/)?(a-vendre|a-louer|te-koop|te-huur|for-sale|to-rent)(?:/([a-z-]+))?/?$`)

// ParseSearchURL turns a zimmo.be search or result-page URL into criteria.
// Supported: ?search=<base64 JSON> (advanced search, saved searches) and
// SEO pages like /fr/bruxelles-1000/a-vendre/appartement.
func ParseSearchURL(raw string) (Criteria, error) {
	var c Criteria
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return c, fmt.Errorf("not a URL: %q", raw)
	}
	if !strings.HasSuffix(strings.ToLower(u.Host), "zimmo.be") {
		return c, fmt.Errorf("not a zimmo.be URL: %q", raw)
	}
	if s := u.Query().Get("search"); s != "" {
		filter, err := decodeSearchParam(s)
		if err != nil {
			return c, err
		}
		c.Raw = filter
		return c, nil
	}
	m := seoPath.FindStringSubmatch(strings.TrimSuffix(u.Path, "/") + "/")
	if m == nil {
		m = seoPath.FindStringSubmatch(u.Path)
	}
	if m == nil {
		return c, fmt.Errorf("unsupported zimmo.be URL path %q: paste a search result page or an advanced-search URL with ?search=", u.Path)
	}
	if m[2] != "" {
		c.Postcodes = []string{m[2]}
	}
	st, _ := ParseStatus(m[3])
	c.Statuses = []string{st}
	if m[4] != "" {
		if cat, err := ParseCategory(m[4]); err == nil {
			c.Categories = []string{cat}
		}
	}
	if m[2] == "" && m[1] != "" && m[1] != "recherche" && m[1] != "zoeken" && m[1] != "search" {
		c.Communes = []string{strings.ReplaceAll(m[1], "-", " ")}
	}
	return c, nil
}

func decodeSearchParam(s string) (map[string]any, error) {
	// url.Query() decodes an unescaped "+" of standard base64 as a space.
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "+")
	var raw []byte
	var err error
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		raw, err = enc.DecodeString(s)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decoding search= parameter: %w", err)
	}
	var wrapper struct {
		Filter map[string]any `json:"filter"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Filter == nil {
		return nil, fmt.Errorf("search= parameter is not a Zimmo filter")
	}
	return wrapper.Filter, nil
}

// SquareAround returns a closed square ring of half-side radiusM metres
// around (lat, lon), as [lat, lon] points.
func SquareAround(lat, lon, radiusM float64) [][2]float64 {
	dLat := radiusM / 111320.0
	dLon := radiusM / (111320.0 * math.Cos(lat*math.Pi/180))
	return [][2]float64{
		{lat - dLat, lon - dLon},
		{lat - dLat, lon + dLon},
		{lat + dLat, lon + dLon},
		{lat + dLat, lon - dLon},
		{lat - dLat, lon - dLon},
	}
}

// DistanceM is the haversine distance in metres.
func DistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// Fold lower-cases and strips common Latin accents for matching.
func Fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'à', 'â', 'ä', 'á':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'î', 'ï', 'í':
			r = 'i'
		case 'ô', 'ö', 'ó':
			r = 'o'
		case 'ù', 'û', 'ü', 'ú':
			r = 'u'
		case 'ç':
			r = 'c'
		case '\'', '’':
			r = ' '
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SortedKeys is a small helper for deterministic output.
func SortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
