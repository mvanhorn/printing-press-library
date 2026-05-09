// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// StoreDistance is a nearby store result with distance in km.
type StoreDistance struct {
	ID         string  `json:"id"`
	Country    string  `json:"country"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	City       string  `json:"city"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	DistanceKm float64 `json:"distance_km"`
}

func newStoresNearestCmd(flags *rootFlags) *cobra.Command {
	var lat float64
	var lon float64
	var limit int
	var country string
	var maxKm float64

	cmd := &cobra.Command{
		Use:   "nearest",
		Short: "Find the closest IQOS stores to any GPS coordinate",
		Long: `Query the local synced store catalog to find nearby IQOS retail locations.
Works offline using coordinates extracted from store pages.

Requires a synced store catalog: iqos-pp-cli sync --country us`,
		Example: strings.Trim(`
  iqos-pp-cli stores nearest --lat 30.27 --lon -97.74 --limit 3 --json
  iqos-pp-cli stores nearest --lat 51.50 --lon -0.12 --limit 5 --agent
  iqos-pp-cli stores nearest --lat 48.85 --lon 2.35 --country fr`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lat == 0 && lon == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli stores nearest --lat <lat> --lon <lon>")
				return cmd.Help()
			}

			db, err := iqosOpenDB(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			stores, err := queryNearestStores(cmd.Context(), db.DB(), lat, lon, country, limit, maxKm)
			if err != nil {
				return fmt.Errorf("querying stores: %w", err)
			}

			if len(stores) == 0 {
				msg := "No stores found in the local database"
				if country != "" {
					msg += " for " + country
				}
				msg += ". Run 'iqos-pp-cli sync --country us' to populate the store catalog."
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			data, err := json.Marshal(stores)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude of the reference point (e.g. 30.27)")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude of the reference point (e.g. -97.74)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of stores to return")
	cmd.Flags().StringVar(&country, "country", "", "Limit to stores in a specific country")
	cmd.Flags().Float64Var(&maxKm, "max-km", 0, "Maximum distance in km (0 = no limit)")
	return cmd
}

// haversineKm returns the great-circle distance in kilometers between two lat/lon points.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthR = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthR * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// queryNearestStores loads stores from SQLite and sorts by distance.
// SQLite has no native spatial index; for ≤ tens of thousands of stores
// the full-table-scan + in-process sort is fast enough.
func queryNearestStores(ctx context.Context, rawDB *sql.DB, lat, lon float64, country string, limit int, maxKm float64) ([]StoreDistance, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `SELECT id, country, COALESCE(name,''), COALESCE(address,''), COALESCE(city,''), lat, lon
	          FROM iqos_stores WHERE lat != 0 AND lon != 0`
	args := []interface{}{}
	if country != "" {
		query += " AND country=?"
		args = append(args, country)
	}

	rows, err := rawDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []StoreDistance
	for rows.Next() {
		var s StoreDistance
		if err := rows.Scan(&s.ID, &s.Country, &s.Name, &s.Address, &s.City, &s.Lat, &s.Lon); err != nil {
			continue
		}
		s.DistanceKm = math.Round(haversineKm(lat, lon, s.Lat, s.Lon)*100) / 100
		if maxKm > 0 && s.DistanceKm > maxKm {
			continue
		}
		all = append(all, s)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].DistanceKm < all[j].DistanceKm
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// saveIQOSStores upserts store rows (with coordinates) into iqos_stores.
func saveIQOSStores(ctx context.Context, db interface {
	DB() *sql.DB
}, rows []iqosStoreRow) error {
	rawDB := db.DB()
	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO iqos_stores (id, country, name, address, city, lat, lon, data, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '{}', CURRENT_TIMESTAMP)
		ON CONFLICT(id, country) DO UPDATE SET
			name=excluded.name, address=excluded.address, city=excluded.city,
			lat=excluded.lat, lon=excluded.lon, synced_at=excluded.synced_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.ID, r.Country, r.Name, r.Address, r.City, r.Lat, r.Lon); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type iqosStoreRow struct {
	ID      string
	Country string
	Name    string
	Address string
	City    string
	Lat     float64
	Lon     float64
}

// parseStoreCoords extracts lat/lon from a schema.org/Store JSON-LD block.
func parseStoreCoords(html string) (lat, lon float64) {
	const needle = `"@type":"Place"`
	lower := strings.ToLower(html)
	remaining := lower
	for {
		idx := strings.Index(remaining, "geo")
		if idx < 0 {
			break
		}
		seg := remaining[idx:]
		// Look for "latitude": number pattern
		var la, lo float64
		latIdx := strings.Index(seg, "latitude")
		lonIdx := strings.Index(seg, "longitude")
		if latIdx < 0 || lonIdx < 0 {
			remaining = seg[1:]
			continue
		}
		// Simple numeric extraction
		_, _ = fmt.Sscanf(extractJSONNumber(seg, latIdx), "%f", &la)
		_, _ = fmt.Sscanf(extractJSONNumber(seg, lonIdx), "%f", &lo)
		if la != 0 && lo != 0 {
			return la, lo
		}
		remaining = seg[1:]
	}
	_ = needle
	return 0, 0
}

// extractJSONNumber extracts the first numeric value after offset in a string.
func extractJSONNumber(s string, offset int) string {
	s = s[offset:]
	colonIdx := strings.Index(s, ":")
	if colonIdx < 0 {
		return ""
	}
	s = strings.TrimSpace(s[colonIdx+1:])
	// Remove quotes if present
	s = strings.Trim(s, `"`)
	// Take up to the first non-numeric char
	end := 0
	for end < len(s) && (s[end] == '-' || s[end] == '.' || (s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	return s[:end]
}
