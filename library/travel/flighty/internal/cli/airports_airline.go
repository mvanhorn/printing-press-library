// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports airline — one airline's delay/cancel/divert footprint aggregated
// across the most disrupted airports. The site exposes disruptedAirlines
// per-airport only; no surface aggregates one airline across airports.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// flightyDisruptedAirline is one entry of a detail page's disruptedAirlines.
type flightyDisruptedAirline struct {
	Airline struct {
		ID   string `json:"id,omitempty"`
		IATA string `json:"iata,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"airline"`
	DelayedPercentage  string `json:"delayedPercentage,omitempty"`
	CanceledPercentage string `json:"canceledPercentage,omitempty"`
	DivertedPercentage string `json:"divertedPercentage,omitempty"`
	Delayed            int    `json:"delayed,omitempty"`
	Canceled           int    `json:"canceled,omitempty"`
	Diverted           int    `json:"diverted,omitempty"`
	Total              int    `json:"total,omitempty"`
}

// flightyDetailForAirline is the slice of a detail page the airline command needs.
type flightyDetailForAirline struct {
	IATA string `json:"iata"`
	Name string `json:"name"`
	Today struct {
		DeparturePerformance struct {
			NumOperations     int                      `json:"numOperations,omitempty"`
			DisruptedAirlines []flightyDisruptedAirline `json:"disruptedAirlines"`
		} `json:"departurePerformance"`
		ArrivalPerformance struct {
			NumOperations     int                      `json:"numOperations,omitempty"`
			DisruptedAirlines []flightyDisruptedAirline `json:"disruptedAirlines"`
		} `json:"arrivalPerformance"`
	} `json:"today"`
}

// flightyAirlineAirportRow is the per-airport breakdown row for one airline.
type flightyAirlineAirportRow struct {
	AirportIATA string `json:"airportIata"`
	AirportName string `json:"airportName,omitempty"`
	Side        string `json:"side"`
	Delayed     int    `json:"delayed,omitempty"`
	Canceled    int    `json:"canceled,omitempty"`
	Diverted    int    `json:"diverted,omitempty"`
	Total       int    `json:"total,omitempty"`
	DelayedPct  string `json:"delayedPercentage,omitempty"`
}

func newNovelAirportsAirlineCmd(flags *rootFlags) *cobra.Command {
	var flagTop int
	var flagRegion string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "airline <airline-iata>",
		Short: "One airline's delay/cancel/divert footprint aggregated across every synced airport — impossible per-airport on the site.",
		Long:  "Use this command for network-wide airline disruption aggregation. Do NOT use it for one airport's disrupted airlines; use 'airports show' instead. Ranks targets from the local mirror (one live catalog fetch when the mirror is empty), then fetches their detail pages live.",
		Example: "  flighty-pp-cli airports airline UA --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "airline-iata=ua;--top=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports airline")
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("airline IATA code is required\nUsage: %s <airline-iata>", cmd.CommandPath()))
			}
			airlineCode := upper(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Scan cap: top-N disrupted airports from the catalog (bounded
			// scan; --top widens, dogfood narrows).
			top := flagTop
			if top <= 0 {
				top = 10
			}
			airports, err := flightyCatalogAutoWithHints(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if len(airports) == 0 {
				return fmt.Errorf("catalog unavailable. run 'flighty-pp-cli sync --resources airports --full' first")
			}
			targets := flightyRankWorst(airports, flagRegion, "", top)

			// Resolve every slug up front: each target is already a catalog
			// row, so no per-goroutine catalog fetch (avoids N+1 amplification).
			slugOf := map[string]string{}
			for _, t := range targets {
				ap, err := flightyResolveAirport(airports, t.IATA)
				if err == nil && ap.Slug != "" {
					slugOf[t.IATA] = ap.Slug
				}
			}

			type detailResult struct {
				row    flightyWorstRow
				detail flightyDetailForAirline
				err    error
			}
			results := make(chan detailResult, len(targets))
			var wg sync.WaitGroup
			// Bound concurrent detail fetches so a wide --top cannot stampede
			// the site with simultaneous requests.
			sem := make(chan struct{}, 4)
			for _, t := range targets {
				wg.Add(1)
				go func(t flightyWorstRow) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					slug, ok := slugOf[t.IATA]
					if !ok {
						results <- detailResult{row: t, err: fmt.Errorf("no URL slug for %s", t.IATA)}
						return
					}
					raw, err := flightyFetchDetailBySlug(ctx, flags, slug)
					if err != nil {
						results <- detailResult{row: t, err: err}
						return
					}
					var d flightyDetailForAirline
					if json.Unmarshal(raw, &d) != nil {
						results <- detailResult{row: t, err: fmt.Errorf("parsing detail for %s", t.IATA)}
						return
					}
					results <- detailResult{row: t, detail: d}
				}(t)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			rows := []flightyAirlineAirportRow{}
			failures := []map[string]string{}
			agg := map[string]int{}
			for res := range results {
				if res.err != nil {
					failures = append(failures, map[string]string{"airport": res.row.IATA, "error": res.err.Error()})
					continue
				}
				for _, side := range []struct {
					name string
					list []flightyDisruptedAirline
				}{
					{"departures", res.detail.Today.DeparturePerformance.DisruptedAirlines},
					{"arrivals", res.detail.Today.ArrivalPerformance.DisruptedAirlines},
				} {
					for _, da := range side.list {
						if upper(da.Airline.IATA) != airlineCode {
							continue
						}
						rows = append(rows, flightyAirlineAirportRow{
							AirportIATA: res.row.IATA,
							AirportName: res.row.Name,
							Side:        side.name,
							Delayed:     da.Delayed,
							Canceled:    da.Canceled,
							Diverted:    da.Diverted,
							Total:       da.Total,
							DelayedPct:  da.DelayedPercentage,
						})
						agg["delayed"] += da.Delayed
						agg["canceled"] += da.Canceled
						agg["diverted"] += da.Diverted
						agg["total"] += da.Total
					}
				}
			}
			view := map[string]any{
				"airline":     airlineCode,
				"airportsScanned": len(targets),
				"breakdown":   rows,
				"aggregate":   agg,
			}
			if agg["total"] > 0 {
				view["aggregate"] = map[string]any{
					"delayed":  agg["delayed"],
					"canceled": agg["canceled"],
					"diverted": agg["diverted"],
					"total":    agg["total"],
				}
			}
			if len(failures) > 0 {
				view["fetchFailures"] = failures
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; aggregation covers the remaining %d airport(s)\n",
					len(failures), len(targets), len(targets)-len(failures))
			}
			if len(rows) == 0 {
				view["note"] = fmt.Sprintf("airline %s not among the disrupted airlines at the %d most disrupted airports; raise --top to scan more", airlineCode, len(targets))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No disruption entries for %s at the scanned airports.\n", airlineCode)
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-4s %-11s delayed:%4d canceled:%3d total:%4d (%s%%)\n",
					r.AirportIATA, r.Side, r.Delayed, r.Canceled, r.Total, r.DelayedPct)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "TOTAL delayed:%d canceled:%d diverted:%d across %d entries\n",
				agg["delayed"], agg["canceled"], agg["diverted"], len(rows))
			return nil
		},
	}
	cmd.Flags().IntVar(&flagTop, "top", 10, "How many of the most disrupted airports to scan")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Restrict the scan to one map region")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// flightyDisruptedAirlineSide adapts the two performance arrays.
type flightyDisruptedAirlineSide struct {
	list []flightyDisruptedAirline
}

var _ = strings.ToUpper // keep strings imported for helper use above
