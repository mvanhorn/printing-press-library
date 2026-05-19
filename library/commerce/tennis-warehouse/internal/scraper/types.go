// Package scraper parses Tennis Warehouse HTML pages into typed records.
//
// The site is server-side rendered with rich data-* attributes on product
// cards (data-pcode, data-prod_name, data-gtm_impression_price, etc.) and
// a labeled spec table on detail pages (<td class="SpecsLt|SpecsDk">
// Label:</td><td>Value</td>). All extraction is offline-deterministic
// given saved HTML; no JavaScript execution required.
package scraper

import "time"

// Racquet is a current (new) racquet from the brand catalog or all-racquets index.
type Racquet struct {
	SKU           string    `json:"sku"`
	Brand         string    `json:"brand"`
	Model         string    `json:"model"`
	Price         float64   `json:"price"`
	MSRP          float64   `json:"msrp,omitempty"`
	URL           string    `json:"url"`
	ImageURL      string    `json:"image_url,omitempty"`
	HeadSizeIn2   float64   `json:"head_size_in2,omitempty"`
	StrungWeight  float64   `json:"strung_weight_oz,omitempty"`
	UnstrungOz    float64   `json:"unstrung_weight_oz,omitempty"`
	Balance       string    `json:"balance,omitempty"`
	Swingweight   int       `json:"swingweight,omitempty"`
	Stiffness     int       `json:"stiffness,omitempty"`
	BeamWidth     string    `json:"beam_width_mm,omitempty"`
	StringPattern string    `json:"string_pattern,omitempty"`
	LengthIn      float64   `json:"length_in,omitempty"`
	Composition   string    `json:"composition,omitempty"`
	PowerLevel    string    `json:"power_level,omitempty"`
	StrokeStyle   string    `json:"stroke_style,omitempty"`
	Status        string    `json:"status,omitempty"`
	Rating        float64   `json:"rating,omitempty"`
	Reviews       int       `json:"reviews,omitempty"`
	Description   string    `json:"description,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// UsedModel is a used-racquet model (the SKU has 1-N physical units in stock).
// Most spec fields mirror Racquet because used and new pages share the same
// spec-table shape.
type UsedModel struct {
	PCode         string    `json:"pcode"`
	Brand         string    `json:"brand"`
	Model         string    `json:"model"`
	URL           string    `json:"url"`
	ImageURL      string    `json:"image_url,omitempty"`
	PriceLow      float64   `json:"price_low,omitempty"`
	PriceHigh     float64   `json:"price_high,omitempty"`
	MSRP          float64   `json:"msrp,omitempty"`
	HeadSizeIn2   float64   `json:"head_size_in2,omitempty"`
	StrungWeight  float64   `json:"strung_weight_oz,omitempty"`
	UnstrungOz    float64   `json:"unstrung_weight_oz,omitempty"`
	Balance       string    `json:"balance,omitempty"`
	Swingweight   int       `json:"swingweight,omitempty"`
	Stiffness     int       `json:"stiffness,omitempty"`
	BeamWidth     string    `json:"beam_width_mm,omitempty"`
	StringPattern string    `json:"string_pattern,omitempty"`
	LengthIn      float64   `json:"length_in,omitempty"`
	Composition   string    `json:"composition,omitempty"`
	PowerLevel    string    `json:"power_level,omitempty"`
	StrokeStyle   string    `json:"stroke_style,omitempty"`
	UnitCount     int       `json:"unit_count"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// UsedUnit is a single physical used racquet listed under a UsedModel.
type UsedUnit struct {
	StockCode   string    `json:"stock_code"`
	PCode       string    `json:"pcode"`
	Grade       string    `json:"grade"`
	GripSize    string    `json:"grip_size,omitempty"`
	Price       float64   `json:"price"`
	Notes       string    `json:"notes,omitempty"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// BrandCode maps a human brand to the ccode used on the website.
var BrandCodes = map[string]string{
	"babolat":    "BABRACS",
	"dunlop":     "DUNLOPRACS",
	"head":       "HEADRACS",
	"prokennex":  "KENNEXRACS",
	"prince":     "PRINCERACS",
	"solinco":    "SOLINCORAC",
	"tecnifibre": "TECRACS",
	"volkl":      "VOLKLRACS",
	"wilson":     "WILSONRACS",
	"yonex":      "YONEXRACS",
}

// NewBrandPath maps a human brand to the URL path for the new-racquet brand catalog.
var NewBrandPath = map[string]string{
	"babolat":    "/Babolatracquets.html",
	"dunlop":     "/Dunlopracquets.html",
	"head":       "/Headracquets.html",
	"prokennex":  "/ProKennexracquets.html",
	"prince":     "/Princeracquets.html",
	"solinco":    "/Solincoracquets.html",
	"tecnifibre": "/Tecnifibreracquets.html",
	"volkl":      "/Volklracquets.html",
	"wilson":     "/Wilsonracquets.html",
	"yonex":      "/Yonexracquets.html",
	"mizuno":     "/Mizunoracquets.html",
	"lacoste":    "/Lacosteracquets.html",
}

// AllNewBrands returns the canonical brand slugs the CLI knows about.
func AllNewBrands() []string {
	return []string{
		"babolat", "dunlop", "head", "lacoste", "mizuno", "prince",
		"prokennex", "solinco", "tecnifibre", "volkl", "wilson", "yonex",
	}
}

// AllUsedBrands returns the brands with a used-inventory page.
func AllUsedBrands() []string {
	return []string{
		"babolat", "dunlop", "head", "prince", "prokennex", "solinco",
		"tecnifibre", "volkl", "wilson", "yonex",
	}
}

// GradeRank assigns an ordering to condition grades. Lower is better.
func GradeRank(g string) int {
	switch g {
	case "Unused":
		return 0
	case "Grade A":
		return 1
	case "Grade B":
		return 2
	case "Grade C":
		return 3
	default:
		return 99
	}
}

// GradeLegend returns the official Tennis Warehouse grade definitions
// (curated static reference; not derived from a live endpoint).
var GradeLegend = []struct {
	Grade       string `json:"grade"`
	Description string `json:"description"`
}{
	{"Unused", "Not been hit with but may have minor cosmetic defects."},
	{"Grade A", "Very little use; wear evident on grip and grommets only."},
	{"Grade B", "Used and shows some minor cosmetic wear."},
	{"Grade C", "Clear wear from groundstrokes in multiple places."},
}
