// Copyright 2026 jmbernabotto and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/travel/sncf-connect/internal/store"
	"github.com/spf13/cobra"
)

func newIsochroneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "isochrone",
		Short: "Reachability queries — which stations can I reach?",
	}
	cmd.AddCommand(newIsochroneStationsCmd(flags))
	return cmd
}

func newIsochroneStationsCmd(flags *rootFlags) *cobra.Command {
	var from, coverage, dbPath string
	var durationMins int

	cmd := &cobra.Command{
		Use:   "stations",
		Short: "List stations reachable within a travel duration",
		Long: `Given a departure point and a travel duration in minutes, calls /isochrones
to get the reachable geographic boundary, then cross-references the local
stop-areas store to list all train stations within that boundary.

Requires a prior 'sncf-connect-pp-cli sync' to populate the local store.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli isochrone stations --from "stop_area:SNCF:87686006" --duration 60
  sncf-connect-pp-cli isochrone stations --from "2.3522;48.8566" --duration 90 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if from == "" {
				return fmt.Errorf("--from is required (stop area URI or lon;lat)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			durationSec := durationMins * 60

			// Call isochrones — requires lon;lat in path
			// If from is a stop URI, use /coverage/{region}/journeys to get coords
			// For simplicity: if it contains ";", treat as lon;lat; else use coverage path
			var isoPath string
			isoParams := map[string]string{
				"from":         from,
				"max_duration": strconv.Itoa(durationSec),
			}

			if isLonLat(from) {
				lon, lat := splitLonLat(from)
				isoPath = "/coverage/" + lon + ";" + lat + "/isochrones"
				delete(isoParams, "from")
			} else {
				isoPath = fmt.Sprintf("/coverage/%s/isochrones", coverage)
			}

			isoData, _, err := resolveRead(cmd.Context(), c, flags, "isochrones", false, isoPath, isoParams, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse the isochrone boundary
			bbox, err := extractIsochroneBbox(isoData)
			if err != nil {
				return fmt.Errorf("parsing isochrone: %w", err)
			}

			// Query local store for stop_areas within the bounding box
			if dbPath == "" {
				dbPath = defaultDBPath("sncf-connect-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sncf-connect-pp-cli sync' first.", err)
			}
			defer s.Close()

			stations, err := queryStopAreasInBbox(cmd.Context(), s, bbox)
			if err != nil {
				return fmt.Errorf("querying local stops: %w", err)
			}

			truncated := len(stations) == 1000

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"from":          from,
					"duration_mins": durationMins,
					"bbox":          bbox,
					"station_count": len(stations),
					"truncated":     truncated,
					"stations":      stations,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stations reachable from %s within %d minutes:\n\n", from, durationMins)
			if len(stations) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No stations found in local store. Run 'sncf-connect-pp-cli sync' first.")
				return nil
			}
			if truncated {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: result capped at 1000 stations; the actual reachable area may contain more.")
			}
			for _, st := range stations {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %s\n", st["name"], st["id"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d stations total.\n", len(stations))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Departure point: stop area URI or lon;lat")
	cmd.Flags().IntVar(&durationMins, "duration", 60, "Maximum travel duration in minutes")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/sncf-connect-pp-cli/data.db)")
	return cmd
}

func isLonLat(s string) bool {
	for _, c := range s {
		if c == ';' {
			return true
		}
	}
	return false
}

func splitLonLat(s string) (lon, lat string) {
	for i, c := range s {
		if c == ';' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

type geoBbox struct {
	MinLon, MaxLon, MinLat, MaxLat float64
}

func extractIsochroneBbox(data json.RawMessage) (geoBbox, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return geoBbox{}, err
	}

	isochrones, _ := root["isochrones"].([]any)
	if len(isochrones) == 0 {
		// data might be directly a list
		var list []map[string]any
		if err := json.Unmarshal(data, &list); err == nil {
			isochrones = make([]any, len(list))
			for i, v := range list {
				isochrones[i] = v
			}
		}
	}

	bbox := geoBbox{MinLon: 1e9, MaxLon: -1e9, MinLat: 1e9, MaxLat: -1e9}

	var visitCoords func(any)
	visitCoords = func(v any) {
		switch t := v.(type) {
		case []any:
			if len(t) == 2 {
				if lon, ok := toFloat(t[0]); ok {
					if lat, ok := toFloat(t[1]); ok {
						if lon < bbox.MinLon {
							bbox.MinLon = lon
						}
						if lon > bbox.MaxLon {
							bbox.MaxLon = lon
						}
						if lat < bbox.MinLat {
							bbox.MinLat = lat
						}
						if lat > bbox.MaxLat {
							bbox.MaxLat = lat
						}
						return
					}
				}
			}
			for _, item := range t {
				visitCoords(item)
			}
		case map[string]any:
			if geo, ok := t["geojson"]; ok {
				visitCoords(geo)
				return
			}
			for _, val := range t {
				visitCoords(val)
			}
		}
	}
	for _, iso := range isochrones {
		visitCoords(iso)
	}

	// Pad by ~1km to be inclusive
	const pad = 0.01
	bbox.MinLon -= pad
	bbox.MaxLon += pad
	bbox.MinLat -= pad
	bbox.MaxLat += pad

	return bbox, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func queryStopAreasInBbox(ctx context.Context, s *store.Store, bbox geoBbox) ([]map[string]any, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT json_extract(data,'$.id'), json_extract(data,'$.name'),
		        json_extract(data,'$.coord.lon'), json_extract(data,'$.coord.lat')
		 FROM coverage_stop_areas
		 WHERE CAST(json_extract(data,'$.coord.lon') AS REAL) BETWEEN ? AND ?
		   AND CAST(json_extract(data,'$.coord.lat') AS REAL) BETWEEN ? AND ?
		 ORDER BY json_extract(data,'$.name')
		 LIMIT 1000`,
		bbox.MinLon, bbox.MaxLon, bbox.MinLat, bbox.MaxLat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var id, name string
		var lon, lat *float64
		if err := rows.Scan(&id, &name, &lon, &lat); err != nil {
			continue
		}
		entry := map[string]any{"id": id, "name": name}
		if lon != nil {
			entry["lon"] = *lon
		}
		if lat != nil {
			entry["lat"] = *lat
		}
		result = append(result, entry)
	}
	return result, nil
}
