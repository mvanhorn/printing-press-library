// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/internal/store"
)

// --- Time-window parsing ---

var sinceUnitRe = regexp.MustCompile(`^(\d+)(s|m|h|d|w)$`)

// parseSinceArg accepts "24h", "7d", "30m", "1w" (relative) OR an ISO-8601
// timestamp (e.g. 2024-01-01 or 2024-01-01T00:00:00). Returns a time.Time in UTC.
func parseSinceArg(arg string) (time.Time, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return time.Time{}, fmt.Errorf("--since requires a value (e.g. 24h, 7d, 2024-01-01)")
	}
	if m := sinceUnitRe.FindStringSubmatch(arg); m != nil {
		n, _ := strconv.Atoi(m[1])
		var dur time.Duration
		switch m[2] {
		case "s":
			dur = time.Duration(n) * time.Second
		case "m":
			dur = time.Duration(n) * time.Minute
		case "h":
			dur = time.Duration(n) * time.Hour
		case "d":
			dur = time.Duration(n*24) * time.Hour
		case "w":
			dur = time.Duration(n*24*7) * time.Hour
		}
		return time.Now().UTC().Add(-dur), nil
	}
	// Try absolute ISO 8601 / RFC3339.
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, arg); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse --since value %q (expected 24h, 7d, or ISO 8601 date)", arg)
}

// fdsnTimeFormat returns the canonical FDSN timestamp form (YYYY-MM-DDThh:mm:ss).
func fdsnTimeFormat(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}

// --- Haversine distance ---

const earthRadiusKm = 6371.0

// haversineKm returns the great-circle distance between two lat/lon points in km.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// --- Local store access ---

// openLocalStore opens the printed CLI's SQLite database. Returns (nil, nil)
// when the DB does not exist yet (caller should treat that as "no local data").
func openLocalStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("usgs-earthquakes-pp-cli")
	return store.OpenWithContext(ctx, dbPath)
}

// localEventByID loads a single event from the local store by ID.
// Returns (nil, nil) when not found.
func localEventByID(ctx context.Context, db *store.Store, id string) (map[string]any, error) {
	row := db.DB().QueryRowContext(ctx,
		`SELECT data FROM resources WHERE resource_type='events' AND id=?`, id)
	var raw sql.NullString
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if !raw.Valid {
		return nil, nil
	}
	var feature map[string]any
	if err := json.Unmarshal([]byte(raw.String), &feature); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return feature, nil
}

// localEventCoords extracts (lat, lon, depth_km, time_ms) from a stored event feature.
// time_ms is properties.time (Unix ms). Returns ok=false when any value is missing or malformed.
func localEventCoords(feature map[string]any) (lat, lon, depthKm float64, timeMs int64, ok bool) {
	geom, _ := feature["geometry"].(map[string]any)
	if geom == nil {
		return 0, 0, 0, 0, false
	}
	coords, _ := geom["coordinates"].([]any)
	if len(coords) < 3 {
		return 0, 0, 0, 0, false
	}
	lonV, lonOK := coords[0].(float64)
	latV, latOK := coords[1].(float64)
	depV, depOK := coords[2].(float64)
	if !lonOK || !latOK || !depOK {
		return 0, 0, 0, 0, false
	}
	props, _ := feature["properties"].(map[string]any)
	tMs, _ := props["time"].(float64)
	return latV, lonV, depV, int64(tMs), true
}

// --- Composite editorial scoring ---

// alertWeight maps PAGER alert level to a numeric weight for composite scoring.
// green=1, yellow=4, orange=10, red=25, unknown=1.
func alertWeight(alert string) float64 {
	switch strings.ToLower(alert) {
	case "green":
		return 1
	case "yellow":
		return 4
	case "orange":
		return 10
	case "red":
		return 25
	}
	return 1
}

// compositeScore = sig * alertWeight(alert) * (1 + ln(1+felt)) * (1 + 2*tsunami).
func compositeScore(sig float64, alert string, felt int64, tsunami int64) float64 {
	if sig <= 0 {
		sig = 1
	}
	feltTerm := 1 + math.Log1p(float64(felt))
	tsunamiTerm := 1 + 2*float64(tsunami)
	return sig * alertWeight(alert) * feltTerm * tsunamiTerm
}
