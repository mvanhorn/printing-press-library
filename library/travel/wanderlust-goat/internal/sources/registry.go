// Package sources is the typed registry of every data source the GOAT stack
// fans out to. Each Source carries trust, tier, and the locale signals the
// goat-score formula needs. There is no shared Source interface — clients
// live in their own packages — but every client agrees to populate one of
// these rows when it returns a result.
package sources

// Tier classifies a source by editorial weight. Higher tiers always win
// ties in goat-score. The trust weights below are the per-source numbers
// from the brief.
type Tier int

const (
	TierFoundation Tier = iota // Geocode, routing, OSM tags
	TierEditorial              // Curated by humans (Michelin, NYT 36 Hours, Wikipedia, Eater, Time Out, Wikivoyage)
	TierRegional               // Local-language regional sources (Tabelog, Naver, Le Fooding) — get +country_boost
	TierHidden                 // Atlas Obscura
	TierCrowd                  // Reddit
)

// Country is an ISO 3166-1 alpha-2 code or "*" (universal).
type Country string

const (
	CountryUniversal Country = "*"
	CountryJapan     Country = "JP"
	CountryKorea     Country = "KR"
	CountryFrance    Country = "FR"
)

// Source describes one data source in the registry.
type Source struct {
	// Name is the display name (Nominatim, Tabelog, Le Fooding).
	Name string
	// Slug is the kebab-case identifier (matches the internal package name).
	Slug string
	// Tier classifies the source.
	Tier Tier
	// Trust is the brief's per-source trust weight, 0-1.
	Trust float64
	// Country is the country this source is canonical for; CountryUniversal
	// for sources that work everywhere (Nominatim, Overpass, OSRM, Wikipedia).
	Country Country
	// Locale is the lang code the source returns (en, ja, ko, fr).
	Locale string
	// CountryMatchBoost is added to the trust when the location's country
	// equals Country. Brief: "Local-language sources get a +0.05 boost".
	CountryMatchBoost float64
	// Intents lists the place intents this source returns.
	Intents []Intent
	// Stub is true for sources whose v1 implementation is intentionally light
	// (sitemap discovery only, deferred body extraction). They count toward
	// coverage and dispatch but rank lower until promoted.
	Stub bool
	// StubReason is the user-facing explanation when Stub is true.
	StubReason string
}

// Intent is a per-place classification used in scoring and dispatch.
type Intent string

const (
	IntentFood     Intent = "food"
	IntentCoffee   Intent = "coffee"
	IntentCulture  Intent = "culture"
	IntentHistoric Intent = "historic"
	IntentViewpoint Intent = "viewpoint"
	IntentShopping Intent = "shopping"
	IntentNature   Intent = "nature"
	IntentDrinks   Intent = "drinks"
	IntentLodging  Intent = "lodging"
)

