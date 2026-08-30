// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Flighty Airports catalog access: store-or-live catalog fetch
// and identifier resolution (IATA / slug / name / city / K-ICAO).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/flighty/internal/store"
)

// flightyCatalogPage is the SSR page the meltdown-map catalog lives on.
const flightyCatalogPage = "/airports"

// flightyCatalogLive fetches the catalog live from the SSR homepage and
// extracts the region-annotated airport array.
func flightyCatalogLive(ctx context.Context, flags *rootFlags) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.GetWithHeaders(ctx, flightyCatalogPage, nil, nil)
	if err != nil {
		return nil, err
	}
	if isDryRunResponseForClient(c, raw) {
		return raw, nil
	}
	return extractFlightyRSC("catalog", raw)
}

// flightyCatalogLocal reads synced catalog rows from the local mirror.
func flightyCatalogLocal(ctx context.Context, dbPath string) ([]flightyCatalogAirport, error) {
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `
		SELECT data FROM resources
		WHERE resource_type IN ('airports', 'airports-tv')`)
	if err != nil {
		return nil, fmt.Errorf("querying local catalog: %w", err)
	}
	byIATA := map[string]flightyCatalogAirport{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning local catalog row: %w", err)
		}
		var ap flightyCatalogAirport
		if err := json.Unmarshal([]byte(data), &ap); err != nil {
			continue // skip malformed rows
		}
		key := strings.ToUpper(strings.TrimSpace(ap.IATA))
		if key == "" {
			key = ap.Slug
		}
		if existing, seen := byIATA[key]; seen && existing.Region != "" && ap.Region == "" {
			continue
		}
		byIATA[key] = ap
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating local catalog: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local catalog rows: %w", err)
	}
	out := make([]flightyCatalogAirport, 0, len(byIATA))
	for _, ap := range byIATA {
		out = append(out, ap)
	}
	return out, nil
}

// flightyCatalogForFlags returns the catalog honoring the --data-source
// strategy: local reads the store only; auto/live fetch live with a
// network-fallback to the store. A missing local mirror under local mode
// returns an empty catalog with a sync hint on stderr.
func flightyCatalogForFlags(ctx context.Context, flags *rootFlags, dbPath string, hintWriter interface{ Write([]byte) (int, error) }) ([]flightyCatalogAirport, error) {
	if flags.dataSource == "local" {
		rows, err := flightyCatalogLocal(ctx, dbPath)
		if err != nil {
			fmt.Fprint(os.Stderr, "no local catalog. run: flighty-pp-cli sync --resources airports --full\n")
			return []flightyCatalogAirport{}, err
		}
		if len(rows) == 0 {
			fmt.Fprint(os.Stderr, "no local catalog. run: flighty-pp-cli sync --resources airports --full\n")
		}
		return rows, nil
	}
	rows, err := flightyCatalogLive(ctx, flags)
	if err == nil {
		var parsed []flightyCatalogAirport
		if err := json.Unmarshal(rows, &parsed); err != nil {
			return nil, fmt.Errorf("parsing live catalog: %w", err)
		}
		return parsed, nil
	}
	if flags.dataSource == "live" {
		return nil, err
	}
	// auto: network error — fall back to the store.
	local, localErr := flightyCatalogLocal(ctx, dbPath)
	if localErr != nil {
		return nil, fmt.Errorf("API unreachable and no local data. Run 'flighty-pp-cli sync --resources airports --full' to enable offline access.\n\nOriginal error: %w", err)
	}
	return local, nil
}

