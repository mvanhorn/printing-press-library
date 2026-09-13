package immo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Listing is the flattened, agent-sized view of one Immoweb classified.
// Pointer fields are nil when Immoweb did not publish the value.
type Listing struct {
	ID          int64    `json:"id"`
	URL         string   `json:"url"`
	Deal        string   `json:"deal"`
	Type        string   `json:"type"`
	Subtype     string   `json:"subtype,omitempty"`
	Title       string   `json:"title,omitempty"`
	Locality    string   `json:"locality,omitempty"`
	PostalCode  string   `json:"postal_code,omitempty"`
	Province    string   `json:"province,omitempty"`
	Street      string   `json:"street,omitempty"`
	Lat         *float64 `json:"lat,omitempty"`
	Lng         *float64 `json:"lng,omitempty"`
	Price       *float64 `json:"price,omitempty"`      // sale price, or monthly rent for FOR_RENT
	OldPrice    *float64 `json:"old_price,omitempty"`  // previous asking price when Immoweb reports a reduction
	RentCosts   *float64 `json:"rent_costs,omitempty"` // monthly charges for FOR_RENT
	Bedrooms    *int     `json:"bedrooms,omitempty"`
	Surface     *float64 `json:"surface_m2,omitempty"`
	Land        *float64 `json:"land_m2,omitempty"`
	PricePerSqm *float64 `json:"price_per_m2,omitempty"`
	Agency      string   `json:"agency,omitempty"`
	Private     bool     `json:"private_seller"`
	UnderOption bool     `json:"under_option"`
	NewPrice    bool     `json:"new_price"`
	Flag        string   `json:"flag,omitempty"` // new | under_option | ...
	EPC         string   `json:"epc,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	ModifiedAt  string   `json:"modified_at,omitempty"`
}

// Detail extends Listing with the fields only the classified detail endpoint
// returns.
type Detail struct {
	Listing
	Description      string   `json:"description,omitempty"`
	Bathrooms        *int     `json:"bathrooms,omitempty"`
	ConstructionYear *int     `json:"construction_year,omitempty"`
	Condition        string   `json:"condition,omitempty"`
	Facades          *int     `json:"facades,omitempty"`
	Heating          string   `json:"heating,omitempty"`
	EPCKwhPerSqm     *float64 `json:"epc_kwh_per_m2,omitempty"`
	RenovationOblig  *bool    `json:"renovation_obligation,omitempty"`
	CadastralIncome  *float64 `json:"cadastral_income,omitempty"`
	Garden           *float64 `json:"garden_m2,omitempty"`
	Terrace          *float64 `json:"terrace_m2,omitempty"`
	ParkingIndoor    *int     `json:"parking_indoor,omitempty"`
	ParkingOutdoor   *int     `json:"parking_outdoor,omitempty"`
	Views            *int     `json:"views,omitempty"`
	Bookmarks        *int     `json:"bookmarks,omitempty"`
	AgencyPhone      string   `json:"agency_phone,omitempty"`
	AgencyEmail      string   `json:"agency_email,omitempty"`
	AgencyWebsite    string   `json:"agency_website,omitempty"`
	Pictures         []string `json:"pictures,omitempty"`
	Sold             bool     `json:"sold_or_rented"`
}

// ListingURL is the canonical short link for a listing ID (Immoweb redirects
// it to the full SEO URL).
func ListingURL(id int64) string {
	return "https://www.immoweb.be/en/classified/" + strconv.FormatInt(id, 10)
}

var idInURL = regexp.MustCompile(`(?:^|/)([0-9]{6,10})(?:[/?#]|$)`)

// ParseListingID accepts a bare ID or any Immoweb listing URL.
func ParseListingID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n, nil
	}
	if u, err := url.Parse(s); err == nil && u.Path != "" {
		if m := idInURL.FindStringSubmatch(u.Path); m != nil {
			n, _ := strconv.ParseInt(m[1], 10, 64)
			return n, nil
		}
	}
	return 0, fmt.Errorf("%q is not an Immoweb listing ID or URL", s)
}

type jsonMap = map[string]any

func obj(m jsonMap, key string) jsonMap {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func str(m jsonMap, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func num(m jsonMap, key string) *float64 {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case float64:
		if v == 0 && key != "latitude" && key != "longitude" {
			return nil
		}
		return &v
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f != 0 {
			return &f
		}
	}
	return nil
}

func intp(m jsonMap, key string) *int {
	f := num(m, key)
	if f == nil {
		return nil
	}
	n := int(*f)
	return &n
}

func boolv(m jsonMap, key string) bool {
	if m == nil {
		return false
	}
	b, _ := m[key].(bool)
	return b
}

// ErrNoID marks a payload without a listing id (Immoweb's reply for a
// removed or unknown listing).
var ErrNoID = errors.New("listing without id")

// FromResult flattens one object from search-results / search-results-map /
// similar results.
func FromResult(raw json.RawMessage) (Listing, error) {
	var m jsonMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return Listing{}, fmt.Errorf("decoding listing: %w", err)
	}
	return fromMap(m)
}

func fromMap(m jsonMap) (Listing, error) {
	var l Listing
	idf, ok := m["id"].(float64)
	if !ok || idf <= 0 {
		return l, ErrNoID
	}
	l.ID = int64(idf)
	l.URL = ListingURL(l.ID)
	prop := obj(m, "property")
	loc := obj(prop, "location")
	tx := obj(m, "transaction")
	flags := obj(m, "flags")
	pub := obj(m, "publication")

	l.Type = str(prop, "type")
	l.Subtype = str(prop, "subtype")
	l.Title = strings.TrimSpace(str(prop, "title"))
	l.Locality = CleanLocality(str(loc, "locality"))
	l.PostalCode = str(loc, "postalCode")
	l.Province = str(loc, "province")
	l.Street = strings.TrimSpace(strings.Join(nonEmpty(str(loc, "street"), str(loc, "number")), " "))
	if loc != nil {
		if lat, ok := loc["latitude"].(float64); ok && lat != 0 {
			l.Lat = &lat
		}
		if lng, ok := loc["longitude"].(float64); ok && lng != 0 {
			l.Lng = &lng
		}
	}
	l.Bedrooms = intp(prop, "bedroomCount")
	if l.Bedrooms == nil {
		if b, ok := prop["bedroomCount"].(float64); ok && b == 0 {
			z := 0
			l.Bedrooms = &z
		}
	}
	l.Surface = num(prop, "netHabitableSurface")
	l.Land = num(prop, "landSurface")
	if l.Land == nil {
		l.Land = num(obj(prop, "land"), "surface")
	}
	l.Deal = str(tx, "type")
	if rental := obj(tx, "rental"); rental != nil {
		l.Price = num(rental, "monthlyRentalPrice")
		l.RentCosts = num(rental, "monthlyRentalCosts")
	}
	if l.Price == nil {
		if sale := obj(tx, "sale"); sale != nil {
			l.Price = num(sale, "price")
		}
	}
	if l.Price == nil {
		l.Price = num(obj(m, "price"), "mainValue")
	}
	l.OldPrice = num(obj(tx, "sale"), "oldPrice")
	if l.OldPrice == nil {
		l.OldPrice = num(obj(m, "price"), "oldValue")
	}
	if l.OldPrice != nil && (l.Price == nil || *l.OldPrice <= *l.Price) {
		l.OldPrice = nil
	}
	if cert := obj(tx, "certificates"); cert != nil {
		l.EPC = strings.ToUpper(str(cert, "epcScore"))
	}
	if l.EPC == "" {
		l.EPC = strings.ToUpper(str(tx, "certificate"))
	}
	l.PricePerSqm = PricePerSqm(l.Deal, l.Price, l.Surface)
	l.Agency = str(m, "customerName")
	if l.Agency == "" {
		if cs, ok := m["customers"].([]any); ok && len(cs) > 0 {
			if c0, ok := cs[0].(map[string]any); ok {
				l.Agency = str(c0, "name")
				l.Private = strings.EqualFold(str(c0, "type"), "PRIVATE")
			}
		}
	} else if strings.EqualFold(l.Agency, "PRIVATE") {
		// Immoweb labels private sellers with the literal customerName "PRIVATE".
		l.Private = true
		l.Agency = ""
	}
	l.Flag = str(flags, "main")
	l.UnderOption = l.Flag == "under_option" || boolv(flags, "isUnderOption")
	l.NewPrice = boolv(flags, "isNewPrice") || l.OldPrice != nil
	if sec, ok := flags["secondary"].([]any); ok {
		for _, s := range sec {
			if s == "new_price" {
				l.NewPrice = true
			}
		}
	}
	l.CreatedAt = str(pub, "creationDate")
	l.ModifiedAt = str(pub, "lastModificationDate")
	return l, nil
}

// FromClassified flattens the /classified/get-result/{id} envelope (or the
// bare classified object).
func FromClassified(raw json.RawMessage) (Detail, error) {
	var m jsonMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return Detail{}, fmt.Errorf("decoding classified: %w", err)
	}
	if inner := obj(m, "classified"); inner != nil {
		m = inner
	}
	base, err := fromMap(m)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Listing: base}
	prop := obj(m, "property")
	d.Description = strings.TrimSpace(str(prop, "description"))
	d.Bathrooms = intp(prop, "bathroomCount")
	bld := obj(prop, "building")
	d.ConstructionYear = intp(bld, "constructionYear")
	d.Condition = str(bld, "condition")
	d.Facades = intp(bld, "facadeCount")
	d.Heating = str(obj(prop, "energy"), "heatingType")
	d.Garden = num(prop, "gardenSurface")
	d.Terrace = num(prop, "terraceSurface")
	d.ParkingIndoor = intp(prop, "parkingCountIndoor")
	d.ParkingOutdoor = intp(prop, "parkingCountOutdoor")
	tx := obj(m, "transaction")
	if cert := obj(tx, "certificates"); cert != nil {
		d.EPCKwhPerSqm = num(cert, "primaryEnergyConsumptionPerSqm")
		if v, ok := cert["renovationObligation"].(bool); ok {
			d.RenovationOblig = &v
		}
	}
	d.CadastralIncome = num(obj(tx, "sale"), "cadastralIncome")
	stats := obj(m, "statistics")
	d.Views = intp(stats, "viewCount")
	d.Bookmarks = intp(stats, "bookmarkCount")
	if cs, ok := m["customers"].([]any); ok && len(cs) > 0 {
		if c0, ok := cs[0].(map[string]any); ok {
			d.AgencyPhone = firstNonEmpty(str(c0, "mobileNumber"), str(c0, "phoneNumber"))
			d.AgencyEmail = str(c0, "email")
			d.AgencyWebsite = str(c0, "website")
		}
	}
	d.Sold = boolv(obj(m, "flags"), "isSoldOrRented")
	if media := obj(m, "media"); media != nil {
		if pics, ok := media["pictures"].([]any); ok {
			for _, p := range pics {
				if pm, ok := p.(map[string]any); ok {
					if u := firstNonEmpty(str(pm, "largeUrl"), str(pm, "mediumUrl"), str(pm, "smallUrl")); u != "" {
						d.Pictures = append(d.Pictures, u)
					}
				}
			}
		}
	}
	return d, nil
}

