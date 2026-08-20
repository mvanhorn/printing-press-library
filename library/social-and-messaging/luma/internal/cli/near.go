// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (NOT generated).
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/internal/store"
)

type nearView struct {
	Events   []lumaEventView `json:"events"`
	Count    int             `json:"count"`
	Center   [2]float64      `json:"center"`
	RadiusKm float64         `json:"radius_km"`
	Scanned  int             `json:"scanned_events"`
	Note     string          `json:"note,omitempty"`
}

func newNovelNearCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLng float64
	var flagRadiusKm float64
	var flagWindow string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "near",
		Short: "Find events within N km of a lat/lng, ranked by distance.",
		Long: "Haversine radius search over your locally synced events. The public API exposes each\n" +
			"event's coordinate but has no radius filter, so this reads the local store.\n\n" +
			"Run `sync --resources events --resource-param events:slug=<city>` first to populate it.",
		Example:     "  luma-pp-cli near --lat 37.77 --lng -122.42 --radius-km 5 --window 14d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan local events for those within the radius")
				return nil
			}
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lng") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--lat and --lng are required"))
			}
			window, err := parseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", flagWindow, err))
			}
			if flagDB == "" {
				flagDB = defaultDBPath("luma-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: luma-pp-cli sync --resources events --resource-param events:slug=sf --db %s\n", flagDB, flagDB)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, flagDB)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "events")

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'events'`)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}
			defer rows.Close()

			now := time.Now()
			scanned := 0
			matches := make([]lumaEntry, 0)
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					continue
				}
				scanned++
				var e lumaEntry
				if json.Unmarshal([]byte(data), &e) != nil || e.Event.Coordinate == nil {
					continue
				}
				if t, ok := e.startTime(); ok && !withinWindow(t, now, window) {
					continue
				}
				dist := haversineKm(flagLat, flagLng, e.Event.Coordinate.Latitude, e.Event.Coordinate.Longitude)
				if dist > flagRadiusKm {
					continue
				}
				matches = append(matches, e)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating events: %w", err)
			}

			sort.SliceStable(matches, func(i, j int) bool {
				di := haversineKm(flagLat, flagLng, matches[i].Event.Coordinate.Latitude, matches[i].Event.Coordinate.Longitude)
				dj := haversineKm(flagLat, flagLng, matches[j].Event.Coordinate.Latitude, matches[j].Event.Coordinate.Longitude)
				return di < dj
			})
			if flagLimit > 0 && len(matches) > flagLimit {
				matches = matches[:flagLimit]
			}

			views := make([]lumaEventView, 0, len(matches))
			for _, e := range matches {
				v := e.view()
				v.DistanceKm = round1(haversineKm(flagLat, flagLng, e.Event.Coordinate.Latitude, e.Event.Coordinate.Longitude))
				views = append(views, v)
			}

			view := nearView{
				Events:   views,
				Count:    len(views),
				Center:   [2]float64{flagLat, flagLng},
				RadiusKm: flagRadiusKm,
				Scanned:  scanned,
			}
			if len(views) == 0 {
				if scanned == 0 {
					view.Note = "no events in the local store; run sync first"
				} else {
					view.Note = fmt.Sprintf("scanned %d events; none within %.1f km — widen --radius-km or sync more cities", scanned, flagRadiusKm)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().Float64Var(&flagLat, "lat", 0, "Center latitude (required)")
	cmd.Flags().Float64Var(&flagLng, "lng", 0, "Center longitude (required)")
	cmd.Flags().Float64Var(&flagRadiusKm, "radius-km", 10, "Search radius in kilometers")
	cmd.Flags().StringVar(&flagWindow, "window", "", "Only events within this window from now (e.g. 14d); empty = all upcoming")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Max events to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/luma-pp-cli/data.db)")
	return cmd
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
