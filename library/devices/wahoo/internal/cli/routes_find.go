// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: offline route finder. Not generated.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type routeMatch struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	DistanceKm  float64 `json:"distance_km"`
	DistanceRaw float64 `json:"distance_raw"`
	AscentM     float64 `json:"ascent_m"`
	DescentM    float64 `json:"descent_m"`
	StartLat    float64 `json:"start_lat,omitempty"`
	StartLng    float64 `json:"start_lng,omitempty"`
	FromKm      float64 `json:"from_km,omitempty"`
}

func newNovelRoutesFindCmd(flags *rootFlags) *cobra.Command {
	var distance string
	var minAscent, maxAscent float64
	var near string
	var radius float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Filter saved routes by distance, climbing, and proximity",
		Long: "Query your saved routes by distance band, total ascent, and proximity to a\n" +
			"point — a route library the Wahoo app can only scroll. Distances are in km and\n" +
			"elevation in meters (the API's stored values are treated as meters; the raw\n" +
			"value is preserved in JSON as distance_raw). For free-text name matching use\n" +
			"'search --type routes' instead.",
		Example: "  wahoo-pp-cli routes find --distance 80-120 --max-ascent 1000 --agent\n" +
			"  wahoo-pp-cli routes find --near 40.7,-74.0 --radius 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			minDist, maxDist, err := parseKmRange(distance)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			var nearLat, nearLng float64
			haveNear := false
			if strings.TrimSpace(near) != "" {
				nearLat, nearLng, err = parseLatLng(near)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(err)
				}
				haveNear = true
				if radius <= 0 {
					radius = 50
				}
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "routes", `[]`) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "routes") {
				hintIfStale(cmd, db, "routes", flags.maxAge)
			}
			matches, err := findRoutes(db, routeFilter{
				minDistKm: minDist, maxDistKm: maxDist,
				minAscent: minAscent, maxAscent: maxAscent,
				haveNear: haveNear, nearLat: nearLat, nearLng: nearLng, radiusKm: radius,
			})
			if err != nil {
				return fmt.Errorf("reading routes: %w", err)
			}
			// Closest-first when --near is set, else longest-first.
			if haveNear {
				sort.SliceStable(matches, func(i, j int) bool { return matches[i].FromKm < matches[j].FromKm })
			} else {
				sort.SliceStable(matches, func(i, j int) bool { return matches[i].DistanceKm > matches[j].DistanceKm })
			}
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(matches) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No saved routes match those filters.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				if haveNear {
					fmt.Fprintln(tw, "NAME\tDIST(km)\tASCENT(m)\tFROM(km)")
					for _, m := range matches {
						fmt.Fprintf(tw, "%s\t%.1f\t%.0f\t%.1f\n", m.Name, m.DistanceKm, m.AscentM, m.FromKm)
					}
				} else {
					fmt.Fprintln(tw, "NAME\tDIST(km)\tASCENT(m)\tDESCENT(m)")
					for _, m := range matches {
						fmt.Fprintf(tw, "%s\t%.1f\t%.0f\t%.0f\n", m.Name, m.DistanceKm, m.AscentM, m.DescentM)
					}
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
		},
	}
	cmd.Flags().StringVar(&distance, "distance", "", "Distance band in km, e.g. 80-120 (or 80- / -120 for open ends)")
	cmd.Flags().Float64Var(&minAscent, "min-ascent", 0, "Minimum total ascent in meters")
	cmd.Flags().Float64Var(&maxAscent, "max-ascent", 0, "Maximum total ascent in meters (0 = no max)")
	cmd.Flags().StringVar(&near, "near", "", "Center point as LAT,LNG for proximity search")
	cmd.Flags().Float64Var(&radius, "radius", 0, "Radius in km around --near (default 50 when --near is set)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum routes to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

type routeFilter struct {
	minDistKm, maxDistKm float64
	minAscent, maxAscent float64
	haveNear             bool
	nearLat, nearLng     float64
	radiusKm             float64
}

func findRoutes(db *store.Store, f routeFilter) ([]routeMatch, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'routes'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]routeMatch, 0)
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		distRaw, _ := parseLooseFloat(obj["distance"])
		distKm := distRaw / 1000
		ascent, _ := parseLooseFloat(obj["ascent"])
		descent, _ := parseLooseFloat(obj["descent"])
		m := routeMatch{
			ID:          idString(obj["id"]),
			DistanceRaw: round1(distRaw),
			DistanceKm:  round2(distKm),
			AscentM:     round1(ascent),
			DescentM:    round1(descent),
		}
		if n, ok := obj["name"].(string); ok {
			m.Name = n
		}
		if f.minDistKm > 0 && distKm < f.minDistKm {
			continue
		}
		if f.maxDistKm > 0 && distKm > f.maxDistKm {
			continue
		}
		if f.minAscent > 0 && ascent < f.minAscent {
			continue
		}
		if f.maxAscent > 0 && ascent > f.maxAscent {
			continue
		}
		if f.haveNear {
			lat, latOK := parseLooseFloat(obj["starting_lat"])
			lng, lngOK := parseLooseFloat(obj["starting_lng"])
			if !latOK || !lngOK {
				continue
			}
			from := haversineKm(f.nearLat, f.nearLng, lat, lng)
			if from > f.radiusKm {
				continue
			}
			m.StartLat = lat
			m.StartLng = lng
			m.FromKm = round2(from)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// parseKmRange parses "MIN-MAX", "MIN-", "-MAX", or "" (no constraint). Returns
// (0,0) for an empty string. A bare negative like "-120" means "up to 120".
func parseKmRange(s string) (min, max float64, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, nil
	}
	lo, hi, ok := strings.Cut(s, "-")
	if !ok {
		// Single value: treat as an exact-ish band [v, v] is too strict; treat
		// as a minimum so "100" means "at least 100km".
		v, perr := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if perr != nil {
			return 0, 0, fmt.Errorf("invalid --distance %q: want MIN-MAX in km", s)
		}
		return v, 0, nil
	}
	lo, hi = strings.TrimSpace(lo), strings.TrimSpace(hi)
	if lo != "" {
		if min, err = strconv.ParseFloat(lo, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid --distance lower bound %q", lo)
		}
	}
	if hi != "" {
		if max, err = strconv.ParseFloat(hi, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid --distance upper bound %q", hi)
		}
	}
	if min > 0 && max > 0 && min > max {
		return 0, 0, fmt.Errorf("invalid --distance: lower bound %.0f exceeds upper bound %.0f", min, max)
	}
	return min, max, nil
}

func parseLatLng(s string) (lat, lng float64, err error) {
	a, b, ok := strings.Cut(strings.TrimSpace(s), ",")
	if !ok {
		return 0, 0, fmt.Errorf("invalid --near %q: want LAT,LNG", s)
	}
	if lat, err = strconv.ParseFloat(strings.TrimSpace(a), 64); err != nil {
		return 0, 0, fmt.Errorf("invalid --near latitude %q", a)
	}
	if lng, err = strconv.ParseFloat(strings.TrimSpace(b), 64); err != nil {
		return 0, 0, fmt.Errorf("invalid --near longitude %q", b)
	}
	return lat, lng, nil
}
