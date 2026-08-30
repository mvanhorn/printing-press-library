// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelTripsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB    string
		flagSince string
	)
	cmd := &cobra.Command{
		Use:         "trips <vin>",
		Short:       "Reconstruct trips (start, end, elapsed, distance) from your vehicle's location breadcrumbs.",
		Long:        "Segments the navigation.currentLocation time-series into trips locally. The API streams points, never trips.",
		Example:     "  bmw-cardata-pp-cli trips WBAJB3105JUV12345 --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would reconstruct trips from location breadcrumbs")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a VIN is required"))
			}
			if err := validateVIN(args[0]); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			vin := args[0]
			since := sinceFromWindow(flagSince)
			dbPath := resolveDBPath(flagDB)
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bmw-cardata-pp-cli customers get-telematic-data %s --container-id <id>\n", dbPath, vin)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := openCardataStore(dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			// Pull location breadcrumbs ordered by id (≈ time).
			pts, err := locationBreadcrumbs(db, vin, since)
			if err != nil {
				return configErr(fmt.Errorf("querying location breadcrumbs: %w", err))
			}

			trips := segmentTrips(pts)
			view := map[string]any{
				"vin":              vin,
				"since":            flagSince,
				"breadcrumb_count": len(pts),
				"trip_count":       len(trips),
				"trips":            trips,
			}
			if len(pts) == 0 {
				view["note"] = "no location breadcrumbs in the local store; fetch telematic data including navigation.currentLocation"
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Trips for %s (last %s): %d\n", vin, flagSince, len(trips))
			for i, t := range trips {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s -> %s  %.1f km  %s\n",
					i+1, t["start_at"], t["end_at"], t["distance_km"], t["duration"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Time window (e.g. 7d, 30d)")
	return cmd
}

type breadcrumb struct {
	ts  string
	lat float64
	lng float64
}

// locationBreadcrumbs pairs latitude/longitude snapshots by their VSS
// timestamp into ordered breadcrumbs for trip segmentation.
func locationBreadcrumbs(db *store.Store, vin string, since time.Time) ([]breadcrumb, error) {
	rows, err := db.DB().Query(
		`SELECT descriptor, value, COALESCE(ts,''), fetched_at FROM cardata_telematic_snapshots
		 WHERE vin = ? AND descriptor IN (?, ?) AND fetched_at >= ?
		 ORDER BY id ASC`,
		vin, cardataLatDescriptor, cardataLongDescriptor, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type half struct {
		lat, lng         float64
		haveLat, haveLng bool
	}
	byKey := map[string]*half{}
	order := []string{}
	for rows.Next() {
		var desc, val, ts, fetched string
		if err := rows.Scan(&desc, &val, &ts, &fetched); err != nil {
			return nil, err
		}
		key := ts
		if key == "" {
			key = fetched
		}
		h, ok := byKey[key]
		if !ok {
			h = &half{}
			byKey[key] = h
			order = append(order, key)
		}
		f, _ := strconv.ParseFloat(val, 64)
		if desc == cardataLatDescriptor {
			h.lat, h.haveLat = f, true
		} else {
			h.lng, h.haveLng = f, true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]breadcrumb, 0, len(order))
	for _, k := range order {
		h := byKey[k]
		if h.haveLat && h.haveLng {
			out = append(out, breadcrumb{ts: k, lat: h.lat, lng: h.lng})
		}
	}
	return out, nil
}

// segmentTrips groups breadcrumbs into trips. A new trip starts when two
// consecutive points are more than 15 minutes apart (the car was parked).
func segmentTrips(pts []breadcrumb) []map[string]any {
	trips := make([]map[string]any, 0)
	const gap = 15 * time.Minute
	var cur []breadcrumb
	flush := func() {
		if len(cur) < 2 {
			cur = nil
			return
		}
		dist := 0.0
		for i := 1; i < len(cur); i++ {
			dist += haversineKm(cur[i-1].lat, cur[i-1].lng, cur[i].lat, cur[i].lng)
		}
		startT, startErr := time.Parse(time.RFC3339, cur[0].ts)
		endT, endErr := time.Parse(time.RFC3339, cur[len(cur)-1].ts)
		trip := map[string]any{
			"start_at":    cur[0].ts,
			"end_at":      cur[len(cur)-1].ts,
			"distance_km": dist,
			"points":      len(cur),
		}
		// Emit duration only when both timestamps parsed; otherwise a zero
		// time.Time (year 0001) would produce a multi-year bogus duration.
		// Surface the parse error so the reader can correlate it with the
		// raw start_at / end_at strings.
		if startErr != nil || endErr != nil {
			trip["duration"] = nil
			trip["duration_parse_error"] = fmt.Sprintf("start=%v end=%v", startErr, endErr)
		} else {
			trip["duration"] = endT.Sub(startT).Round(time.Second).String()
		}
		trips = append(trips, trip)
		cur = nil
	}
	for i, p := range pts {
		if i > 0 {
			prevT, _ := time.Parse(time.RFC3339, pts[i-1].ts)
			thisT, _ := time.Parse(time.RFC3339, p.ts)
			if !prevT.IsZero() && !thisT.IsZero() && thisT.Sub(prevT) > gap {
				flush()
			}
		}
		cur = append(cur, p)
	}
	flush()
	return trips
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
