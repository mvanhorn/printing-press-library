// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports nearby — distance-ranked alternates near one airport, flagging
// which have normal operations. The map renders coordinates but never ranks
// "nearest healthy airport".
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

// flightyNearbyRow is one distance-ranked alternate.
type flightyNearbyRow struct {
	IATA        string  `json:"iata"`
	Name        string  `json:"name"`
	City        string  `json:"city"`
	Status      string  `json:"status,omitempty"`
	Region      string  `json:"region,omitempty"`
	DistanceKm  float64 `json:"distanceKm"`
	Warnings    []string `json:"warnings,omitempty"`
}

type flightyLatLon struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func newNovelAirportsNearbyCmd(flags *rootFlags) *cobra.Command {
	var flagHealthyOnly bool
	var flagLimit int
	var flagMaxKm float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "nearby <airport>",
		Short: "Distance-ranked nearby airports, flagging which have normal operations right now.",
		Long:  "Use this command for distance-ranked alternates near one airport. Do NOT use it for region browsing; use 'airports list' instead. Reads the local mirror and falls back to one live catalog fetch when the mirror is empty; run 'flighty-pp-cli sync --resources airports --full' for offline use.",
		Example: "  flighty-pp-cli airports nearby sfo --healthy-only --limit 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "airport=den;--max-km=3000"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports nearby")
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("airport is required\nUsage: %s <airport>", cmd.CommandPath()))
			}
			airports, err := flightyCatalogAutoWithHints(cmd, flags, dbPath)
			if err != nil {
				return err
			}

			origin, err := flightyResolveAirport(airports, args[0])
			if err != nil {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{"error": err.Error()}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(err)
			}
			var originLoc flightyLatLon
			if err := json.Unmarshal(origin.Location, &originLoc); err != nil {
				return fmt.Errorf("airport %s has no usable coordinates in the local catalog: %w", origin.IATA, err)
			}

			rows := flightyRankNearby(airports, origin.IATA, originLoc, flagHealthyOnly, flagMaxKm, flagLimit)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				view := map[string]any{
					"origin": map[string]any{
						"iata":   origin.IATA,
						"name":   origin.Name,
						"city":   origin.City,
						"status": origin.Status,
					},
					"results": rows,
				}
				if len(rows) == 0 {
					// Zero-match note: name the flag that widens the search so
					// agents can distinguish "empty because too tight" from
					// "empty because broken".
					if flagHealthyOnly {
						view["note"] = "no airports with normal operations within range; raise --max-km, drop --healthy-only, or check 'airports show' for the origin's status"
					} else {
						view["note"] = "no tracked airports within range; the catalog covers 156 major airports, so raise --max-km (nearest tracked airport may be far away)"
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No nearby airports matched. Widen --max-km, drop --healthy-only, or sync first.")
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-4s %-28s %-14s %8.1f km  %s\n",
					r.IATA, r.Name, r.Status, r.DistanceKm, r.City)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagHealthyOnly, "healthy-only", false, "Only return airports with NORMAL_OPERATIONS status")
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Maximum alternates to return")
	cmd.Flags().Float64Var(&flagMaxKm, "max-km", 1000, "Maximum distance in kilometers (the catalog tracks 156 major airports, so neighbors can be far apart)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// flightyRankNearby haversine-ranks the catalog around the origin airport.
func flightyRankNearby(airports []flightyCatalogAirport, originIATA string, origin flightyLatLon, healthyOnly bool, maxKm float64, limit int) []flightyNearbyRow {
	rows := []flightyNearbyRow{}
	for _, ap := range airports {
		if ap.IATA == originIATA {
			continue
		}
		if healthyOnly && ap.Status != "" && ap.Status != "NORMAL_OPERATIONS" {
			continue
		}
		var loc flightyLatLon
		if json.Unmarshal(ap.Location, &loc) != nil {
			continue
		}
		if loc.Latitude == 0 && loc.Longitude == 0 {
			continue
		}
		dist := flightyHaversineKm(origin.Latitude, origin.Longitude, loc.Latitude, loc.Longitude)
		if maxKm > 0 && dist > maxKm {
			continue
		}
		rows = append(rows, flightyNearbyRow{
			IATA:       ap.IATA,
			Name:       ap.Name,
			City:       ap.City,
			Status:     ap.Status,
			Region:     ap.Region,
			DistanceKm: math.Round(dist*10) / 10,
			Warnings:   ap.Warnings,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].DistanceKm != rows[j].DistanceKm {
			return rows[i].DistanceKm < rows[j].DistanceKm
		}
		return rows[i].IATA < rows[j].IATA
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}
