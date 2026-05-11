// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// Curated US place registry — the fallback dataset the resolver uses
// when dynamic Tock SSR hydration hasn't run or doesn't cover a slug.
//
// Inclusion criteria for v1:
//   - Major US metros likely to surface in agent prompts.
//   - Every city that participates in an ambiguous-name disambiguation
//     fixture (R14 F1-F7): Bellevue (WA/NE/KY), Portland (OR/ME),
//     Springfield (MA/IL/MO/OR), Columbia (SC/MO/MD).
//
// Centroids are city-hall / downtown coordinates, not population
// centroids. Populations are 2020 US Census estimates rounded to the
// nearest hundred where the source rounded. RadiusKm is curated, not
// derived: 75 km for metros (covers commuter belt the way OpenTable's
// metro hierarchy does), 25 km for cities (a tight downtown radius so
// city-level ReverseLookup beats metro centroids when a query point
// is genuinely in-city).
//
// ProviderCoverage is intentionally nil for curated entries — it's
// reserved for live values that hydration fills in (Tock's
// BusinessCount). ParentMetro is populated for cities whose
// OpenTable hierarchy puts them under a different metro than they'd
// "feel" geographically — Bellevue WA is its own Tock metro but
// rolls under "seattle" for OpenTable lookups.

var curatedPlaces = []Place{
	// --- Major US metros (RadiusKm=75, Tier=MetroCentroid) ---
	{
		Slug:         "seattle",
		Name:         "Seattle",
		State:        "WA",
		Lat:          47.6062,
		Lng:          -122.3321,
		RadiusKm:     75,
		Population:   753675,
		ContextHints: []string{"PNW", "Puget Sound"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:       "new-york-city",
		Name:       "New York City",
		State:      "NY",
		Lat:        40.7128,
		Lng:        -74.0060,
		RadiusKm:   75,
		Population: 8804190,
		// PATCH: lookupbyname-alias-aware — "new york" added so the
		// alias-aware lookupByNameIn resolves the natural-language
		// truncation users type when referring to NYC. "ny" intentionally
		// omitted — it would collide with the LocCityState state-code.
		// (Prior: U17 removed "manhattan" alias when the dedicated borough
		// entry was carved out below; tighter 10 km radius beats the
		// metro's 75 km via ReverseLookup's smallest-radius tiebreak.)
		Aliases:      []string{"nyc", "new-york", "new york"},
		ContextHints: []string{"NYC metro", "Tri-state"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:         "san-francisco",
		Name:         "San Francisco",
		State:        "CA",
		Lat:          37.7749,
		Lng:          -122.4194,
		RadiusKm:     75,
		Population:   873965,
		Aliases:      []string{"sf"},
		ContextHints: []string{"Bay Area"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:         "los-angeles",
		Name:         "Los Angeles",
		State:        "CA",
		Lat:          34.0522,
		Lng:          -118.2437,
		RadiusKm:     75,
		Population:   3898747,
		Aliases:      []string{"la"},
		ContextHints: []string{"SoCal"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:       "chicago",
		Name:       "Chicago",
		State:      "IL",
		Lat:        41.8781,
		Lng:        -87.6298,
		RadiusKm:   75,
		Population: 2746388,
		Tier:       PlaceTierMetroCentroid,
	},
	{
		Slug:       "omaha",
		Name:       "Omaha",
		State:      "NE",
		Lat:        41.2565,
		Lng:        -95.9345,
		RadiusKm:   75,
		Population: 486051,
		Tier:       PlaceTierMetroCentroid,
	},
	{
		Slug:       "cincinnati",
		Name:       "Cincinnati",
		State:      "OH",
		Lat:        39.1031,
		Lng:        -84.5120,
		RadiusKm:   75,
		Population: 309317,
		Tier:       PlaceTierMetroCentroid,
	},
	// --- Legacy metros required by goat_test.go's metroCityName fixture.
	// Kept as full curated entries (not aliases on a phantom slug) so
	// `--metro washington-dc` and `--metro new-orleans` still work the
	// way pre-U3 CLIs did. RadiusKm + Population are 2020 Census-shape.
	{
		Slug:       "washington-dc",
		Name:       "Washington DC",
		State:      "DC",
		Lat:        38.9072,
		Lng:        -77.0369,
		RadiusKm:   75,
		Population: 689545,
		// U17: removed "dc" and "washington" aliases — they now resolve
		// to the dedicated washington-dc-city entry (12 km radius city
		// tier). The metro stays addressable via its canonical slug.
		Tier: PlaceTierMetroCentroid,
	},
	{
		Slug:       "new-orleans",
		Name:       "New Orleans",
		State:      "LA",
		Lat:        29.9511,
		Lng:        -90.0715,
		RadiusKm:   75,
		Population: 383997,
		Aliases:    []string{"nola"},
		Tier:       PlaceTierMetroCentroid,
	},
	// --- Portland (OR vs ME) — R14 F4 ambiguous-name fixture ---
	{
		Slug:         "portland-or",
		Name:         "Portland",
		State:        "OR",
		Lat:          45.5152,
		Lng:          -122.6784,
		RadiusKm:     75,
		Population:   652503,
		ContextHints: []string{"Pacific Northwest"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:         "portland-me",
		Name:         "Portland",
		State:        "ME",
		Lat:          43.6591,
		Lng:          -70.2568,
		RadiusKm:     75,
		Population:   68408,
		ContextHints: []string{"Greater Portland", "Casco Bay"},
		Tier:         PlaceTierMetroCentroid,
	},
	// --- Springfield (MA/IL/MO/OR) — R14 F5 ---
	{
		Slug:       "springfield-ma",
		Name:       "Springfield",
		State:      "MA",
		Lat:        42.1015,
		Lng:        -72.5898,
		RadiusKm:   75,
		Population: 155929,
		Tier:       PlaceTierMetroCentroid,
	},
	{
		Slug:         "springfield-il",
		Name:         "Springfield",
		State:        "IL",
		Lat:          39.7817,
		Lng:          -89.6501,
		RadiusKm:     75,
		Population:   114394,
		ContextHints: []string{"IL capital"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:       "springfield-mo",
		Name:       "Springfield",
		State:      "MO",
		Lat:        37.2153,
		Lng:        -93.2982,
		RadiusKm:   75,
		Population: 169176,
		Tier:       PlaceTierMetroCentroid,
	},
	{
		Slug:       "springfield-or",
		Name:       "Springfield",
		State:      "OR",
		Lat:        44.0462,
		Lng:        -123.0220,
		RadiusKm:   75,
		Population: 59403,
		Tier:       PlaceTierMetroCentroid,
	},
	// --- Columbia (SC/MO/MD) — R14 F6 ---
	{
		Slug:         "columbia-sc",
		Name:         "Columbia",
		State:        "SC",
		Lat:          34.0007,
		Lng:          -81.0348,
		RadiusKm:     75,
		Population:   137996,
		ContextHints: []string{"SC capital"},
		Tier:         PlaceTierMetroCentroid,
	},
	{
		Slug:       "columbia-mo",
		Name:       "Columbia",
		State:      "MO",
		Lat:        38.9517,
		Lng:        -92.3341,
		RadiusKm:   75,
		Population: 126254,
		Tier:       PlaceTierMetroCentroid,
	},
	{
		Slug:       "columbia-md",
		Name:       "Columbia",
		State:      "MD",
		Lat:        39.2156,
		Lng:        -76.8612,
		RadiusKm:   75,
		Population: 104681,
		Tier:       PlaceTierMetroCentroid,
	},

	// --- Cities (RadiusKm=25, Tier=City) ---
	// Three Bellevues — R14 F1/F2/F3 disambiguation fixture.
	{
		Slug:         "bellevue-wa",
		Name:         "Bellevue",
		State:        "WA",
		Lat:          47.6101,
		Lng:          -122.2015,
		RadiusKm:     25,
		Population:   151854,
		ParentMetro:  map[string]string{"opentable": "seattle"},
		ContextHints: []string{"Seattle metro", "Eastside", "tech hub"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:         "bellevue-ne",
		Name:         "Bellevue",
		State:        "NE",
		Lat:          41.1370,
		Lng:          -95.9145,
		RadiusKm:     25,
		Population:   53178,
		ParentMetro:  map[string]string{"opentable": "omaha"},
		ContextHints: []string{"Omaha metro", "Offutt AFB"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:         "bellevue-ky",
		Name:         "Bellevue",
		State:        "KY",
		Lat:          39.1067,
		Lng:          -84.4744,
		RadiusKm:     25,
		Population:   5563,
		ParentMetro:  map[string]string{"opentable": "cincinnati"},
		ContextHints: []string{"Cincinnati metro", "Northern KY"},
		Tier:         PlaceTierCity,
	},

	// --- U17: NYC boroughs / LA neighborhoods / DC city ---
	// Codex P2-I addresses radius-only filtering with hand-curated
	// tighter Place entries. NYC's 75 km metro circle covers half of
	// Long Island plus most of NJ — a user asking about "manhattan"
	// wants Manhattan, not the tri-state. Each entry's RadiusKm is
	// chosen so a query point inside the borough/neighborhood beats
	// the parent metro via ReverseLookup's smallest-radius tiebreak.
	// ParentMetro hints the per-provider routing (OpenTable and Tock
	// both keep boroughs under "new-york-city"; LA neighborhoods
	// route to "los-angeles"; DC city routes to "washington-dc").
	{
		Slug:       "manhattan",
		Name:       "Manhattan",
		State:      "NY",
		Lat:        40.7831,
		Lng:        -73.9712,
		RadiusKm:   10,
		Population: 1628000,
		Aliases:    []string{"nyc-manhattan"},
		ParentMetro: map[string]string{
			"opentable": "new-york-city",
			"tock":      "new-york-city",
		},
		ContextHints: []string{"NYC borough", "Tri-state", "Midtown / Downtown"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:       "brooklyn",
		Name:       "Brooklyn",
		State:      "NY",
		Lat:        40.6782,
		Lng:        -73.9442,
		RadiusKm:   10,
		Population: 2561000,
		Aliases:    []string{"nyc-brooklyn", "bk"},
		ParentMetro: map[string]string{
			"opentable": "new-york-city",
			"tock":      "new-york-city",
		},
		ContextHints: []string{"NYC borough", "Williamsburg / Dumbo / Park Slope"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:       "queens",
		Name:       "Queens",
		State:      "NY",
		Lat:        40.7282,
		Lng:        -73.7949,
		RadiusKm:   12,
		Population: 2253000,
		Aliases:    []string{"nyc-queens"},
		ParentMetro: map[string]string{
			"opentable": "new-york-city",
			"tock":      "new-york-city",
		},
		ContextHints: []string{"NYC borough", "Astoria / Long Island City / Flushing"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:       "west-hollywood",
		Name:       "West Hollywood",
		State:      "CA",
		Lat:        34.0900,
		Lng:        -118.3617,
		RadiusKm:   5,
		Population: 35000,
		Aliases:    []string{"weho"},
		ParentMetro: map[string]string{
			"opentable": "los-angeles",
			"tock":      "los-angeles",
		},
		ContextHints: []string{"LA metro", "Sunset Strip / Melrose"},
		Tier:         PlaceTierNeighborhood,
	},
	{
		Slug:       "santa-monica",
		Name:       "Santa Monica",
		State:      "CA",
		Lat:        34.0195,
		Lng:        -118.4912,
		RadiusKm:   8,
		Population: 93000,
		ParentMetro: map[string]string{
			"opentable": "los-angeles",
			"tock":      "los-angeles",
		},
		ContextHints: []string{"LA metro", "Westside / Beach"},
		Tier:         PlaceTierCity,
	},
	{
		Slug:       "beverly-hills",
		Name:       "Beverly Hills",
		State:      "CA",
		Lat:        34.0736,
		Lng:        -118.4004,
		RadiusKm:   5,
		Population: 32000,
		ParentMetro: map[string]string{
			"opentable": "los-angeles",
			"tock":      "los-angeles",
		},
		ContextHints: []string{"LA metro", "Rodeo Drive / Westside"},
		Tier:         PlaceTierNeighborhood,
	},
	{
		Slug:       "washington-dc-city",
		Name:       "Washington",
		State:      "DC",
		Lat:        38.9072,
		Lng:        -77.0369,
		RadiusKm:   12,
		Population: 712000,
		// PATCH: lookupbyname-alias-aware — "washington" added so the
		// alias-aware lookupByNameIn resolves the natural-language
		// truncation through the city entry (the tighter 12 km radius,
		// ContextHints set). The "washington-dc" alias is intentionally
		// omitted: it would be shadowed by the existing washington-dc
		// metro slug (which appears earlier in the list and matches
		// first in lookupIn's iteration). Keeping it would invite the
		// wrong mental model.
		Aliases: []string{"dc", "the-district", "washington"},
		ParentMetro: map[string]string{
			"opentable": "washington-dc",
		},
		ContextHints: []string{"DC metro", "Capital region", "Tri-state area"},
		Tier:         PlaceTierCity,
	},
}
