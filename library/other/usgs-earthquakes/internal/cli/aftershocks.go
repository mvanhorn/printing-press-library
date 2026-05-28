// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAftershocksCmd(flags *rootFlags) *cobra.Command {
	var (
		radiusKm float64
		days     int
		minMag   float64
		dataSrc  string
	)
	cmd := &cobra.Command{
		Use:   "aftershocks <event-id>",
		Short: "Show events within R km and T days after a mainshock, ordered by time",
		Long: `Show events occurring within R km and T days after a given mainshock event.

The mainshock event ID is looked up first (local store if available, else live FDSN).
The aftershock query then runs as a bounded local SQLite haversine query when the
events are cached, with a live FDSN fallback otherwise.`,
		Example: strings.Trim(`
  # Aftershocks within 100 km, 30 days after a mainshock
  usgs-earthquakes-pp-cli aftershocks us7000abcd --radius-km 100 --days 30 --json

  # Larger sequence, force live API
  usgs-earthquakes-pp-cli aftershocks us6000sy84 --radius-km 200 --days 60 --min-mag 3.0 --data-source live --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			eventID := args[0]
			ctx := cmd.Context()

			// 1) Resolve the mainshock origin.
			var (
				lat, lon          float64
				timeMs            int64
				mag               float64
				place             string
				resolvedFromLocal bool
			)
			if dataSrc != "live" {
				db, err := openLocalStore(ctx)
				if err == nil {
					defer db.Close()
					feat, err := localEventByID(ctx, db, eventID)
					if err == nil && feat != nil {
						la, lo, _, tm, ok := localEventCoords(feat)
						if ok {
							lat, lon, timeMs = la, lo, tm
							if props, _ := feat["properties"].(map[string]any); props != nil {
								mag, _ = props["mag"].(float64)
								place, _ = props["place"].(string)
							}
							resolvedFromLocal = true
						}
					}
				}
			}
			if !resolvedFromLocal {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get("/query", map[string]string{
					"eventid": eventID,
					"format":  "geojson",
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var feat map[string]any
				if err := json.Unmarshal(data, &feat); err != nil {
					return fmt.Errorf("parse mainshock: %w", err)
				}
				// /query?eventid=... returns a Feature, not a FeatureCollection.
				la, lo, _, tm, ok := localEventCoords(feat)
				if !ok {
					return notFoundErr(fmt.Errorf("event %q not found or missing geometry", eventID))
				}
				lat, lon, timeMs = la, lo, tm
				if props, _ := feat["properties"].(map[string]any); props != nil {
					mag, _ = props["mag"].(float64)
					place, _ = props["place"].(string)
				}
			}

			mainshockTime := time.Unix(timeMs/1000, 0).UTC()
			endTime := mainshockTime.Add(time.Duration(days) * 24 * time.Hour)

			// 2) Query aftershocks.
			results, err := queryAftershocks(ctx, flags, eventID, lat, lon, radiusKm, mainshockTime, endTime, minMag, dataSrc)
			if err != nil {
				return err
			}

			out := map[string]any{
				"mainshock": map[string]any{
					"id":    eventID,
					"mag":   mag,
					"place": place,
					"time":  mainshockTime.Format(time.RFC3339),
				},
				"params": map[string]any{
					"radius_km": radiusKm,
					"days":      days,
					"min_mag":   minMag,
				},
				"aftershock_count": len(results),
				"aftershocks":      results,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "Mainshock\t%s\tM%.1f\t%s\t%s\n", eventID, mag, place, mainshockTime.Format(time.RFC3339))
			fmt.Fprintf(w, "Aftershocks\t%d events within %.0f km, %d days, M%.1f+\n\n", len(results), radiusKm, days, minMag)
			fmt.Fprintln(w, "TIME\tID\tMAG\tDIST_KM\tPLACE")
			for _, a := range results {
				ts := time.Unix(a.TimeMs/1000, 0).UTC().Format(time.RFC3339)
				fmt.Fprintf(w, "%s\t%s\tM%.1f\t%.1f\t%s\n", ts, a.ID, a.Mag, a.DistanceKm, a.Place)
			}
			return w.Flush()
		},
	}
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 100, "Aftershock search radius in km")
	cmd.Flags().IntVar(&days, "days", 30, "Days after mainshock to search")
	cmd.Flags().Float64Var(&minMag, "min-mag", 2.0, "Minimum aftershock magnitude")
	cmd.Flags().StringVar(&dataSrc, "data-source", "auto", "Data source: auto (local with live fallback), live, local")
	return cmd
}

type aftershockRow struct {
	ID         string  `json:"id"`
	Mag        float64 `json:"mag"`
	Place      string  `json:"place"`
	TimeMs     int64   `json:"time_ms"`
	DistanceKm float64 `json:"distance_km"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
}

func queryAftershocks(ctx context.Context, flags *rootFlags, mainshockID string, lat, lon, radiusKm float64, startTime, endTime time.Time, minMag float64, dataSrc string) ([]aftershockRow, error) {
	// Try local store first when allowed.
	if dataSrc != "live" {
		db, err := openLocalStore(ctx)
		if err == nil {
			defer db.Close()
			// Pre-filter by time bounds in SQL; haversine in Go.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT id, data FROM resources
				WHERE resource_type='events'
				  AND CAST(json_extract(data, '$.properties.time') AS INTEGER) BETWEEN ? AND ?
				  AND id != ?
			`, startTime.UnixMilli(), endTime.UnixMilli(), mainshockID)
			if err == nil {
				defer rows.Close()
				var results []aftershockRow
				for rows.Next() {
					var id sql.NullString
					var raw sql.NullString
					if rows.Scan(&id, &raw) != nil || !id.Valid || !raw.Valid {
						continue
					}
					var feat map[string]any
					if json.Unmarshal([]byte(raw.String), &feat) != nil {
						continue
					}
					eLat, eLon, _, tMs, ok := localEventCoords(feat)
					if !ok {
						continue
					}
					dist := haversineKm(lat, lon, eLat, eLon)
					if dist > radiusKm {
						continue
					}
					props, _ := feat["properties"].(map[string]any)
					mag, _ := props["mag"].(float64)
					place, _ := props["place"].(string)
					if mag < minMag {
						continue
					}
					results = append(results, aftershockRow{
						ID:         id.String,
						Mag:        mag,
						Place:      place,
						TimeMs:     tMs,
						DistanceKm: round2(dist),
						Lat:        eLat,
						Lon:        eLon,
					})
				}
				if len(results) > 0 || dataSrc == "local" {
					sort.Slice(results, func(i, j int) bool { return results[i].TimeMs < results[j].TimeMs })
					return results, nil
				}
			}
		}
		if dataSrc == "local" {
			return nil, nil
		}
	}

	// Live FDSN fallback.
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"format":       "geojson",
		"latitude":     strconv.FormatFloat(lat, 'f', -1, 64),
		"longitude":    strconv.FormatFloat(lon, 'f', -1, 64),
		"maxradiuskm":  strconv.FormatFloat(radiusKm, 'f', -1, 64),
		"starttime":    fdsnTimeFormat(startTime),
		"endtime":      fdsnTimeFormat(endTime),
		"minmagnitude": strconv.FormatFloat(minMag, 'f', -1, 64),
		"orderby":      "time-asc",
		"limit":        "1000",
	}
	data, err := c.Get("/query", params)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var fc struct {
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse aftershocks: %w", err)
	}
	var results []aftershockRow
	for _, raw := range fc.Features {
		var feat map[string]any
		if json.Unmarshal(raw, &feat) != nil {
			continue
		}
		id, _ := feat["id"].(string)
		if id == mainshockID {
			continue
		}
		eLat, eLon, _, tMs, ok := localEventCoords(feat)
		if !ok {
			continue
		}
		props, _ := feat["properties"].(map[string]any)
		mag, _ := props["mag"].(float64)
		place, _ := props["place"].(string)
		results = append(results, aftershockRow{
			ID:         id,
			Mag:        mag,
			Place:      place,
			TimeMs:     tMs,
			DistanceKm: round2(haversineKm(lat, lon, eLat, eLon)),
			Lat:        eLat,
			Lon:        eLon,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].TimeMs < results[j].TimeMs })
	return results, nil
}