// Registry is the v1 source list. Every typed Client in the build registers
// here so dispatch, coverage, and scoring all read from one place.
var Registry = []Source{
	// Foundation — universal, key-less, deterministic.
	{Name: "Nominatim", Slug: "nominatim", Tier: TierFoundation, Trust: 1.00, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCulture, IntentHistoric, IntentLodging, IntentShopping, IntentNature}},
	{Name: "OSM Overpass", Slug: "overpass", Tier: TierFoundation, Trust: 0.90, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCoffee, IntentCulture, IntentHistoric, IntentViewpoint, IntentShopping, IntentNature, IntentDrinks, IntentLodging}},
	{Name: "OSRM", Slug: "osrm", Tier: TierFoundation, Trust: 1.00, Country: CountryUniversal, Locale: "en"},

	// Editorial — curated by humans.
	{Name: "Michelin Guide", Slug: "michelin", Tier: TierEditorial, Trust: 0.95, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood}},
	{Name: "NYT 36 Hours", Slug: "nyt36hours", Tier: TierEditorial, Trust: 0.95, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCulture, IntentHistoric, IntentDrinks}},
	{Name: "Wikipedia", Slug: "wikipedia", Tier: TierEditorial, Trust: 0.95, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentCulture, IntentHistoric, IntentNature}},
	{Name: "Eater", Slug: "eater", Tier: TierEditorial, Trust: 0.90, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCoffee, IntentDrinks}},
	{Name: "Time Out", Slug: "timeout", Tier: TierEditorial, Trust: 0.85, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCulture, IntentDrinks, IntentShopping}},
	{Name: "Wikivoyage", Slug: "wikivoyage", Tier: TierEditorial, Trust: 0.85, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentCulture, IntentFood, IntentHistoric}},

	// Regional — local-language sources for v1 (JP + KR + FR).
	{Name: "Tabelog", Slug: "tabelog", Tier: TierRegional, Trust: 0.90, Country: CountryJapan, Locale: "ja", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentCoffee}},
	{Name: "Le Fooding", Slug: "lefooding", Tier: TierRegional, Trust: 0.90, Country: CountryFrance, Locale: "fr", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentDrinks}},
	{Name: "Naver Map", Slug: "navermap", Tier: TierRegional, Trust: 0.85, Country: CountryKorea, Locale: "ko", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentCoffee, IntentShopping}},
	{Name: "Naver Blog", Slug: "naverblog", Tier: TierRegional, Trust: 0.80, Country: CountryKorea, Locale: "ko", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentCulture}},

	// Regional v1 stubs (real package, sitemap-only surface, deferred body extraction).
	{Name: "Kakao Map", Slug: "kakaomap", Tier: TierRegional, Trust: 0.80, Country: CountryKorea, Locale: "ko", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentShopping}, Stub: true, StubReason: "v1 ships package shell + listing surface; rich body extraction deferred to v2 (KR signal already covered by Naver Map)"},
	{Name: "MangoPlate", Slug: "mangoplate", Tier: TierRegional, Trust: 0.75, Country: CountryKorea, Locale: "ko", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood}, Stub: true, StubReason: "v1 ships read-only listing surface; service has been winding down public coverage"},
	{Name: "4travel", Slug: "fourtravel", Tier: TierRegional, Trust: 0.70, Country: CountryJapan, Locale: "ja", CountryMatchBoost: 0.05, Intents: []Intent{IntentCulture}, Stub: true, StubReason: "v1 ships sitemap discovery; rich blog-body extraction deferred to v2"},
	{Name: "Retty", Slug: "retty", Tier: TierRegional, Trust: 0.75, Country: CountryJapan, Locale: "ja", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood}, Stub: true, StubReason: "v1 ships sitemap discovery; rich body extraction deferred to v2"},
	{Name: "Hot Pepper", Slug: "hotpepper", Tier: TierRegional, Trust: 0.75, Country: CountryJapan, Locale: "ja", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood, IntentDrinks}, Stub: true, StubReason: "v1 ships public-listing scrape shell; rich data is behind official API key (deferred to v2)"},
	{Name: "Note.com", Slug: "notecom", Tier: TierRegional, Trust: 0.70, Country: CountryJapan, Locale: "ja", CountryMatchBoost: 0.05, Intents: []Intent{IntentCulture, IntentFood}, Stub: true, StubReason: "v1 ships search-only shell; body extraction deferred to v2"},
	{Name: "Pudlo", Slug: "pudlo", Tier: TierRegional, Trust: 0.85, Country: CountryFrance, Locale: "fr", CountryMatchBoost: 0.05, Intents: []Intent{IntentFood}, Stub: true, StubReason: "v1 ships sitemap discovery; rich body extraction deferred to v2"},

	// Hidden — Atlas Obscura.
	{Name: "Atlas Obscura", Slug: "atlasobscura", Tier: TierHidden, Trust: 0.80, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentCulture, IntentHistoric, IntentNature, IntentViewpoint}},

	// Crowd — Reddit (≥10 upvotes, ≥3 comments filter applied client-side).
	{Name: "Reddit", Slug: "reddit", Tier: TierCrowd, Trust: 0.75, Country: CountryUniversal, Locale: "en", Intents: []Intent{IntentFood, IntentCoffee, IntentCulture, IntentDrinks, IntentShopping, IntentViewpoint, IntentNature}},
}

// BySlug returns the Source row for a slug, or nil.
func BySlug(slug string) *Source {
	for i := range Registry {
		if Registry[i].Slug == slug {
			return &Registry[i]
		}
	}
	return nil
}

// ForCountry returns sources canonical for the given country plus all
// universal sources, in tier order. Used by dispatch and fanout.
func ForCountry(c Country) []Source {
	var out []Source
	for _, s := range Registry {
		if s.Country == CountryUniversal || s.Country == c {
			out = append(out, s)
		}
	}
	return out
}

// Score computes the per-source contribution to a place's goat-score.
// Formula: trust × (1 + country_boost_if_matches) × intent_match × walking_decay.
// `walking_decay = 1 / (1 + walking_minutes/15)`.
// `intent_match` is 1.0 if the place's intent is in the source's Intents,
// else 0.5.
func (s Source) Score(placeCountry Country, walkingMinutes float64, intent Intent) float64 {
	boost := 0.0
	if s.CountryMatchBoost > 0 && s.Country == placeCountry {
		boost = s.CountryMatchBoost
	}
	intentMatch := 0.5
	for _, allowed := range s.Intents {
		if allowed == intent {
			intentMatch = 1.0
			break
		}
	}
	walkDecay := 1.0 / (1.0 + walkingMinutes/15.0)
	return s.Trust * (1.0 + boost) * intentMatch * walkDecay
}