// PictureURLs returns the picture URLs of a classified at the requested size
// (small, medium, large, xl).
func PictureURLs(raw json.RawMessage, size string) ([]string, error) {
	var m jsonMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decoding classified: %w", err)
	}
	if inner := obj(m, "classified"); inner != nil {
		m = inner
	}
	key := map[string]string{"small": "smallUrl", "medium": "mediumUrl", "large": "largeUrl", "xl": "extralargeUrl"}[strings.ToLower(size)]
	if key == "" {
		return nil, fmt.Errorf("unknown picture size %q (use small, medium, large or xl)", size)
	}
	out := []string{}
	if pics, ok := obj(m, "media")["pictures"].([]any); ok {
		for _, p := range pics {
			if pm, ok := p.(map[string]any); ok {
				if u := firstNonEmpty(str(pm, key), str(pm, "largeUrl")); u != "" {
					out = append(out, u)
				}
			}
		}
	}
	return out, nil
}

// DaysListed returns whole days between the listing's creation date (or the
// fallback first-seen time) and now. ok=false when neither is known.
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

func nonEmpty(vals ...string) []string {
	out := []string{}
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// CleanLocality tidies a locality as typed by the advertiser: collapsed
// spaces, and title case when it was written in capitals ("1050 IXELLES").
func CleanLocality(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || strings.ToUpper(s) != s || strings.ToLower(s) == s {
		return s
	}
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		parts := strings.Split(w, "-")
		for j, p := range parts {
			if r := []rune(p); len(r) > 0 {
				parts[j] = strings.ToUpper(string(r[0])) + string(r[1:])
			}
		}
		words[i] = strings.Join(parts, "-")
	}
	return strings.Join(words, " ")
}

