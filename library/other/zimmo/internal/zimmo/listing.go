// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package zimmo

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// PriceChange is one entry of Zimmo's own price history.
type PriceChange struct {
	Date   string  `json:"date"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// Document is an attached PDF (EPC, urbanism, pre-emption...).
type Document struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Listing is the flat, CLI-facing view of a Zimmo listing. Field names
// follow immoweb-pp-cli / immovlan-pp-cli where the concepts match.
type Listing struct {
	Code            string        `json:"zimmo_code"`
	ID              string        `json:"id"`
	URL             string        `json:"url"`
	Status          string        `json:"status"`
	Type            string        `json:"type"`
	SubType         string        `json:"subtype,omitempty"`
	IsProject       bool          `json:"is_project,omitempty"`
	Price           *float64      `json:"price"`
	PricePerM2      *float64      `json:"price_per_m2"`
	Street          string        `json:"street,omitempty"`
	Number          string        `json:"number,omitempty"`
	Box             string        `json:"box,omitempty"`
	Address         string        `json:"address"`
	PostalCode      string        `json:"postal_code"`
	Locality        string        `json:"locality"`
	PlaceIDs        []int         `json:"place_ids,omitempty"`
	Lat             *float64      `json:"lat"`
	Lng             *float64      `json:"lng"`
	GeoPrecision    string        `json:"geo_precision,omitempty"`
	Bedrooms        *int          `json:"bedrooms"`
	Bathrooms       *int          `json:"bathrooms"`
	Surface         *float64      `json:"surface_m2"`
	Plot            *float64      `json:"plot_m2"`
	Year            *int          `json:"construction_year"`
	Condition       string        `json:"condition,omitempty"`
	Facades         *int          `json:"facades,omitempty"`
	NewBuild        *bool         `json:"new_build,omitempty"`
	EPC             string        `json:"epc"`
	EPCKWh          *float64      `json:"epc_kwh_m2"`
	EPCNumber       string        `json:"epc_certificate,omitempty"`
	EPCSuspect      bool          `json:"epc_kwh_suspect,omitempty"`       // kWh/m² above 1500: almost always an input error
	RenovationDuty  string        `json:"renovation_obligation,omitempty"` // YES | NO | UNKNOWN
	RentPerYear     *float64      `json:"rent_per_year"`                   // current rental income when the property is let
	Rented          bool          `json:"rented"`
	FloodRisk       []string      `json:"flood_risk,omitempty"`     // flooding types with status YES
	PlanningFlags   []string      `json:"planning_flags,omitempty"` // planning types with status YES
	Subpoena        string        `json:"subpoena,omitempty"`       // YES | NO | UNKNOWN
	FreeOn          string        `json:"free_on,omitempty"`
	SoldBy          string        `json:"sold_by,omitempty"`
	Agency          string        `json:"agency,omitempty"`
	AgencyID        string        `json:"agency_id,omitempty"`
	AgencyPhone     string        `json:"agency_phone,omitempty"`
	AgencyURL       string        `json:"agency_url,omitempty"`
	AgencyScore     *float64      `json:"agency_review_score,omitempty"`
	PublishedAt     string        `json:"published_at,omitempty"`
	CreatedAt       string        `json:"created_at,omitempty"`
	UpdatedAt       string        `json:"updated_at,omitempty"`
	DaysOnMarket    *int          `json:"days_on_market"`
	PriceHistory    []PriceChange `json:"price_history,omitempty"`
	TotalCutPct     *float64      `json:"total_cut_pct,omitempty"`
	MainImage       string        `json:"main_image,omitempty"`
	Photos          []string      `json:"photos,omitempty"`
	Documents       []Document    `json:"documents,omitempty"`
	Description     string        `json:"description,omitempty"`
	SaleFlags       []string      `json:"sale_flags,omitempty"` // viager, bare_ownership, usufruct, public_sale, shared_ownership
	VirtualVisitURL string        `json:"virtual_visit_url,omitempty"`
}

// rawListing is the subset of Zimmo's listing JSON we read.
type rawListing struct {
	ID        string          `json:"id"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	Estate    *rawEstate      `json:"estate"`
	Project   json.RawMessage `json:"project"`
	Dealer    *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		PhoneNumber string `json:"phoneNumber"`
		Website     string `json:"website"`
		Enriched    struct {
			URL     map[string]string `json:"url"`
			Reviews struct {
				Combined struct {
					Score float64 `json:"score"`
				} `json:"combined"`
			} `json:"reviews"`
		} `json:"enriched"`
	} `json:"dealer"`
	Options struct {
		Publication struct {
			Start string `json:"start"`
		} `json:"publication"`
	} `json:"options"`
	Files struct {
		Images []struct {
			Order      int               `json:"order"`
			Thumbnails map[string]string `json:"thumbnails"`
		} `json:"images"`
		Documents []Document `json:"documents"`
	} `json:"files"`
	Enriched struct {
		Bedrooms      *int              `json:"bedrooms"`
		Bathrooms     *int              `json:"bathrooms"`
		PlaceIDs      []int             `json:"placeIds"`
		PlaceName     map[string]string `json:"placeName"`
		DetailPageURL map[string]string `json:"detailPageUrl"`
		MainImageURL  string            `json:"mainImageUrl"`
		Description   map[string]string `json:"description"`
		Coordinates   *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"coordinates"`
	} `json:"enriched"`
	Extra struct {
		Code      string `json:"code"`
		ZimmoCode string `json:"zimmoCode"`
	} `json:"extra"`
	PriceHistory []struct {
		Date string `json:"date"`
		Diff struct {
			Before float64 `json:"before"`
			After  float64 `json:"after"`
		} `json:"diff"`
	} `json:"priceHistory"`
}

type valueUnit struct {
	Value *float64 `json:"value"`
	Unit  string   `json:"unit"`
}

type rawEstate struct {
	Location struct {
		Street       string            `json:"street"`
		StreetNumber string            `json:"streetNumber"`
		StreetBus    string            `json:"streetBus"`
		PostalCode   string            `json:"postalCode"`
		Locality     map[string]string `json:"locality"`
		Coordinates  struct {
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
			Precision string   `json:"precision"`
		} `json:"coordinates"`
	} `json:"location"`
	Description       map[string]string `json:"description"`
	Type              string            `json:"type"`
	SubType           string            `json:"subType"`
	Status            string            `json:"status"`
	Price             *valueUnit        `json:"price"`
	FloorspaceSurface *valueUnit        `json:"floorspaceSurface"`
	Plot              *struct {
		PlotSurface *valueUnit `json:"plotSurface"`
	} `json:"plot"`
	ConstructionYear *int       `json:"constructionYear"`
	Condition        string     `json:"condition"`
	FacadesCount     *int       `json:"facadesCount"`
	NewConstruction  *bool      `json:"newConstruction"`
	SoldBy           string     `json:"soldBy"`
	RentPerYear      *valueUnit `json:"rentPerYear"`
	VirtualVisitURL  string     `json:"virtualVisitUrl"`
	FreeOn           struct {
		Status string `json:"freeOnStatus"`
	} `json:"freeOn"`
	Certificate struct {
		EPC struct {
			Status               string     `json:"certificateStatus"`
			Number               string     `json:"epcCertificateNumber"`
			Value                *valueUnit `json:"epcValue"`
			EnergyLabel          string     `json:"energyLabel"`
			RenovationObligation string     `json:"renovationObligation"`
		} `json:"epcCertificate"`
	} `json:"certificate"`
	Flooding struct {
		Values []struct {
			Type   string `json:"floodingType"`
			Status string `json:"floodingStatus"`
		} `json:"floodingValues"`
	} `json:"flooding"`
	Planning []struct {
		Type   string `json:"planningType"`
		Status string `json:"planningStatus"`
	} `json:"planning"`
}

func pickLang(m map[string]string, lang string) string {
	if v := m[lang]; v != "" {
		return v
	}
	for _, k := range []string{"fr", "nl", "en", "de"} {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}

func titleCase(s string) string {
	if s == "" || strings.ToUpper(s) != s {
		return s
	}
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r, n := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(words, " ")
}

// Parse flattens one listing JSON object. lang picks fr/nl/en texts.
func Parse(raw json.RawMessage, lang string, now time.Time) (Listing, error) {
	var r rawListing
	if err := json.Unmarshal(raw, &r); err != nil {
		return Listing{}, fmt.Errorf("decoding listing: %w", err)
	}
	if lang == "" {
		lang = "fr"
	}
	l := Listing{
		ID:          r.ID,
		Code:        r.Extra.ZimmoCode,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		PublishedAt: r.Options.Publication.Start,
		PlaceIDs:    r.Enriched.PlaceIDs,
		Bedrooms:    r.Enriched.Bedrooms,
		Bathrooms:   r.Enriched.Bathrooms,
		MainImage:   r.Enriched.MainImageURL,
		IsProject:   len(r.Project) > 0 && string(r.Project) != "null",
		Documents:   r.Files.Documents,
	}
	if l.Code == "" {
		l.Code = r.Extra.Code
	}
	l.URL = pickLang(r.Enriched.DetailPageURL, lang)
	if r.Dealer != nil {
		l.AgencyID = r.Dealer.ID
		l.Agency = r.Dealer.Name
		l.AgencyPhone = r.Dealer.PhoneNumber
		l.AgencyURL = pickLang(r.Dealer.Enriched.URL, lang)
		if s := r.Dealer.Enriched.Reviews.Combined.Score; s > 0 {
			l.AgencyScore = &s
		}
	}
	imgs := r.Files.Images
	sort.SliceStable(imgs, func(i, j int) bool { return imgs[i].Order < imgs[j].Order })
	for _, im := range imgs {
		u := im.Thumbnails["z-detail-large"]
		if u == "" {
			u = im.Thumbnails["z-detail-1300"]
		}
		if u == "" {
			u = im.Thumbnails["z-result"]
		}
		if u != "" {
			l.Photos = append(l.Photos, u)
		}
	}
	if e := r.Estate; e != nil {
		loc := e.Location
		l.Street = loc.Street
		l.Number = loc.StreetNumber
		l.Box = loc.StreetBus
		l.PostalCode = loc.PostalCode
		l.Locality = titleCase(pickLang(loc.Locality, lang))
		if loc.Coordinates.Latitude != nil && loc.Coordinates.Longitude != nil {
			l.Lat, l.Lng = loc.Coordinates.Latitude, loc.Coordinates.Longitude
			l.GeoPrecision = loc.Coordinates.Precision
		}
		l.Status = e.Status
		l.Type = e.Type
		l.SubType = e.SubType
		if e.Price != nil && e.Price.Value != nil && *e.Price.Value > 0 {
			l.Price = e.Price.Value
		}
		if e.FloorspaceSurface != nil && e.FloorspaceSurface.Value != nil && *e.FloorspaceSurface.Value > 0 {
			l.Surface = e.FloorspaceSurface.Value
		}
		if e.Plot != nil && e.Plot.PlotSurface != nil && e.Plot.PlotSurface.Value != nil && *e.Plot.PlotSurface.Value > 0 {
			l.Plot = e.Plot.PlotSurface.Value
		}
		l.Year = e.ConstructionYear
		l.Condition = e.Condition
		l.Facades = e.FacadesCount
		l.NewBuild = e.NewConstruction
		l.SoldBy = e.SoldBy
		l.FreeOn = e.FreeOn.Status
		l.VirtualVisitURL = e.VirtualVisitURL
		if e.RentPerYear != nil && e.RentPerYear.Value != nil && *e.RentPerYear.Value > 0 {
			l.RentPerYear = e.RentPerYear.Value
			l.Rented = true
		}
		cert := e.Certificate.EPC
		l.EPC = NormalizeEPC(cert.EnergyLabel)
		if cert.Value != nil && cert.Value.Value != nil && *cert.Value.Value > 0 {
			l.EPCKWh = cert.Value.Value
			l.EPCSuspect = *cert.Value.Value > 1500
		}
		l.EPCNumber = cert.Number
		l.RenovationDuty = cert.RenovationObligation
		for _, f := range e.Flooding.Values {
			if f.Status == "YES" {
				l.FloodRisk = append(l.FloodRisk, f.Type)
			}
		}
		for _, p := range e.Planning {
			if p.Type == "SUBPOENA" {
				l.Subpoena = p.Status
			}
			if p.Status == "YES" {
				l.PlanningFlags = append(l.PlanningFlags, p.Type)
			}
		}
		l.Description = pickLang(e.Description, lang)
	}
	if l.Description == "" {
		l.Description = pickLang(r.Enriched.Description, lang)
	}
	if l.Locality == "" {
		l.Locality = titleCase(pickLang(r.Enriched.PlaceName, lang))
	}
	if l.Lat == nil && r.Enriched.Coordinates != nil && (r.Enriched.Coordinates.Lat != 0 || r.Enriched.Coordinates.Lon != 0) {
		lat, lon := r.Enriched.Coordinates.Lat, r.Enriched.Coordinates.Lon
		l.Lat, l.Lng = &lat, &lon
	}
	l.Address = FormatAddress(l.Street, l.Number, l.Box, l.PostalCode, l.Locality)
	if l.Price != nil && l.Surface != nil && *l.Surface > 0 {
		pps := math.Round(*l.Price / *l.Surface)
		l.PricePerM2 = &pps
	}
	for _, ph := range r.PriceHistory {
		l.PriceHistory = append(l.PriceHistory, PriceChange{Date: ph.Date, Before: ph.Diff.Before, After: ph.Diff.After})
	}
	sort.SliceStable(l.PriceHistory, func(i, j int) bool { return l.PriceHistory[i].Date < l.PriceHistory[j].Date })
	if n := len(l.PriceHistory); n > 0 && l.PriceHistory[0].Before > 0 {
		last := l.PriceHistory[n-1].After
		cut := math.Round((l.PriceHistory[0].Before-last)/l.PriceHistory[0].Before*1000) / 10
		l.TotalCutPct = &cut
	}
	if start := firstNonEmpty(l.PublishedAt, l.CreatedAt); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil && !now.IsZero() {
			d := int(now.Sub(t).Hours() / 24)
			if d < 0 {
				d = 0
			}
			l.DaysOnMarket = &d
		}
	}
	if r.Estate != nil {
		l.SaleFlags = saleFlags(r.Estate.Description, l.Description)
	}
	if l.IsProject && l.Status == "" {
		l.Status, l.Type = "PROJECT", "PROJECT"
	}
	if l.Code == "" && l.ID == "" {
		return l, fmt.Errorf("listing has no id")
	}
	return l, nil
}

// NormalizeEPC turns Zimmo's enum (A_PLUS_PLUS, D_MINUS) into the label
// printed on certificates (A++, D-).
func NormalizeEPC(label string) string {
	l := strings.ToUpper(strings.TrimSpace(label))
	l = strings.ReplaceAll(l, "_PLUS", "+")
	l = strings.ReplaceAll(l, "_MINUS", "-")
	return l
}

// EPCLetter is the bare letter of a normalised label ("D-" -> "D").
func EPCLetter(label string) string {
	if label == "" {
		return ""
	}
	return label[:1]
}

var saleFlagWords = []struct{ flag, word string }{
	{"viager", "viager"}, {"viager", "lijfrente"}, {"viager", "rente viagère"}, {"viager", "bouquet"},
	{"bare_ownership", "nue-propriété"}, {"bare_ownership", "nue propriété"}, {"bare_ownership", "naakte eigendom"},
	{"usufruct", "usufruit"}, {"usufruct", "vruchtgebruik"},
	{"public_sale", "vente publique"}, {"public_sale", "openbare verkoop"}, {"public_sale", "biddit"},
	{"shared_ownership", "quote-part"}, {"shared_ownership", "en indivision"}, {"shared_ownership", "onverdeeld"},
}

// saleFlags detects sale forms whose asking price is not a market price.
func saleFlags(texts map[string]string, extra string) []string {
	var b strings.Builder
	for _, t := range texts {
		b.WriteString(strings.ToLower(t))
		b.WriteByte(' ')
	}
	b.WriteString(strings.ToLower(extra))
	all := b.String()
	seen := map[string]bool{}
	var out []string
	for _, fw := range saleFlagWords {
		if !seen[fw.flag] && strings.Contains(all, fw.word) {
			seen[fw.flag] = true
			out = append(out, fw.flag)
		}
	}
	return out
}

// FormatAddress joins the address parts the Belgian way.
func FormatAddress(street, number, box, postcode, locality string) string {
	var b strings.Builder
	if street != "" {
		b.WriteString(street)
		if number != "" {
			b.WriteString(" " + number)
		}
		if box != "" {
			b.WriteString(" bte " + box)
		}
	}
	tail := strings.TrimSpace(postcode + " " + locality)
	if tail != "" {
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(tail)
	}
	return b.String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Median returns the median of v (false when empty).
func Median(v []float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2], true
	}
	return (s[n/2-1] + s[n/2]) / 2, true
}

// Quantile returns the q-quantile (0..1) using linear interpolation.
func Quantile(v []float64, q float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo], true
	}
	return s[lo] + (s[hi]-s[lo])*(pos-float64(lo)), true
}

// Normalize re-applies derived fields to a listing decoded from an older
// store row (labels, suspect kWh, sale-form flags).
func (l *Listing) Normalize() {
	l.EPC = NormalizeEPC(l.EPC)
	if l.EPCKWh != nil {
		l.EPCSuspect = *l.EPCKWh > 1500
	}
	if l.SaleFlags == nil {
		l.SaleFlags = saleFlags(nil, l.Description)
	}
	// Days on market is relative to today, not to when the row was stored.
	if start := firstNonEmpty(l.PublishedAt, l.CreatedAt); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			d := int(time.Since(t).Hours() / 24)
			if d < 0 {
				d = 0
			}
			l.DaysOnMarket = &d
		}
	}
}