// flightyResolveAirport matches a user query (IATA, slug, name, city, or
// K-prefixed ICAO like KDEN) against the catalog. Preference order: exact
// IATA, exact slug, then substring matches on name/city (shortest name wins).
func flightyResolveAirport(airports []flightyCatalogAirport, query string) (*flightyCatalogAirport, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	if q == "" {
		return nil, fmt.Errorf("airport query is empty")
	}
	// K-prefixed ICAO nicety: KDEN -> DEN for US airports.
	if len(q) == 4 && strings.HasPrefix(q, "K") {
		if ap := flightyFindByIATA(airports, q[1:]); ap != nil {
			return ap, nil
		}
	}
	if ap := flightyFindByIATA(airports, q); ap != nil {
		return ap, nil
	}
	qLower := strings.ToLower(strings.TrimSpace(query))
	for i := range airports {
		if strings.EqualFold(airports[i].Slug, qLower) {
			return &airports[i], nil
		}
	}
	// Substring matches on name and city; prefer exact name, then prefix,
	// then contains — shortest matching name wins for determinism.
	best := -1
	bestRank := 99
	bestLen := 1 << 30
	for i := range airports {
		rank, ok := flightyMatchRank(airports[i].Name, airports[i].City, qLower)
		if !ok {
			continue
		}
		nameLen := len(airports[i].Name)
		if rank < bestRank || (rank == bestRank && nameLen < bestLen) {
			best = i
			bestRank = rank
			bestLen = nameLen
		}
	}
	if best >= 0 {
		return &airports[best], nil
	}
	return nil, fmt.Errorf("no tracked airport matches %q; run 'flighty-pp-cli airports list' to see tracked airports", query)
}

func flightyFindByIATA(airports []flightyCatalogAirport, iata string) *flightyCatalogAirport {
	for i := range airports {
		if strings.EqualFold(strings.TrimSpace(airports[i].IATA), iata) {
			return &airports[i]
		}
	}
	return nil
}

// flightyMatchRank returns 1 for exact name, 2 for name prefix, 3 for exact
// city, 4 for city prefix, 5 for substring, and ok=false when nothing matches.
func flightyMatchRank(name, city, q string) (int, bool) {
	nameLower := strings.ToLower(name)
	cityLower := strings.ToLower(city)
	switch {
	case nameLower == q:
		return 1, true
	case strings.HasPrefix(nameLower, q):
		return 2, true
	case cityLower == q:
		return 3, true
	case strings.HasPrefix(cityLower, q):
		return 4, true
	case strings.Contains(nameLower, q), strings.Contains(cityLower, q):
		return 5, true
	}
	return 0, false
}

// flightyResolveSlugForFetch resolves a user identifier to the URL slug the
// site's detail/board routes require, fetching the catalog with the
// --data-source strategy (local/auto/live). E.g. "den" -> "denver-intl-den".
// Under local availability the resolution is FTS-backed via SearchAirports,
// avoiding a full catalog load for single-airport lookups.
func flightyResolveSlugForFetch(ctx context.Context, flags *rootFlags, query string) (string, error) {
	dbPath := defaultDBPath("flighty-pp-cli")
	if flags.dataSource != "live" {
		if db, err := store.OpenReadOnlyContext(ctx, dbPath); err == nil {
			rows, searchErr := db.SearchAirports(ctx, query, 5)
			db.Close()
			if searchErr == nil && len(rows) > 0 {
				if ap, ok := flightyBestAirportMatch(rows, query); ok {
					if ap.Slug != "" {
						return ap.Slug, nil
					}
				}
			}
		}
	}
	airports, err := flightyCatalogForFlags(ctx, flags, dbPath, os.Stderr)
	if err != nil {
		return "", err
	}
	ap, err := flightyResolveAirport(airports, query)
	if err != nil {
		return "", err
	}
	if ap.Slug == "" {
		return "", fmt.Errorf("airport %q has no URL slug in the catalog", query)
	}
	return ap.Slug, nil
}

// flightyBestAirportMatch picks the best catalog match from FTS result rows
// using the same ranking as flightyResolveAirport.
func flightyBestAirportMatch(rows []json.RawMessage, query string) (*flightyCatalogAirport, bool) {
	candidates := make([]flightyCatalogAirport, 0, len(rows))
	for _, raw := range rows {
		var ap flightyCatalogAirport
		if json.Unmarshal(raw, &ap) != nil || ap.IATA == "" {
			continue
		}
		candidates = append(candidates, ap)
	}
	if len(candidates) == 0 {
		return nil, false
	}
	best, err := flightyResolveAirport(candidates, query)
	if err != nil {
		return nil, false
	}
	return best, true
}

// flightyHaversineKm computes great-circle distance in kilometers between two
// lat/lon points. Catalog coordinates are plain JSON numbers.
func flightyHaversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKm = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthKm * math.Asin(math.Min(1, math.Sqrt(a)))
}