// roomWords always describe a single room; flatshareWords also appear on
// whole flats (in Belgian French "chambre" means bedroom: "appartement
// 1 chambre à louer" is a whole flat), so they only count when the rent per
// bedroom is low. Both match whole words only ("bedrooms in" is not "rooms in").
var roomWords = []string{"kot", "studentroom", "student room", "studentenkamer", "rooms in", "room in", "furnished room",
	"shared house", "shared accommodation", "shared housing", "house to share", "maison partagée", "gedeelde woning",
	"kamer te huur", "room for rent", "rooms for rent", "coliving", "co-living"}
var flatshareWords = []string{"colocation", "cohousing", "co-housing", "flatshare", "flat share", "shared apartment", "shared flat",
	"shared apt", "chambre à louer", "chambres à louer", "chambre meublée", "chambres meublées"}

// IsRoomLet reports student rooms (subtype KOT) and rentals priced per room:
// a rent below 175 EUR per bedroom for 3+ bedrooms, a room title, or a
// flat-share title with a rent below 500 EUR per bedroom. Such listings
// distort whole-home medians, yields and rankings.
func IsRoomLet(l Listing) bool {
	if strings.EqualFold(l.Subtype, "KOT") {
		return true
	}
	if l.Deal != "FOR_RENT" || l.Price == nil {
		return false
	}
	beds := 1.0
	if l.Bedrooms != nil && *l.Bedrooms > 0 {
		beds = float64(*l.Bedrooms)
	}
	if beds >= 3 && *l.Price < 175*beds {
		return true
	}
	title := strings.ToLower(l.Title)
	for _, w := range roomWords {
		if containsWord(title, w) {
			return true
		}
	}
	if *l.Price/beds < 500 {
		for _, w := range flatshareWords {
			if containsWord(title, w) {
				return true
			}
		}
	}
	return false
}

// containsWord reports whether phrase occurs in s with no letter or digit
// directly before or after it.
func containsWord(s, phrase string) bool {
	for from := 0; ; {
		i := strings.Index(s[from:], phrase)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(phrase)
		before, _ := utf8.DecodeLastRuneInString(s[:start])
		after, _ := utf8.DecodeRuneInString(s[end:])
		if (start == 0 || !isWordRune(before)) && (end == len(s) || !isWordRune(after)) {
			return true
		}
		from = start + 1
	}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
