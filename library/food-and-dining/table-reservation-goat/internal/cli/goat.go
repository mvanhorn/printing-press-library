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
}

type goatResponse struct {
	Query     string       `json:"query"`
	Results   []goatResult `json:"results"`
	Errors    []string     `json:"errors,omitempty"`
	Sources   []string     `json:"sources_queried"`
	QueriedAt string       `json:"queried_at"`
}

// newGoatCmd is the headline transcendence command: a single query that hits
// OpenTable's Autocomplete and Tock's venue search simultaneously, merges
// results into one ranked list, and returns agent-shaped JSON. This is the
// single command an agent should reach for when asked to find a table.
func newGoatCmd(flags *rootFlags) *cobra.Command {
	var (
		latitude  float64
		longitude float64
		metro     string
		network   string
		limit     int
		party     int
		when      string
	)
	cmd := &cobra.Command{
		Use:     "goat <query>",
		Short:   "Cross-network unified restaurant search (OpenTable + Tock)",
		Long:    "Search OpenTable and Tock simultaneously and return one ranked list. Use this any time an agent or user needs a restaurant search that crosses both reservation networks.",
		Example: "  table-reservation-goat-pp-cli goat 'omakase' --metro seattle --party 6 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			// `--metro <slug>` resolves to lat/lng for autocomplete unless
			// explicit lat/lng is provided. Without this, queries without
			// geo defaulted to NYC midtown (40.7589, -73.9851) — so
			// `goat 'tasting menu' --metro seattle` previously returned
			// New York results.
			if metro != "" && latitude == 0 && longitude == 0 {
				if lat, lng, ok := metroLatLng(metro); ok {
					latitude, longitude = lat, lng
				} else {
					return fmt.Errorf("unknown metro %q (known: %s)", metro, strings.Join(knownMetros(), ", "))
				}
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), goatResponse{
					Query: query,
					Results: []goatResult{
						{Network: "opentable", Name: "(dry-run sample)", MatchScore: 1.0},
					},
					Sources:   []string{"opentable", "tock"},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			ctx := cmd.Context()
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
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Geo-narrowed search latitude (defaults to NYC unless --metro is set)")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Geo-narrowed search longitude (defaults to NYC unless --metro is set)")
	cmd.Flags().StringVar(&metro, "metro", "", "Metro slug (seattle, chicago, new-york, san-francisco, los-angeles, ...) — sets lat/lng for autocomplete")
	cmd.Flags().StringVar(&network, "network", "", "Restrict to one network (opentable, tock); default queries both")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max merged results to return")
	cmd.Flags().IntVar(&party, "party", 2, "Party size (informational; OT autocomplete does not filter on this)")
	cmd.Flags().StringVar(&when, "when", "", "Time hint for search (e.g., 'fri 7-9pm', 'tonight', 'this weekend'); informational in v1")
	_ = when
	return cmd
}

// metroLatLng resolves a known metro slug to a representative lat/lng for
// the OpenTable Autocomplete geo-narrowing. Returns ok=false on unknown
// slugs so the caller can return a clear error.
func metroLatLng(slug string) (lat, lng float64, ok bool) {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "seattle":
		return 47.6062, -122.3321, true
	case "chicago":
		return 41.8781, -87.6298, true
	case "new-york", "new-york-city", "nyc", "manhattan":
		return 40.7589, -73.9851, true
	case "san-francisco", "sf":
		return 37.7749, -122.4194, true
	case "los-angeles", "la":
		return 34.0522, -118.2437, true
	case "miami":
		return 25.7617, -80.1918, true
	case "boston":
		return 42.3601, -71.0589, true
	case "washington-dc", "dc", "washington":
		return 38.9072, -77.0369, true
	case "austin":
		return 30.2672, -97.7431, true
	case "portland":
		return 45.5152, -122.6784, true
	case "denver":
		return 39.7392, -104.9903, true
	case "philadelphia", "philly":
		return 39.9526, -75.1652, true
	case "atlanta":
		return 33.7490, -84.3880, true
	case "houston":
		return 29.7604, -95.3698, true
	case "dallas":
		return 32.7767, -96.7970, true
	case "san-diego":
		return 32.7157, -117.1611, true
	case "minneapolis":
		return 44.9778, -93.2650, true
	case "nashville":
		return 36.1627, -86.7816, true
	case "new-orleans", "nola":
		return 29.9511, -90.0715, true
	case "las-vegas", "vegas":
		return 36.1699, -115.1398, true
	}
	return 0, 0, false
}

// metroCityName resolves a metro slug to its display name. Tock's city-search
// URL accepts both: the slug appears as the path segment (`/city/seattle/`)
// while the display name appears as the `?city=` query param (`?city=Seattle`).
// Returns "" on unknown slug so callers can fall back.
func metroCityName(slug string) string {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "seattle":
		return "Seattle"
	case "chicago":
		return "Chicago"
	case "new-york", "new-york-city", "nyc", "manhattan":
		// Tock's canonical slug is `new-york-city`; the display name `New York City`
		// produces that slug via citySlugFromName, which matches the URL the
		// /city/new-york-city/search SSR responds at.
		return "New York City"
	case "san-francisco", "sf":
		return "San Francisco"
	case "los-angeles", "la":
		return "Los Angeles"
	case "miami":
		return "Miami"
	case "boston":
		return "Boston"
	case "washington-dc", "dc", "washington":
		return "Washington DC"
	case "austin":
		return "Austin"
	case "portland":
		return "Portland"
	case "denver":
		return "Denver"
	case "philadelphia", "philly":
		return "Philadelphia"
	case "atlanta":
		return "Atlanta"
	case "houston":
		return "Houston"
	case "dallas":
		return "Dallas"
	case "san-diego":
		return "San Diego"
	case "minneapolis":
		return "Minneapolis"
	case "nashville":
		return "Nashville"
	case "new-orleans", "nola":
		return "New Orleans"
	case "las-vegas", "vegas":
		return "Las Vegas"
	}
	return ""
}

func knownMetros() []string {
	return []string{
		"seattle", "chicago", "new-york", "san-francisco", "los-angeles",
		"miami", "boston", "washington-dc", "austin", "portland", "denver",
		"philadelphia", "atlanta", "houston", "dallas", "san-diego",
		"minneapolis", "nashville", "new-orleans", "las-vegas",
	}
}

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

// _ keeps cliutil imported for future limiter wiring.
var _ = cliutil.IsVerifyEnv
