package immovlan

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

// Listing is one Immovlan card or detail page, with the field names
// immoweb-pp-cli uses so the two portals can be merged row by row.
type Listing struct {
	ID          string   `json:"id"` // Immovlan reference, upper-case (VBE69761, RBU21531…)
	URL         string   `json:"url"`
	Deal        string   `json:"deal"`
	Type        string   `json:"type"`
	Subtype     string   `json:"subtype,omitempty"`
	Title       string   `json:"title,omitempty"`
	Locality    string   `json:"locality,omitempty"`
	PostalCode  string   `json:"postal_code,omitempty"`
	Street      string   `json:"street,omitempty"`
	Lat         *float64 `json:"lat,omitempty"`
	Lng         *float64 `json:"lng,omitempty"`
	Price       *float64 `json:"price,omitempty"`
	Bedrooms    *int     `json:"bedrooms,omitempty"`
	Bathrooms   *int     `json:"bathrooms,omitempty"`
	Surface     *float64 `json:"surface_m2,omitempty"`
	Land        *float64 `json:"land_m2,omitempty"`
	PricePerSqm *float64 `json:"price_per_m2,omitempty"`
	Agency      string   `json:"agency,omitempty"`
	AgencyID    string   `json:"agency_id,omitempty"`
	Private     bool     `json:"private_seller"`
	SellerType  string   `json:"seller_type,omitempty"` // estateAgents, private… as Immovlan reports it
	Flag        string   `json:"flag,omitempty"`        // new | best_of
	EPC         string   `json:"epc,omitempty"`         // PEB letter when known
	EPCBand     string   `json:"epc_band,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"` // JSON-LD datePosted (detail only)
	Phone       string   `json:"phone,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Detail is a listing page: the card fields plus the feature table.
type Detail struct {
	Listing
	Condition       string            `json:"condition,omitempty"`
	Rented          *bool             `json:"rented,omitempty"`
	CadastralIncome *float64          `json:"cadastral_income,omitempty"`
	Year            *int              `json:"construction_year,omitempty"`
	Garden          *float64          `json:"garden_m2,omitempty"`
	Terrace         *float64          `json:"terrace_m2,omitempty"`
	Heating         string            `json:"heating,omitempty"`
	Software        string            `json:"software,omitempty"` // the agency's CRM feeding Immovlan
	AgencyURL       string            `json:"agency_url,omitempty"`
	Photos          []string          `json:"pictures,omitempty"`
	Features        map[string]string `json:"features,omitempty"`
}

// ErrNoListing marks a page without a listing (withdrawn or unknown ref).
var ErrNoListing = errors.New("no listing on this page")

var refRe = regexp.MustCompile(`^[A-Za-z]{3}[0-9]{4,9}$`)

// ParseReference accepts a reference (vbe69761) or a detail URL and returns
// the upper-case reference.
func ParseReference(s string) (string, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "?"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if !refRe.MatchString(s) {
		return "", fmt.Errorf("%q is not an Immovlan reference (expected e.g. vbe69761 or a detail URL)", s)
	}
	return strings.ToUpper(s), nil
}

// ListingURL is the short canonical form Immovlan redirects from.
func ListingURL(ref string) string { return "https://immovlan.be/fr/detail/" + strings.ToLower(ref) }

// SearchPage is one parsed result page.
type SearchPage struct {
	Items    []Listing
	Page     int  // page this body renders (the active pagination link)
	LastPage int  // highest page number linked from the pagination, 1 when absent
	HasNext  bool // a pagination link points past this page
}

// ParseSearch extracts the listing cards of a search page.
func ParseSearch(body []byte) (SearchPage, error) {
	root, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return SearchPage{}, err
	}
	sp := SearchPage{Page: 1, LastPage: 1}
	linked := []int{}
	walk(root, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode {
			return
		}
		if n.Data == "article" && hasClass(n, "v3-search-card") {
			if l, ok := parseCard(n); ok {
				sp.Items = append(sp.Items, l)
			}
		}
		if n.Data == "a" && hasClass(n, "v3-pagination-btn") {
			// Immovlan windows the page list on long result sets, so the
			// page= parameter of every link (including "Suivant") is the
			// signal, not the visible numbers.
			p := pageParam(attr(n, "href"))
			if hasClass(n, "active") {
				sp.Page = p
			}
			linked = append(linked, p)
		}
	})
	for _, p := range linked {
		if p > sp.LastPage {
			sp.LastPage = p
		}
		if p > sp.Page {
			sp.HasNext = true
		}
	}
	return sp, nil
}

// pageParam reads page=N from a pagination href (1 when absent).
func pageParam(href string) int {
	u, err := url.Parse(html.UnescapeString(href))
	if err != nil {
		return 1
	}
	if p, err := strconv.Atoi(u.Query().Get("page")); err == nil && p > 0 {
		return p
	}
	return 1
}

func parseCard(card *xhtml.Node) (Listing, bool) {
	l := Listing{}
	l.URL = attr(card, "data-url")
	fillFromURL(&l, l.URL)
	switch attr(card, "itemtype") {
	case "http://schema.org/House":
		l.Type = "maison"
	case "http://schema.org/Apartment":
		l.Type = "appartement"
	}
	walk(card, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode {
			return
		}
		if v := attr(n, "data-value-id"); v != "" && l.ID == "" && refRe.MatchString(v) && n.Data == "button" && hasClass(n, "btn-favorite") {
			l.ID = strings.ToUpper(v)
		}
		if hasClass(n, "v3-search-card-price") {
			l.Price = parseEUR(text(n))
		}
		switch attr(n, "itemprop") {
		case "postalCode":
			if pc := NormalizePostalCode(text(n)); pc != "" {
				l.PostalCode = pc
			}
		case "addressLocality":
			l.Locality = strings.TrimSpace(text(n))
		case "description":
			l.Description = strings.Join(strings.Fields(text(n)), " ")
		case "numberOfBedrooms":
			if v, err := strconv.Atoi(attr(n, "content")); err == nil {
				l.Bedrooms = &v
			}
		case "numberOfBathroomsTotal":
			if v, err := strconv.Atoi(attr(n, "content")); err == nil {
				l.Bathrooms = &v
			}
		}
		if n.Data == "h2" && hasClass(n, "v3-search-card-title") {
			l.Title = strings.Join(strings.Fields(text(n)), " ")
		}
		if hasClass(n, "v3-epc-watermark") {
			for _, c := range strings.Fields(attr(n, "class")) {
				if letter := epcFromClass(c); letter != "" {
					l.EPC = letter
				}
			}
		}
		if hasClass(n, "v3-search-card-ribbon") {
			switch strings.ToLower(strings.TrimSpace(text(n))) {
			case "nouveau", "nieuw", "new":
				l.Flag = "new"
			case "best of":
				if l.Flag == "" {
					l.Flag = "best_of"
				}
			}
		}
		if hasClass(n, "v3-search-card-pill") {
			num, label := "", ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == xhtml.ElementNode && c.Data == "strong" {
					num = strings.TrimSpace(text(c))
				} else if c.Type == xhtml.TextNode {
					label += c.Data
				}
			}
			label = strings.ToLower(label)
			switch {
			case strings.Contains(label, "chambre") || strings.Contains(label, "slaapkamer") || strings.Contains(label, "bedroom"):
				if v, err := strconv.Atoi(num); err == nil {
					l.Bedrooms = &v
				}
			case strings.Contains(label, "salle") || strings.Contains(label, "badkamer") || strings.Contains(label, "bath"):
				if v, err := strconv.Atoi(num); err == nil {
					l.Bathrooms = &v
				}
			case strings.Contains(label, "m²") || strings.Contains(label, "m2"):
				if v, err := strconv.ParseFloat(strings.ReplaceAll(num, ",", "."), 64); err == nil {
					l.Surface = &v
				}
			}
		}
		if t := attr(n, "data-value-prouser-types"); t != "" && l.SellerType == "" {
			l.SellerType = t
		}
		if p := attr(n, "data-value-phone"); p != "" && l.Phone == "" {
			l.Phone = p
		}
		if hasClass(n, "v3-search-card-owner-link") {
			if id := attr(n, "data-value-id"); id != "" {
				l.AgencyID = id
			}
		}
	})
	if l.ID == "" {
		return l, false
	}
	if l.EPC != "" {
		if b, err := EPCBand(l.EPC); err == nil {
			l.EPCBand = b
		}
	}
	l.Private = isPrivateSeller(l.SellerType)
	l.PricePerSqm = PricePerSqm(l.Deal, l.Price, l.Surface)
	return l, true
}

func fillFromURL(l *Listing, u string) {
	// /fr/detail/<type>/<transaction>/<postcode>/<slug>/<ref>
	parts := strings.Split(strings.TrimPrefix(u, "https://immovlan.be"), "/")
	if len(parts) >= 8 && parts[2] == "detail" {
		// URL slugs are subtypes (duplex, loft...) that NormalizeType rejects;
		// never overwrite an identity the card or the dataLayer already gave.
		if t, err := NormalizeType(parts[3]); err == nil && l.Type == "" {
			l.Type = t
		}
		if d, err := NormalizeDeal(parts[4]); err == nil && l.Deal == "" {
			l.Deal = d
		}
		if pc := NormalizePostalCode(parts[5]); pc != "" && l.PostalCode == "" {
			l.PostalCode = pc
		}
		if l.ID == "" && refRe.MatchString(parts[7]) {
			l.ID = strings.ToUpper(parts[7])
		}
	}
}

func isPrivateSeller(sellerType string) bool {
	t := strings.ToLower(sellerType)
	return t != "" && !strings.Contains(t, "estateagent") && !strings.Contains(t, "promot") && !strings.Contains(t, "notar")
}

var priceDigits = regexp.MustCompile(`[0-9]`)

func parseEUR(s string) *float64 {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")
	if i := strings.Index(s, ","); i >= 0 {
		s = s[:i]
	}
	digits := strings.Join(priceDigits.FindAllString(s, -1), "")
	if digits == "" {
		return nil
	}
	v, err := strconv.ParseFloat(digits, 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

var epcLetterRe = regexp.MustCompile(`^[A-G](\+\+?)?$`)

// epcFromClass turns "BrusselsF" / "FlandersAPLUS" into a PEB letter; anything
// that is not a letter A-G with an optional + / ++ is dropped.
func epcFromClass(c string) string {
	for _, region := range []string{"Brussels", "Flanders", "Wallonia", "Wallonie", "Vlaanderen", "Bruxelles"} {
		if strings.HasPrefix(c, region) && len(c) > len(region) {
			s := strings.ToUpper(c[len(region):])
			s = strings.ReplaceAll(strings.ReplaceAll(s, "PLUSPLUS", "++"), "PLUS", "+")
			if epcLetterRe.MatchString(s) {
				return s
			}
		}
	}
	return ""
}

// PricePerSqm derives €/m² (or €/m²/month for rentals) with sanity bounds;
// nil when a value is implausible.
func PricePerSqm(deal string, price, surface *float64) *float64 {
	if price == nil || surface == nil || *surface < 10 {
		return nil
	}
	v := *price / *surface
	lo, hi := 300.0, 25000.0
	if deal == DealRent || deal == DealColocation {
		lo, hi = 3, 150
	}
	if v < lo || v > hi {
		return nil
	}
	r := math.Round(v*10) / 10
	return &r
}

var (
	dataLayerRe = regexp.MustCompile(`(?s)dataLayer\.push\((\{.*?\})\s*\|\|`)
)

// ParseDetail extracts a listing page: JSON-LD, dataLayer, meta and the
// feature grid.
func ParseDetail(body []byte) (Detail, error) {
	root, err := xhtml.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Features: map[string]string{}}
	var ld map[string]any
	walk(root, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode {
			return
		}
		if n.Data == "script" && attr(n, "type") == "application/ld+json" && ld == nil {
			var m map[string]any
			if json.Unmarshal([]byte(text(n)), &m) == nil && m["@type"] == "RealEstateListing" {
				ld = m
			}
		}
		if n.Data == "meta" {
			name := attr(n, "name")
			if strings.HasPrefix(name, "cXenseParse:rbf-immovlan-") {
				key := strings.TrimPrefix(name, "cXenseParse:rbf-immovlan-")
				val := attr(n, "content")
				switch key {
				case "peb":
					if letter := epcFromClass(val); letter != "" {
						d.EPC = letter
					}
				case "type":
					if t, err := NormalizeType(val); err == nil && d.Type == "" {
						d.Type = t
					}
				}
			}
		}
		if n.Data == "div" && hasClass(n, "v3-detail-grid-row") {
			label, value := "", ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != xhtml.ElementNode {
					continue
				}
				if hasClass(c, "v3-detail-grid-label") {
					label = strings.Join(strings.Fields(text(c)), " ")
				} else if hasClass(c, "v3-detail-grid-value") {
					value = strings.Join(strings.Fields(text(c)), " ")
				}
			}
			if label != "" && value != "" {
				d.Features[label] = value
			}
		}
		if hasClass(n, "v3-detail-owner-name") && d.Agency == "" {
			d.Agency = strings.Join(strings.Fields(text(n)), " ")
		}
		if n.Data == "a" && hasClass(n, "lnk-pro-website") && d.AgencyID == "" {
			d.AgencyID = attr(n, "data-id")
		}
		if p := attr(n, "data-value-phone"); p != "" && d.Phone == "" {
			d.Phone = p
		}
	})
	if m := dataLayerRe.FindSubmatch(body); m != nil {
		var dl map[string]any
		if json.Unmarshal(m[1], &dl) == nil {
			d.SellerType, _ = dl["seller_type"].(string)
			if id, ok := dl["seller_id"].(string); ok && id != "" {
				d.AgencyID = id
			}
			d.Software, _ = dl["software"].(string)
			// The reference feeds request paths, the store key and a
			// directory name: only accept the site's own reference shape.
			if ref, ok := dl["vlan_code"].(string); ok && refRe.MatchString(ref) {
				d.ID = strings.ToUpper(ref)
			}
			if t, ok := dl["property_type"].(string); ok {
				if v, err := NormalizeType(t); err == nil {
					d.Type = v
				}
			}
			if tr, ok := dl["transaction_type"].(string); ok {
				if v, err := NormalizeDeal(tr); err == nil {
					d.Deal = v
				}
			}
			if z, ok := dl["zip_code"].(string); ok {
				if pc := NormalizePostalCode(z); pc != "" {
					d.PostalCode = pc
				}
			}
			if st, ok := dl["property_sub_type"].(string); ok {
				d.Subtype = st
			}
			if ls, ok := dl["livable_surface"].(string); ok {
				if v, err := strconv.ParseFloat(strings.ReplaceAll(ls, ",", "."), 64); err == nil && v > 0 {
					d.Surface = &v
				}
			}
			// dataLayer prices are machine-formatted ("725000.00"): the dot is a
			// decimal point, never a thousands separator as in the HTML price.
			if pr, ok := dl["price"].(string); ok && d.Price == nil {
				if v, err := strconv.ParseFloat(strings.ReplaceAll(pr, ",", "."), 64); err == nil && v > 0 {
					d.Price = &v
				}
			}
		}
	}
	if ld != nil {
		applyJSONLD(&d, ld)
	}
	if d.URL != "" {
		fillFromURL(&d.Listing, d.URL) // reference from the canonical URL when the dataLayer is absent
	}
	if d.ID == "" {
		return d, ErrNoListing
	}
	if d.URL == "" {
		d.URL = ListingURL(d.ID)
	}
	if d.Type == "" {
		if me, ok := ld["mainEntity"].(map[string]any); ok {
			switch me["@type"] {
			case "House":
				d.Type = "maison"
			case "Apartment":
				d.Type = "appartement"
			}
		}
	}
	d.applyFeatures()
	if d.EPC != "" {
		if b, err := EPCBand(d.EPC); err == nil {
			d.EPCBand = b
		}
	}
	d.Private = isPrivateSeller(d.SellerType)
	d.PricePerSqm = PricePerSqm(d.Deal, d.Price, d.Surface)
	return d, nil
}

func applyJSONLD(d *Detail, ld map[string]any) {
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return s }
	num := func(v any) *float64 {
		switch t := v.(type) {
		case float64:
			return &t
		case string:
			if f, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", "."), 64); err == nil {
				return &f
			}
		}
		return nil
	}
	if u := str(ld, "url"); u != "" && immovlanHost(u) {
		d.URL = u
	}
	if v := str(ld, "datePosted"); v != "" {
		d.CreatedAt = normalizeDate(v)
	}
	if v := str(ld, "name"); v != "" {
		d.Title = strings.Join(strings.Fields(v), " ")
	}
	if v := str(ld, "description"); v != "" {
		d.Description = strings.Join(strings.Fields(v), " ")
	}
	if imgs, ok := ld["image"].([]any); ok {
		for _, i := range imgs {
			if s, ok := i.(string); ok {
				d.Photos = append(d.Photos, s)
			}
		}
	}
	if me, ok := ld["mainEntity"].(map[string]any); ok {
		if a, ok := me["address"].(map[string]any); ok {
			d.Street = str(a, "streetAddress")
			if v := str(a, "addressLocality"); v != "" {
				d.Locality = v
			}
			if v := NormalizePostalCode(str(a, "postalCode")); v != "" {
				d.PostalCode = v
			}
		}
		if g, ok := me["geo"].(map[string]any); ok {
			d.Lat, d.Lng = num(g["latitude"]), num(g["longitude"])
		}
		if fs, ok := me["floorSize"].(map[string]any); ok {
			if v := num(fs["value"]); v != nil && *v > 0 {
				d.Surface = v
			}
		}
		if v := num(me["numberOfBedrooms"]); v != nil {
			n := int(*v)
			d.Bedrooms = &n
		}
		if v := num(me["numberOfBathroomsTotal"]); v != nil {
			n := int(*v)
			d.Bathrooms = &n
		}
		if v := num(me["yearBuilt"]); v != nil && *v > 1000 {
			n := int(*v)
			d.Year = &n
		}
		if props, ok := me["additionalProperty"].([]any); ok {
			for _, p := range props {
				if pm, ok := p.(map[string]any); ok && str(pm, "propertyID") == "propertySubType" && d.Subtype == "" {
					d.Subtype = str(pm, "value")
				}
			}
		}
	}
	if of, ok := ld["offers"].(map[string]any); ok {
		if v := num(of["price"]); v != nil && *v > 0 {
			d.Price = v
		}
		if by, ok := of["offeredBy"].(map[string]any); ok {
			if v := str(by, "name"); v != "" {
				d.Agency = v
			}
			d.AgencyURL = str(by, "url")
			if d.SellerType == "" {
				if t := str(by, "@type"); t == "Person" {
					d.SellerType = "private"
				} else if t != "" {
					d.SellerType = "estateAgents"
				}
			}
		}
	}
}

func (d *Detail) applyFeatures() {
	get := func(keys ...string) string {
		for k, v := range d.Features {
			fk := Fold(k)
			for _, want := range keys {
				if fk == want {
					return v
				}
			}
		}
		return ""
	}
	if v := get("etat du bien", "staat van het goed"); v != "" {
		d.Condition = v
	}
	if v := get("bien actuellement loue", "momenteel verhuurd"); v != "" {
		b := strings.EqualFold(v, "Oui") || strings.EqualFold(v, "Ja") || strings.EqualFold(v, "Yes")
		d.Rented = &b
	}
	if v := get("revenu cadastral", "kadastraal inkomen"); v != "" {
		d.CadastralIncome = parseEUR(v)
	}
	if v := get("annee de construction", "bouwjaar"); v != "" && d.Year == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			d.Year = &n
		}
	}
	if v := get("surface du jardin", "oppervlakte tuin"); v != "" {
		d.Garden = parseSqm(v)
	}
	if v := get("surface de la terrasse", "oppervlakte terras"); v != "" {
		d.Terrace = parseSqm(v)
	}
	if v := get("surface totale du terrain", "totale oppervlakte terrein", "surface du terrain"); v != "" {
		d.Land = parseSqm(v)
	}
	if v := get("surface habitable", "bewoonbare oppervlakte"); v != "" && d.Surface == nil {
		d.Surface = parseSqm(v)
	}
	if v := get("type de chauffage", "type verwarming"); v != "" {
		d.Heating = v
	}
	if v := get("nombre de chambres a coucher", "aantal slaapkamers"); v != "" && d.Bedrooms == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			d.Bedrooms = &n
		}
	}
}

func parseSqm(s string) *float64 {
	s = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), " ", ""))
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "m²"), "m2"))
	s = strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

// normalizeDate returns an RFC3339 UTC date, or "" when the page's value
// does not parse (an unknown date, never a raw string).
func normalizeDate(s string) string {
	for _, layout := range []string{"2006-01-02T15:04:05.9999999", "2006-01-02T15:04:05", time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// immovlanHost reports whether a URL points at immovlan.be over https.
func immovlanHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "immovlan.be" || strings.HasSuffix(h, ".immovlan.be")
}

// DaysListed returns whole days since an RFC3339 date.
func DaysListed(date string, now time.Time) (int, bool) {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return 0, false
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d, true
}

// ---- html helpers

func walk(n *xhtml.Node, fn func(*xhtml.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *xhtml.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func text(n *xhtml.Node) string {
	var b strings.Builder
	walk(n, func(c *xhtml.Node) {
		if c.Type == xhtml.TextNode {
			b.WriteString(c.Data)
		}
	})
	return b.String()
}
