// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
	"math"
	"sort"
	"strconv"
	"strings"
)

func newNovelShortlistListCmd(flags *rootFlags) *cobra.Command {
	var audit bool
	var origin, categories string
	var radius float64
	var limit int
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "list", Short: "Read saved facility evidence, recheck reasons or straight-line proximity.",
		Long:        "Read the saved shortlist offline. --audit prioritizes missing, conflicting, old or undated source evidence. --origin ranks saved facilities by straight-line distance and makes no accessible-route or connection claim. For refreshing observations use shortlist changes; for unsaved discovery use spots search; for chosen IDs use spots compare.",
		Example:     "  wheelog-pp-cli shortlist list --audit --require-question 102 --data-source local --agent\n  wheelog-pp-cli shortlist list --origin 35.7742,140.3879 --radius-m 500 --category toilet --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--data-source=local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "read saved WheeLog evidence")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("shortlist list accepts flags, not positional arguments"))
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			if mode == "live" {
				return usageErr(fmt.Errorf("shortlist list has no live equivalent; use shortlist changes to refresh"))
			}
			if limit < 1 || limit > 50 {
				return usageErr(fmt.Errorf("--limit must be 1..50"))
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			cats, err := wheelogCategories(categories)
			if err != nil {
				return err
			}
			var position *wheelog.Coordinate
			if origin != "" {
				parts := strings.Split(origin, ",")
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("--origin must be latitude,longitude"))
				}
				lat, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				lon, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if e1 != nil || e2 != nil || !wheelog.ValidCoordinate(lat, lon) {
					return usageErr(fmt.Errorf("--origin requires finite latitude -90..90 and longitude -180..180"))
				}
				position = &wheelog.Coordinate{Latitude: lat, Longitude: lon}
			}
			if cmd.Flags().Changed("radius-m") && position == nil {
				return usageErr(fmt.Errorf("--radius-m requires --origin"))
			}
			if math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 0 || radius > 1000000 {
				return usageErr(fmt.Errorf("--radius-m must be finite and between 0 and 1000000 meters"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			saved, err := savedWheelog(ctx, options.DB)
			if err != nil {
				return err
			}
			wheelogLocalHint(cmd, saved, maxAge)
			rows := make([]wheelogSpotView, 0, len(saved))
			unknownCoordinates, excludedRadius := 0, 0
			for _, entry := range saved {
				spot := entry.Latest
				spot.Source = "local"
				if len(cats) > 0 && !containsWheelogCategory(cats, spot.Category) {
					continue
				}
				row := viewWheelog(spot, options.Questions, maxAge)
				if position != nil {
					if spot.Location == nil {
						unknownCoordinates++
						if cmd.Flags().Changed("radius-m") {
							continue
						}
					} else {
						distance := wheelog.Distance(*position, *spot.Location)
						row.Distance = &distance
						if cmd.Flags().Changed("radius-m") && distance > radius {
							excludedRadius++
							continue
						}
					}
				}
				if audit && len(row.Recheck) == 0 {
					continue
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if audit && len(rows[i].Recheck) != len(rows[j].Recheck) {
					return len(rows[i].Recheck) > len(rows[j].Recheck)
				}
				if position != nil {
					if rows[i].Distance == nil {
						return false
					}
					if rows[j].Distance == nil {
						return true
					}
					if *rows[i].Distance != *rows[j].Distance {
						return *rows[i].Distance < *rows[j].Distance
					}
				}
				return rows[i].ID < rows[j].ID
			})
			matched := len(rows)
			if len(rows) > limit {
				rows = rows[:limit]
			}
			return emitWheelog(cmd, flags, map[string]any{"results": rows, "coverage": map[string]any{"scope": "saved_shortlist", "scanned_saved_records": len(saved), "matched": matched, "returned": len(rows), "unknown_coordinates": unknownCoordinates, "radius_excluded": excludedRadius, "output_limit": limit}, "note": wheelogEvidenceNote + " Distances are straight-line meters within saved coverage. The supplied origin is not saved."}, "local")
		}}
	cmd.Flags().BoolVar(&audit, "audit", false, "Show only saved entries with explicit reasons to recheck recorded evidence.")
	cmd.Flags().StringVar(&origin, "origin", "", "Invocation-only latitude,longitude for ranking saved facilities by straight-line distance.")
	cmd.Flags().Float64Var(&radius, "radius-m", 0, "Optional maximum straight-line distance in meters; requires --origin.")
	cmd.Flags().StringVar(&categories, "category", "", "Filter exact saved source categories, comma-separated.")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum saved entries returned, between 1 and 50.")
	addWheelogReadFlags(cmd, &options)
	return cmd
}
