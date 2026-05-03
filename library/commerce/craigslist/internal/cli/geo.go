// `geo within` and `geo bbox` filter the local store geographically. Local
// store only — sapi has lat/lng but no native radius search.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

// geoHit is the typed shape per matching listing.
type geoHit struct {
	PID        int64   `json:"pid"`
	Title      string  `json:"title"`
	Site       string  `json:"site"`
	Price      int     `json:"price"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
	DistanceMi float64 `json:"distanceMi,omitempty"`
	URL        string  `json:"url,omitempty"`
}

func newGeoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "geo",
		Short:       "Filter local listings by lat/lng radius or bounding box",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newGeoWithinCmd(flags))
	cmd.AddCommand(newGeoBBoxCmd(flags))
	return cmd
}

func newGeoWithinCmd(flags *rootFlags) *cobra.Command {
	var lat, lng, radiusMi float64
	var site, category string
	cmd := &cobra.Command{
		Use:         "within",
		Short:       "Listings within a radius of a lat/lng",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lat == 0 && lng == 0 && radiusMi == 0 {
				return cmd.Help()
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := geoWithin(ctx, db.DB(), lat, lng, radiusMi, site, category)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Center latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "Center longitude")
	cmd.Flags().Float64Var(&radiusMi, "radius-mi", 0, "Radius in miles")
	cmd.Flags().StringVar(&site, "site", "", "Optional site filter")
	cmd.Flags().StringVar(&category, "category", "", "Optional category filter")
	return cmd
}

func newGeoBBoxCmd(flags *rootFlags) *cobra.Command {
	var minLat, maxLat, minLng, maxLng float64
	var site, category string
	cmd := &cobra.Command{
		Use:         "bbox",
		Short:       "Listings within a lat/lng bounding box",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if minLat == 0 && maxLat == 0 && minLng == 0 && maxLng == 0 {
				return cmd.Help()
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := geoBBox(ctx, db.DB(), minLat, maxLat, minLng, maxLng, site, category)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().Float64Var(&minLat, "min-lat", 0, "Bounding box south latitude")
	cmd.Flags().Float64Var(&maxLat, "max-lat", 0, "Bounding box north latitude")
	cmd.Flags().Float64Var(&minLng, "min-lng", 0, "Bounding box west longitude")
	cmd.Flags().Float64Var(&maxLng, "max-lng", 0, "Bounding box east longitude")
	cmd.Flags().StringVar(&site, "site", "", "Optional site filter")
	cmd.Flags().StringVar(&category, "category", "", "Optional category filter")
	return cmd
}

func geoWithin(ctx context.Context, db *sql.DB, lat, lng, radiusMi float64, site, category string) ([]geoHit, error) {
	rows, err := queryGeoBase(ctx, db, site, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []geoHit
	for rows.Next() {
		var h geoHit
		if err := rows.Scan(&h.PID, &h.Title, &h.Site, &h.Price, &h.Latitude, &h.Longitude, &h.URL); err != nil {
			return nil, err
		}
		if h.Latitude == 0 && h.Longitude == 0 {
			continue
		}
		dist := haversineMi(lat, lng, h.Latitude, h.Longitude)
		if dist > radiusMi {
			continue
		}
		h.DistanceMi = dist
		out = append(out, h)
	}
	return out, rows.Err()
}

func geoBBox(ctx context.Context, db *sql.DB, minLat, maxLat, minLng, maxLng float64, site, category string) ([]geoHit, error) {
	rows, err := queryGeoBase(ctx, db, site, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []geoHit
	for rows.Next() {
		var h geoHit
		if err := rows.Scan(&h.PID, &h.Title, &h.Site, &h.Price, &h.Latitude, &h.Longitude, &h.URL); err != nil {
			return nil, err
		}
		if h.Latitude < minLat || h.Latitude > maxLat || h.Longitude < minLng || h.Longitude > maxLng {
			continue
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func queryGeoBase(ctx context.Context, db *sql.DB, site, category string) (*sql.Rows, error) {
	q := `SELECT pid, COALESCE(title,''), COALESCE(site,''), COALESCE(price,0), COALESCE(lat,0), COALESCE(lng,0), COALESCE(canonical_url,'') FROM listings`
	args := []any{}
	conds := []string{"lat != 0 AND lng != 0"}
	if site != "" {
		conds = append(conds, "site = ?")
		args = append(args, site)
	}
	if category != "" {
		conds = append(conds, "category_abbr = ?")
		args = append(args, category)
	}
	q += " WHERE " + strings.Join(conds, " AND ")
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("geo query: %w", err)
	}
	return rows, nil
}

// haversineMi returns the great-circle distance between two lat/lng pairs in
// miles. Earth radius 3958.8 mi.
func haversineMi(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMi = 3958.8
	rad := math.Pi / 180
	dlat := (lat2 - lat1) * rad
	dlng := (lng2 - lng1) * rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMi * c
}
