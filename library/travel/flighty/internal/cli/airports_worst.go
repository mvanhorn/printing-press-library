// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports worst — rank the network's airports right now by cumulative delay
// and cancellations. The web map color-codes statuses but never ranks.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// flightyWorstRow is one ranked airport.
type flightyWorstRow struct {
	IATA            string          `json:"iata"`
	Name            string          `json:"name"`
	City            string          `json:"city"`
	Region          string          `json:"region,omitempty"`
	Status          string          `json:"status,omitempty"`
	CumulativeDelay int             `json:"cumulativeDelay"`
	CanceledPct     float64         `json:"canceledPercentageValue"`
	Arrival         json.RawMessage `json:"arrival,omitempty"`
	Departure       json.RawMessage `json:"departure,omitempty"`
	Warnings        []string        `json:"warnings,omitempty"`
}

func newNovelAirportsWorstCmd(flags *rootFlags) *cobra.Command {
	var flagRegion string
	var flagStatus string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "worst",
		Short: "Rank the network's airports right now by cumulative delay and cancellations — the web map color-codes but never ranks.",
		Long:  "Use this command for magnitude-ranked answers (\"which airports are worst right now\"). Do NOT use it for browsing or filtering the airport catalog; use 'airports list' instead. Reads the local mirror and falls back to one live catalog fetch when the mirror is empty; run 'flighty-pp-cli sync --resources airports --full' for offline use.",
		Example: "  flighty-pp-cli airports worst --region Europe --limit 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports worst")
			}
			airports, err := flightyCatalogAutoWithHints(cmd, flags, dbPath)
			if err != nil {
				return err
			}

			rows := flightyRankWorst(airports, flagRegion, flagStatus, flagLimit)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching airports. Sync first and check --region/--status values.")
				return nil
			}
			for i, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%2d. %-4s %-28s %-14s delay:%5dm canceled:%3.0f%% %s\n",
					i+1, r.IATA, r.Name, r.Status, r.CumulativeDelay, r.CanceledPct, r.Warnings)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagRegion, "region", "", "Filter by map region (North America, Europe, Asia, Africa, South America, Middle East, Pacific)")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Filter by status: NORMAL_OPERATIONS, MINOR_ISSUES, MAJOR_ISSUES")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum airports to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// flightyRankWorst sorts the catalog by cumulative delay (desc), then
// canceled percentage (desc), then IATA, applying region/status/limit filters.
func flightyRankWorst(airports []flightyCatalogAirport, region, status string, limit int) []flightyWorstRow {
	regionWant := lower(region)
	statusWant := upper(status)
	candidates := make([]flightyWorstRow, 0, len(airports))
	for _, ap := range airports {
		if regionWant != "" && lower(ap.Region) != regionWant {
			continue
		}
		if statusWant != "" && upper(ap.Status) != statusWant {
			continue
		}
		row := flightyWorstRow{
			IATA:            ap.IATA,
			Name:            ap.Name,
			City:            ap.City,
			Region:          ap.Region,
			Status:          ap.Status,
			CumulativeDelay: ap.CumulativeDelay,
			Arrival:         ap.Arrival,
			Departure:       ap.Departure,
			Warnings:        ap.Warnings,
		}
		// Canceled percentage from either side's today block, whichever is higher.
		cancel := maxCanceledPct(ap.Arrival, ap.Departure)
		row.CanceledPct = cancel
		candidates = append(candidates, row)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.CumulativeDelay != b.CumulativeDelay {
			return a.CumulativeDelay > b.CumulativeDelay
		}
		if a.CanceledPct != b.CanceledPct {
			return a.CanceledPct > b.CanceledPct
		}
		return a.IATA < b.IATA
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

// maxCanceledPct extracts the larger today canceledPercentage from the two
// delay-summary blobs.
func maxCanceledPct(arrival, departure json.RawMessage) float64 {
	best := 0.0
	for _, raw := range []json.RawMessage{arrival, departure} {
		if len(raw) == 0 {
			continue
		}
		var summary struct {
			Today struct {
				CanceledPercentage string `json:"canceledPercentage"`
			} `json:"today"`
		}
		if json.Unmarshal(raw, &summary) != nil {
			continue
		}
		if pct, ok := flightyParsePercent(summary.Today.CanceledPercentage); ok && pct > best {
			best = pct
		}
	}
	return best
}
