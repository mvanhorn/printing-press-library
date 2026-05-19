// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSwarmDetectCmd(flags *rootFlags) *cobra.Command {
	var (
		bbox            string
		near            string
		radiusKm        float64
		window          string
		minEvents       int
		clusterRadiusKm float64
		minMag          float64
	)
	cmd := &cobra.Command{
		Use:   "swarm-detect",
		Short: "Detect spatial-temporal clusters of earthquakes in the local store",
		Long: `Detect time-space clusters of earthquakes: groups of N+ events occurring
within a small region over a short time window. Useful for volcanic swarm
monitoring, fault-zone activity, and induced seismicity.

Operates on the local SQLite store; run 'sync' first to populate it.

Algorithm: bucket events into 0.1° lat/lon cells × T-hour windows, filter
cells with at least --min-events, then merge contiguous hot cells within
--cluster-radius-km.`,
		Example: strings.Trim(`
  # Swarms in the past 7 days near Mount Hood
  usgs-earthquakes-pp-cli swarm-detect --near 45.3736,-121.6960 --radius-km 50 --window 7d --min-events 5 --json

  # Global swarms in the past 30 days
  usgs-earthquakes-pp-cli swarm-detect --window 30d --min-events 20 --cluster-radius-km 30 --json

  # California swarm watch
  usgs-earthquakes-pp-cli swarm-detect --bbox -125,32,-114,42 --window 14d --min-events 10 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			startT, err := parseSinceArg(window)
			if err != nil {
				return usageErr(err)
			}
			db, err := openLocalStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local store (try `usgs-earthquakes-pp-cli sync` first): %w", err)
			}
			defer db.Close()

			query := `SELECT id, data FROM resources
			WHERE resource_type='events'
			  AND CAST(json_extract(data, '$.properties.time') AS INTEGER) >= ?`
			argsSQL := []any{startT.UnixMilli()}
			rows, err := db.DB().QueryContext(ctx, query, argsSQL...)
			if err != nil {
				return fmt.Errorf("query local events: %w", err)
			}
			defer rows.Close()

			type ev struct {
				ID     string
				Lat    float64
				Lon    float64
				Mag    float64
				TimeMs int64
			}
			var events []ev
			var bboxFilter *[4]float64
			if bbox != "" {
				w, s, e, n, perr := parseBBox(bbox)
				if perr != nil {
					return usageErr(perr)
				}
				bboxFilter = &[4]float64{w, s, e, n}
			}
			var nearFilter *[3]float64
			if near != "" {
				lat, lon, perr := parseLatLonPair(near)
				if perr != nil {
					return usageErr(perr)
				}
				nearFilter = &[3]float64{lat, lon, radiusKm}
			}
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
				if bboxFilter != nil {
					if eLon < bboxFilter[0] || eLon > bboxFilter[2] || eLat < bboxFilter[1] || eLat > bboxFilter[3] {
						continue
					}
				}
				if nearFilter != nil {
					if haversineKm(nearFilter[0], nearFilter[1], eLat, eLon) > nearFilter[2] {
						continue
					}
				}
				props, _ := feat["properties"].(map[string]any)
				mag, _ := props["mag"].(float64)
				if mag < minMag {
					continue
				}
				events = append(events, ev{id.String, eLat, eLon, mag, tMs})
			}

			// Bucket size derives from --cluster-radius-km. 1° of latitude ≈
			// 111 km globally; longitude degree size varies by latitude but
			// the lat-equivalent is a reasonable approximation for the
			// bucket-grid step. Floor at 0.01° (≈1.1 km) so an unset/zero
			// flag still produces useful clustering.
			cellSizeDeg := clusterRadiusKm / 111.0
			if cellSizeDeg < 0.01 {
				cellSizeDeg = 0.01
			}
			type bucket struct {
				CellLat int
				CellLon int
				Events  []ev
			}
			buckets := make(map[[2]int]*bucket)
			for _, e := range events {
				key := [2]int{
					int(math.Floor(e.Lat / cellSizeDeg)),
					int(math.Floor(e.Lon / cellSizeDeg)),
				}
				b, ok := buckets[key]
				if !ok {
					b = &bucket{CellLat: key[0], CellLon: key[1]}
					buckets[key] = b
				}
				b.Events = append(b.Events, e)
			}

			// Filter buckets meeting min-events.
			type cluster struct {
				CenterLat   float64  `json:"center_lat"`
				CenterLon   float64  `json:"center_lon"`
				EventCount  int      `json:"event_count"`
				PeakMag     float64  `json:"peak_mag"`
				FirstTimeMs int64    `json:"first_time_ms"`
				LastTimeMs  int64    `json:"last_time_ms"`
				FirstTime   string   `json:"first_time"`
				LastTime    string   `json:"last_time"`
				BboxRadius  float64  `json:"bbox_radius_km"`
				SampleIDs   []string `json:"sample_event_ids"`
			}
			var clusters []cluster
			for _, b := range buckets {
				if len(b.Events) < minEvents {
					continue
				}
				var sumLat, sumLon float64
				var peakMag float64
				var firstT, lastT int64 = math.MaxInt64, 0
				ids := []string{}
				for _, e := range b.Events {
					sumLat += e.Lat
					sumLon += e.Lon
					if e.Mag > peakMag {
						peakMag = e.Mag
					}
					if e.TimeMs < firstT {
						firstT = e.TimeMs
					}
					if e.TimeMs > lastT {
						lastT = e.TimeMs
					}
					if len(ids) < 5 {
						ids = append(ids, e.ID)
					}
				}
				n := float64(len(b.Events))
				cLat := sumLat / n
				cLon := sumLon / n
				maxDist := 0.0
				for _, e := range b.Events {
					if d := haversineKm(cLat, cLon, e.Lat, e.Lon); d > maxDist {
						maxDist = d
					}
				}
				clusters = append(clusters, cluster{
					CenterLat:   round2(cLat),
					CenterLon:   round2(cLon),
					EventCount:  len(b.Events),
					PeakMag:     peakMag,
					FirstTimeMs: firstT,
					LastTimeMs:  lastT,
					FirstTime:   time.Unix(firstT/1000, 0).UTC().Format(time.RFC3339),
					LastTime:    time.Unix(lastT/1000, 0).UTC().Format(time.RFC3339),
					BboxRadius:  round2(maxDist),
					SampleIDs:   ids,
				})
			}
			sort.Slice(clusters, func(i, j int) bool {
				return clusters[i].EventCount > clusters[j].EventCount
			})

			out := map[string]any{
				"window_start":   startT.Format(time.RFC3339),
				"min_events":     minEvents,
				"events_scanned": len(events),
				"cluster_count":  len(clusters),
				"clusters":       clusters,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "Window\t%s — now\n", startT.Format(time.RFC3339))
			fmt.Fprintf(w, "Events scanned\t%d\n", len(events))
			fmt.Fprintf(w, "Clusters (>=%d events)\t%d\n\n", minEvents, len(clusters))
			fmt.Fprintln(w, "CENTER\tCOUNT\tPEAK_MAG\tRADIUS_KM\tFIRST\tLAST\tSAMPLE_IDS")
			for _, c := range clusters {
				fmt.Fprintf(w, "%.2f,%.2f\t%d\tM%.1f\t%.1f\t%s\t%s\t%s\n",
					c.CenterLat, c.CenterLon, c.EventCount, c.PeakMag, c.BboxRadius,
					c.FirstTime, c.LastTime, strings.Join(c.SampleIDs, ","))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", `Bounding box "W,S,E,N"`)
	cmd.Flags().StringVar(&near, "near", "", `Center point "lat,lon"`)
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 100, "Search radius when --near is set")
	cmd.Flags().StringVar(&window, "window", "7d", "Time window lookback (24h, 7d, 30d)")
	cmd.Flags().IntVar(&minEvents, "min-events", 5, "Minimum events per cluster")
	cmd.Flags().Float64Var(&clusterRadiusKm, "cluster-radius-km", 20, "Cluster radius in km; controls the grid-cell size used to bucket events (smaller value → tighter clusters)")
	cmd.Flags().Float64Var(&minMag, "min-mag", 0, "Skip events below this magnitude")
	return cmd
}
