package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.

// pp:client-call — `goat` reaches the OpenTable SSR client and the Tock client
// through `internal/source/opentable` and `internal/source/tock`. Dogfood's
// reimplementation_check sibling-import regex matches a single path segment
// after `internal/`, so multi-segment paths under `internal/source/...` aren't
// recognized as a client signal. Documented carve-out per AGENTS.md.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// goatResult is one merged row from a cross-network search.
type goatResult struct {
	Network      string  `json:"network"`
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug,omitempty"`
	Cuisine      string  `json:"cuisine,omitempty"`
	Neighborhood string  `json:"neighborhood,omitempty"`
	Metro        string  `json:"metro,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	URL          string  `json:"url,omitempty"`
	MatchScore   float64 `json:"match_score"`
	// MetroCentroidDistanceKm is populated by applyGeoFilter when a metro
	// centroid is set. Agents can use this to verify a result is actually
	// in the expected metro (issue #406 failure 1: wrong-city venues
	// previously surfaced as "available" with no geo context).
	MetroCentroidDistanceKm float64 `json:"metro_centroid_distance_km,omitempty"`
}

type goatResponse struct {
	Query   string       `json:"query"`
	Results []goatResult `json:"results"`
	Errors  []string     `json:"errors,omitempty"`
	Sources []string     `json:"sources_queried"`
	// LocationResolved is the U5 typed-resolution annotation populated when
	// the caller passed --location (or legacy --metro) and ResolveLocation
	// returned a GeoContext. Omitted on the no-constraint path so existing
	// JSON consumers see no field-shape change.
	LocationResolved *LocationResolvedField `json:"location_resolved,omitempty"`
	// LocationWarning is attached alongside LocationResolved when the
	// resolution had material ambiguity (MEDIUM tier) or the caller
	// forced past LOW with --batch-accept-ambiguous. Omitted when the resolve
	// landed on HIGH or when no location was passed.
	LocationWarning *LocationWarningField `json:"location_warning,omitempty"`
	QueriedAt       string                `json:"queried_at"`
}

// newGoatCmd is the headline transcendence command: a single query that hits
// OpenTable's Autocomplete and Tock's venue search simultaneously, merges
// results into one ranked list, and returns agent-shaped JSON. This is the
// single command an agent should reach for when asked to find a table.
func newGoatCmd(flags *rootFlags) *cobra.Command {
	var (
		latitude            float64
		longitude           float64
		metro               string
		network             string
		limit               int
		party               int
		when                string
		metroRadiusKm       float64
		listMetros          bool
		flagLocation        string
		flagAcceptAmbiguous bool
	)
	cmd := &cobra.Command{
		Use:     "goat <query>",
		Short:   "Cross-network unified restaurant search (OpenTable + Tock + Resy)",
		Long:    "Search OpenTable, Tock, and Resy simultaneously and return one ranked list. Use this any time an agent or user needs a restaurant search that crosses all three reservation networks.",
		Example: "  table-reservation-goat-pp-cli goat 'omakase' --metro seattle --party 6 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Hydrate the metro registry from Tock's metroArea SSR
			// before resolving --metro. Cached for 24h on disk so this
			// is free after the first call. Silent on failure — falls
			// back to the 20-entry static registry. (issue #406 failure 3)
			//
			// Loaded here (not in init) so it benefits from the active
			// auth.Session and respects ctx cancellation. Runs ahead of
			// --list-metros / args check so the dumped registry includes
			// the dynamic 250-metro list when available.
			if loadedSession, sessErr := auth.Load(); sessErr == nil {
				hydrateMetrosFromTock(ctx, loadedSession)
			}

			// --list-metros: dump the full hydrated registry as JSON
			// and exit. Agents can enumerate available --metro values
			// without parsing the on-disk cache file. Issue #406:
			// agents need a programmatic way to discover whether a
			// target city (like Bellevue WA) is a standalone metro or
			// rolled into a parent.
			if listMetros {
				// Single registry snapshot so Total and Metros agree even
				// if a concurrent hydration upgrade fires between calls
				// (PR #425 round-2 Greptile P2: prior shape called
				// getRegistry().All() twice and could TOCTOU-race).
				allMetros := getRegistry().All()
				return printJSONFiltered(cmd.OutOrStdout(), metroListResponse{
					Metros:    allMetros,
					Total:     len(allMetros),
					CityHints: cityHints,
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}

			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")

			// U8: route --location / --metro through the typed
			// ResolveLocation pipeline. The legacy `--metro` flag continues
			// to work as a deprecated alias that implies
			// --batch-accept-ambiguous so back-compat callers never trip the
			// envelope path. When neither flag is set but --latitude /
			// --longitude are passed, the lat/lng pair drives hard-reject
			// as before.
			gc, envelope, locationErr, acceptedAmbiguous := resolveLocationFlags(
				cmd.ErrOrStderr(),
				flagLocation,
				metro,
				flagAcceptAmbiguous,
			)
			if locationErr != nil {
				return locationErr
			}
			if envelope != nil {
				// Disambiguation envelope replaces the result list entirely
				// — the caller must pick a location before the per-network
				// search makes sense.
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			filterMode := metroFilterOff
			if gc != nil {
				// Hard-reject results outside the resolved radius. The
				// explicit-flag intent is authoritative. Use the resolved
				// centroid as the autocomplete anchor unless the caller
				// also passed an explicit --latitude/--longitude override.
				if latitude == 0 && longitude == 0 {
					latitude = gc.Centroid[0]
					longitude = gc.Centroid[1]
				}
				if metroRadiusKm <= 0 || metroRadiusKm == defaultMetroRadiusKm {
					if gc.RadiusKm > 0 {
						metroRadiusKm = gc.RadiusKm
					}
				}
				filterMode = metroFilterHardReject
			} else if latitude != 0 || longitude != 0 {
				// Explicit lat/lng without --location/--metro: hard-reject
				// mode using the provided centroid as the anchor.
				filterMode = metroFilterHardReject
			}
			if dryRunOK(flags) {
				resp := goatResponse{
					Query: query,
					Results: []goatResult{
						{Network: "opentable", Name: "(dry-run sample)", MatchScore: 1.0},
					},
					Sources:   []string{"opentable", "tock"},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}
				resp.LocationResolved, resp.LocationWarning = decorateForList(gc, acceptedAmbiguous)
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			net := strings.ToLower(network)
			results := []goatResult{}
			errors := []string{}
			sources := []string{}

			if net == "" || net == "opentable" {
				sources = append(sources, "opentable")
				otRes, otErr := goatQueryOpenTable(ctx, session, query, latitude, longitude)
				if otErr != nil {
					errors = append(errors, fmt.Sprintf("opentable: %v", otErr))
				} else {
					results = append(results, otRes...)
				}
			}
			if net == "" || net == "tock" {
				sources = append(sources, "tock")
				cityName := metroCityName(metro)
				if cityName == "" && gc != nil {
					// U8: derive the Tock city name from the resolved
					// GeoContext when --location supplied the constraint
					// instead of --metro.
					cityName = gc.ForTock().City
				}
				if cityName == "" {
					cityName = "New York City"
				}
				date := time.Now().UTC().Format("2006-01-02")
				hhmm := "19:00"
				tockRes, tockErr := goatQueryTock(ctx, session, query, cityName, date, hhmm, party, latitude, longitude)
				if tockErr != nil {
					errors = append(errors, fmt.Sprintf("tock: %v", tockErr))
				} else {
					results = append(results, tockRes...)
				}
			}
			// Resy /3/venuesearch/search only requires the public API
			// key (hardcoded shared value on every browser visiting
			// resy.com), not a per-user auth token — discovery is
			// public. Always include Resy in the search fanout so
			// first-run users without `auth login --resy` still see
			// Resy venues and can discover the numeric ids needed for
			// later authenticated commands (book/cancel/availability).
			// Authenticated endpoints stay gated. Pass the resolved
			// GeoContext so Resy's city-code projection (ForResy)
			// drives the search location server-side instead of
			// relying on raw `--metro` slugs.
			if net == "" || net == "resy" {
				sources = append(sources, "resy")
				resyRes, resyErr := goatQueryResy(ctx, session, query, gc, metro)
				if resyErr != nil {
					errors = append(errors, fmt.Sprintf("resy: %v", resyErr))
				} else {
					results = append(results, resyRes...)
				}
			}
			// Geo filter: drop or demote results outside the resolved
			// centroid based on filterMode (#406 failure 1). U8 routes
			// this through the typed GeoContext from ResolveLocation; the
			// explicit --latitude/--longitude path still builds an ad-hoc
			// GeoContext so the post-filter shape is uniform.
			filterCtx := gc
			if filterCtx == nil && filterMode != metroFilterOff {
				radius := metroRadiusKm
				if radius <= 0 {
					radius = defaultMetroRadiusKm
				}
				filterCtx = &GeoContext{
					Centroid: [2]float64{latitude, longitude},
					RadiusKm: radius,
				}
			}
			results = applyGeoFilter(results, filterCtx, filterMode)

			// Rank: match score descending. Ties broken by name for determinism.
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].MatchScore != results[j].MatchScore {
					return results[i].MatchScore > results[j].MatchScore
				}
				return results[i].Name < results[j].Name
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			out := goatResponse{
				Query:     query,
				Results:   results,
				Errors:    errors,
				Sources:   sources,
				QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}
			out.LocationResolved, out.LocationWarning = decorateForList(gc, acceptedAmbiguous)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Geo-narrowed search latitude (defaults to NYC unless --location/--metro is set)")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Geo-narrowed search longitude (defaults to NYC unless --location/--metro is set)")
	cmd.Flags().StringVar(&flagLocation, "location", "",
		"Free-form location: 'bellevue, wa', 'seattle', '47.6,-122.3', or 'seattle metro'. "+
			"Resolves to a typed GeoContext and hard-rejects out-of-region results.")
	cmd.Flags().BoolVar(&flagAcceptAmbiguous, "batch-accept-ambiguous", false,
		"BATCH-ONLY escape hatch: when --location is ambiguous, force-pick the top "+
			"candidate instead of returning a disambiguation envelope. Interactive "+
			"agents must NOT set this — it defeats the disambiguation contract.")
	cmd.Flags().StringVar(&metro, "metro", "",
		"Metro slug (seattle, chicago, new-york, ...). DEPRECATED — use --location <city>. "+
			"Implicit --batch-accept-ambiguous is canonical-only: a single-hit registry lookup "+
			"preserves the legacy result shape; ambiguous or unknown values return the standard "+
			"disambiguation envelope just like --location would.")
	cmd.Flags().StringVar(&network, "network", "",
		"Restrict to one network (opentable, tock, resy); default queries all three "+
			"(Resy search is anonymous-safe, so this works without auth login --resy)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max merged results to return")
	cmd.Flags().IntVar(&party, "party", 2, "Party size (informational; OT autocomplete does not filter on this)")
	cmd.Flags().StringVar(&when, "when", "", "Time hint for search (e.g., 'fri 7-9pm', 'tonight', 'this weekend'); informational in v1")
	cmd.Flags().Float64Var(&metroRadiusKm, "metro-radius-km", defaultMetroRadiusKm,
		"When --metro is set, drop results more than this many km from the metro centroid. Default 50km covers most metros including suburbs.")
	cmd.Flags().BoolVar(&listMetros, "list-metros", false,
		"Print the full hydrated metro registry as JSON (every Tock metro + static fallbacks + city-hint mappings) and exit. Useful for agents discovering valid --metro values programmatically.")
	_ = when
	return cmd
}

// metroListResponse is the JSON shape emitted by `goat --list-metros`.
// Includes the hydrated registry, the static city-hint mappings (so
// agents know which secondary cities roll up under which metro), and
// a queried_at timestamp for cache-age inference.
type metroListResponse struct {
	Metros    []Metro           `json:"metros"`
	Total     int               `json:"total"`
	CityHints map[string]string `json:"city_hints"`
	QueriedAt string            `json:"queried_at"`
}

// metroLatLng, metroCityName, knownMetros all moved to metro_registry.go
// (issue #406): a single declarative registry replaces the 90-line
// triplicate-switch pattern and grows to cover Tock's full 253-metro
// metroArea hydration. Lookups still go through the same functions for
// backward compatibility with existing callers.

func goatQueryOpenTable(ctx context.Context, s *auth.Session, query string, lat, lng float64) ([]goatResult, error) {
	c, err := opentable.New(s)
	if err != nil {
		return nil, err
	}
	if lat == 0 && lng == 0 {
		// Default to NYC midtown if no geo provided.
		lat, lng = 40.7589, -73.9851
	}
	// Use the GraphQL Autocomplete endpoint. OpenTable's /s search and
	// /r/<slug> pages both return a 2.5KB SPA shell to non-Chrome clients —
	// only the home page (/) serves real SSR data, and that data is the home
	// view, not search results. The Autocomplete persisted-query is the only
	// reliable path; it bootstraps CSRF from the home page (one cached fetch
	// per process lifetime) and then queries by term + lat/lng.
	results, err := c.Autocomplete(ctx, query, lat, lng)
	if err != nil {
		return nil, err
	}
	out := make([]goatResult, 0, len(results))
	q := strings.ToLower(query)
	for _, r := range results {
		// Score by match quality. Substring of full query → 0.95;
		// matching just the first token → 0.65; otherwise prefix
		// confidence from the autocomplete API → 0.4.
		score := 0.4
		nameLower := strings.ToLower(r.Name)
		if strings.Contains(nameLower, q) {
			score = 0.95
		} else if firstTok := firstToken(q); firstTok != "" && strings.Contains(nameLower, firstTok) {
			score = 0.65
		}
		// OT autocomplete doesn't return urlSlug; use the restaurant
		// profile path keyed by id, which is the stable canonical link.
		url := ""
		if r.ID != "" {
			url = opentable.Origin + "/restaurant/profile/" + r.ID
		}
		out = append(out, goatResult{
			Network:      "opentable",
			ID:           r.ID,
			Name:         r.Name,
			Metro:        r.MetroName,
			Neighborhood: r.NeighborhoodName,
			Latitude:     r.Latitude,
			Longitude:    r.Longitude,
			URL:          url,
			MatchScore:   score,
		})
	}
	return out, nil
}

func firstToken(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' {
			return s[:i]
		}
	}
	return s
}

func goatQueryTock(ctx context.Context, s *auth.Session, query, cityName, date, hhmm string, partySize int, lat, lng float64) ([]goatResult, error) {
	// Tock's read paths are both SSR-rendered:
	//   1. Slug-direct (`/<slug>`): cheap canonical resolution when the user
	//      types an exact venue slug. Returns 404 for free-text queries.
	//   2. City-search (`/city/<slug>/search?...`): geo-search returning
	//      ~60 venues per metro+date+time+party. Powers broader queries
	//      like `goat 'tasting menu chicago'`.
	// We run slug-direct first (cheap, canonical) and city-search after,
	// then dedupe by slug so a venue surfaced by both paths returns once.
	c, err := tock.New(s)
	if err != nil {
		return nil, err
	}

	out := []goatResult{}
	seenSlugs := map[string]struct{}{}

	// Slug-direct path.
	if slug := slugify(query); slug != "" {
		if detail, derr := c.VenueDetail(ctx, slug); derr == nil {
			if biz, ok := detail["business"].(map[string]any); ok && len(biz) > 0 {
				row := goatResult{
					Network:    "tock",
					MatchScore: 0.95,
					URL:        tock.Origin + "/" + slug,
					Slug:       slug,
				}
				if name, ok := biz["name"].(string); ok {
					row.Name = name
				}
				if id, ok := biz["id"].(float64); ok {
					row.ID = fmt.Sprintf("%d", int(id))
				}
				if city, ok := biz["city"].(string); ok {
					row.Metro = city
				}
				if cuisine, ok := biz["cuisine"].(string); ok {
					row.Cuisine = cuisine
				}
				out = append(out, row)
				seenSlugs[slug] = struct{}{}
			}
		}
		// 404 / non-Tock slug → don't fail; just contribute zero rows from this leg.
	}

	// City-search path. Fall back to NYC defaults when --metro is unset, matching
	// goatQueryOpenTable's existing behavior.
	if cityName == "" {
		cityName = "New York City"
	}
	if lat == 0 && lng == 0 {
		// Match goatQueryOpenTable's NYC fallback. Tock's canonical NYC centroid
		// is ~(40.7128, -74.0060) but the ?city= query param drives metro selection,
		// so midtown coords work fine.
		lat, lng = 40.7589, -73.9851
	}
	if partySize <= 0 {
		partySize = 2
	}
	venues, serr := c.SearchCity(ctx, tock.SearchParams{
		City:      cityName,
		Date:      date,
		Time:      hhmm,
		PartySize: partySize,
		Lat:       lat,
		Lng:       lng,
	})
	if serr != nil {
		// Surface SearchCity errors but keep slug-direct results — partial
		// success beats a hard failure that hides what we did find.
		return out, fmt.Errorf("tock search-city: %w", serr)
	}
	q := strings.ToLower(query)
	for _, v := range venues {
		if _, dup := seenSlugs[v.Slug]; dup {
			continue
		}
		// Score by query match against name + cuisine. Mirror goatQueryOpenTable's
		// scoring so cross-network ranking stays consistent.
		nameLower := strings.ToLower(v.Name)
		cuisineLower := strings.ToLower(v.Cuisine)
		score := 0.4
		if strings.Contains(nameLower, q) {
			score = 0.95
		} else if firstTok := firstToken(q); firstTok != "" && (strings.Contains(nameLower, firstTok) || strings.Contains(cuisineLower, firstTok)) {
			score = 0.65
		}
		out = append(out, goatResult{
			Network:      "tock",
			ID:           fmt.Sprintf("%d", v.ID),
			Name:         v.Name,
			Slug:         v.Slug,
			Cuisine:      v.Cuisine,
			Neighborhood: v.Neighborhood,
			Metro:        v.City,
			Latitude:     v.Latitude,
			Longitude:    v.Longitude,
			URL:          v.URL,
			MatchScore:   score,
		})
		seenSlugs[v.Slug] = struct{}{}
	}
	return out, nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := strings.Builder{}
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && out.Len() > 0 {
				out.WriteRune('-')
				prevDash = true
			}
		}
	}
	res := out.String()
	return strings.TrimSuffix(res, "-")
}

// goatQueryResy runs Resy's venue search and maps results into goatResult
// rows. Anonymous-safe: Resy's /3/venuesearch/search only needs the public
// API key (no per-user auth required for discovery). The `gc` parameter is
// the post-#445 typed GeoContext from ResolveLocation; when set, it drives
// Resy's city-code filter and the in-query metro prefix (see comment
// inside on why the prefix is load-bearing). The legacy `metro` slug is
// still accepted so callers passing only --metro (without --location) get
// the same behavior as before.
func goatQueryResy(ctx context.Context, s *auth.Session, query string, gc *GeoContext, metro string) ([]goatResult, error) {
	creds := resy.Credentials{}
	if s != nil && s.Resy != nil {
		creds.APIKey = s.Resy.APIKey
		creds.AuthToken = s.Resy.AuthToken
		creds.Email = s.Resy.Email
	}
	client := resy.New(creds)
	// Resy's gateway dropped support for the `location` body field
	// (rejected as "Unknown field." HTTP 400 on every call), so the
	// per-call `per_page: 20` limit applies to a global match set. For
	// generic queries like "tasting menu" with --location seattle, the
	// 20 global rows can easily include zero Seattle venues, and the
	// downstream geo filter then leaves us with nothing. Empirically
	// Resy's free-text search DOES honor city words in the query string
	// (live trace: `omakase seattle` returns 10 Seattle venues), so we
	// prepend the city display name to the query when a location is
	// resolved. The original query is preserved for scoring so the
	// match-score reflects the user's intent, not the augmented
	// city-prefix string.
	loc := gc.ForResy()
	effectiveQuery := query
	cityCode := loc.City
	cityDisplay := ""
	if gc != nil {
		cityDisplay, _ = cityAndSlugFromResolvedTo(gc.ResolvedTo)
	}
	if cityDisplay == "" {
		cityDisplay = metroCityName(metro)
	}
	if cityCode == "" {
		cityCode = metroToResyCityCode(metro)
	}
	if cityDisplay != "" {
		effectiveQuery = cityDisplay + " " + query
	}
	venues, err := client.Search(ctx, resy.SearchParams{
		Query: effectiveQuery,
		City:  cityCode,
		Limit: 20,
	})
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	out := make([]goatResult, 0, len(venues))
	for _, v := range venues {
		nameLower := strings.ToLower(v.Name)
		score := 0.4
		if strings.Contains(nameLower, q) {
			score = 0.95
		} else if first := firstToken(q); first != "" && strings.Contains(nameLower, first) {
			score = 0.65
		}
		out = append(out, goatResult{
			Network:    "resy",
			ID:         v.ID,
			Name:       v.Name,
			Slug:       v.Slug,
			Metro:      v.City,
			Latitude:   v.Latitude,
			Longitude:  v.Longitude,
			URL:        v.URL,
			MatchScore: score,
		})
	}
	return out, nil
}

// metroToResyCityCode maps a metro slug to Resy's two/three-letter city
// code. Unknown slugs return empty string, which the Search call treats as
// "no city filter" — agents still get results, just unfiltered.
func metroToResyCityCode(metro string) string {
	switch strings.ToLower(strings.TrimSpace(metro)) {
	case "new-york", "new-york-city", "nyc", "manhattan":
		return "ny"
	case "los-angeles", "la":
		return "la"
	case "san-francisco", "sf", "bay-area":
		return "sf"
	case "chicago", "chi":
		return "chi"
	case "seattle":
		return "sea"
	case "miami":
		return "mia"
	case "washington-dc", "dc", "washington":
		return "dc"
	case "boston", "bos":
		return "bos"
	case "austin", "atx":
		return "atx"
	case "philadelphia", "philly":
		return "phl"
	case "london":
		return "ldn"
	}
	return ""
}

// _ keeps cliutil imported for future limiter wiring.
var _ = cliutil.IsVerifyEnv
