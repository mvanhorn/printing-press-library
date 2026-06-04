// Copyright 2026 jmbernabotto and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/sncf-connect/internal/store"
	"github.com/spf13/cobra"
)

func newStopsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stops",
		Short: "Stop area queries and analysis",
	}
	cmd.AddCommand(newStopsClusterCmd(flags))
	return cmd
}

func newStopsClusterCmd(flags *rootFlags) *cobra.Command {
	var near, coverage, dbPath string
	var radius int

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Group nearby stops within a radius using Haversine distance",
		Long: `Geocodes a place name via /places, then queries the local stop-areas store
and groups stops within 'radius' meters of each other using Haversine distance.

Useful for deduplicating station indexes and resolving ambiguous name matches.

Requires a prior 'sncf-connect-pp-cli sync' to populate the local store.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli stops cluster --near "Gare de Lyon"
  sncf-connect-pp-cli stops cluster --near "Montpellier" --radius 500 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if near == "" {
				return fmt.Errorf("--near is required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Geocode the center point
			placesData, _, err := resolveRead(cmd.Context(), c, flags, "places", false,
				fmt.Sprintf("/coverage/%s/places", coverage),
				map[string]string{"q": near, "count": "1"}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			centerLon, centerLat, err := extractFirstPlaceCoord(placesData)
			if err != nil {
				return fmt.Errorf("geocoding %q: %w", near, err)
			}

			// Query local stop_areas near the center
			if dbPath == "" {
				dbPath = defaultDBPath("sncf-connect-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sncf-connect-pp-cli sync' first.", err)
			}
			defer s.Close()

			// Get all stops within 2× radius for haversine filtering
			degRadius := float64(radius) / 111000.0 * 2
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT json_extract(data,'$.id'), json_extract(data,'$.name'),
				        json_extract(data,'$.coord.lon'), json_extract(data,'$.coord.lat')
				 FROM coverage_stop_areas
				 WHERE CAST(json_extract(data,'$.coord.lon') AS REAL) BETWEEN ? AND ?
				   AND CAST(json_extract(data,'$.coord.lat') AS REAL) BETWEEN ? AND ?`,
				centerLon-degRadius, centerLon+degRadius,
				centerLat-degRadius, centerLat+degRadius)
			if err != nil {
				return fmt.Errorf("querying stops: %w", err)
			}
			defer rows.Close()

			type stopEntry struct {
				ID   string  `json:"id"`
				Name string  `json:"name"`
				Lon  float64 `json:"lon"`
				Lat  float64 `json:"lat"`
				Dist float64 `json:"dist_meters"`
			}

			var nearStops []stopEntry
			for rows.Next() {
				var id, name string
				var lon, lat *float64
				if err := rows.Scan(&id, &name, &lon, &lat); err != nil {
					continue
				}
				if lon == nil || lat == nil {
					continue
				}
				dist := haversineMeters(centerLat, centerLon, *lat, *lon)
				if dist <= float64(radius) {
					nearStops = append(nearStops, stopEntry{ID: id, Name: name, Lon: *lon, Lat: *lat, Dist: dist})
				}
			}

			// Cluster by proximity (greedy: group stops within 100m of each other)
			clusterRadius := 100.0
			type cluster struct {
				Stops []stopEntry `json:"stops"`
			}
			var clusters []cluster
			assigned := make([]bool, len(nearStops))

			for i, stop := range nearStops {
				if assigned[i] {
					continue
				}
				cl := cluster{Stops: []stopEntry{stop}}
				assigned[i] = true
				for j, other := range nearStops {
					if assigned[j] {
						continue
					}
					if haversineMeters(stop.Lat, stop.Lon, other.Lat, other.Lon) <= clusterRadius {
						cl.Stops = append(cl.Stops, other)
						assigned[j] = true
					}
				}
				clusters = append(clusters, cl)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"near":          near,
					"center_lon":    centerLon,
					"center_lat":    centerLat,
					"radius_meters": radius,
					"total_stops":   len(nearStops),
					"clusters":      clusters,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stop clusters near %q (within %dm, center %.4f,%.4f)\n\n",
				near, radius, centerLon, centerLat)

			if len(clusters) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No stops found. Run 'sncf-connect-pp-cli sync' to populate local store.")
				return nil
			}

			for i, cl := range clusters {
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster %d (%d stops):\n", i+1, len(cl.Stops))
				for _, stop := range cl.Stops {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %.0fm  %s\n", stop.Name, stop.Dist, stop.ID)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&near, "near", "", "Place name or address to cluster around")
	cmd.Flags().IntVar(&radius, "radius", 300, "Search radius in meters")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/sncf-connect-pp-cli/data.db)")
	return cmd
}

func extractFirstPlaceCoord(data json.RawMessage) (lon, lat float64, err error) {
	var resp map[string]any
	if e := json.Unmarshal(data, &resp); e != nil {
		return 0, 0, e
	}

	places, _ := resp["places"].([]any)
	if len(places) == 0 {
		var list []map[string]any
		if json.Unmarshal(data, &list) == nil && len(list) > 0 {
			places = make([]any, len(list))
			for i, v := range list {
				places[i] = v
			}
		}
	}

	if len(places) == 0 {
		return 0, 0, fmt.Errorf("no places found")
	}

	p, _ := places[0].(map[string]any)

	// Try embedded_type field to navigate to coord
	embType, _ := p["embedded_type"].(string)
	var embedded map[string]any
	if embType != "" {
		embedded, _ = p[embType].(map[string]any)
	}
	if embedded == nil {
		embedded = p
	}

	coord, _ := embedded["coord"].(map[string]any)
	if coord == nil {
		coord, _ = p["coord"].(map[string]any)
	}
	if coord == nil {
		return 0, 0, fmt.Errorf("no coordinates in place response")
	}

	lonStr, _ := coord["lon"].(string)
	latStr, _ := coord["lat"].(string)

	fmt.Sscanf(lonStr, "%f", &lon)
	fmt.Sscanf(latStr, "%f", &lat)

	if lon == 0 && lat == 0 {
		if f, ok := coord["lon"].(float64); ok {
			lon = f
		}
		if f, ok := coord["lat"].(float64); ok {
			lat = f
		}
	}

	return lon, lat, nil
}

// haversineMeters returns the great-circle distance in meters between two lat/lon points.
func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
