// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports route — both directions of one origin-destination pair's
// delay/cancel/divert stats. disruptedRoutes are directional (origin-only)
// upstream; the reciprocal view needs two fetches plus reconciliation.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

// flightyDisruptedRoute is one entry of a detail page's disruptedRoutes array.
type flightyDisruptedRoute struct {
	Airport struct {
		ID   string `json:"id,omitempty"`
		IATA string `json:"iata,omitempty"`
		City string `json:"city,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"airport"`
	DelayedPercentage    string `json:"delayedPercentage,omitempty"`
	CanceledPercentage   string `json:"canceledPercentage,omitempty"`
	DivertedPercentage   string `json:"divertedPercentage,omitempty"`
	Delayed              int    `json:"delayed,omitempty"`
	Canceled             int    `json:"canceled,omitempty"`
	Diverted             int    `json:"diverted,omitempty"`
	Total                int    `json:"total,omitempty"`
}

// flightyDetailForRoute is the slice of a detail page the route command needs.
type flightyDetailForRoute struct {
	IATA string `json:"iata"`
	Name string `json:"name"`
	Today struct {
		DeparturePerformance struct {
			DisruptedRoutes []flightyDisruptedRoute `json:"disruptedRoutes"`
		} `json:"departurePerformance"`
		ArrivalPerformance struct {
			DisruptedRoutes []flightyDisruptedRoute `json:"disruptedRoutes"`
		} `json:"arrivalPerformance"`
	} `json:"today"`
}

func newNovelAirportsRouteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route <origin-iata> <dest-iata>",
		Short: "Both directions of one origin-destination pair's delay/cancel/divert stats, joined from each side's disrupted routes.",
		Long:  "Use this command for a single origin-destination pair. Do NOT use it for full side-by-side comparison; use 'airports compare', or 'airports show' for one airport's disrupted-route list.",
		Example: "  flighty-pp-cli airports route sfo den --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "origin-iata=man;dest-iata=cdg"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports route")
			}
			if len(args) < 2 || args[0] == "" || args[1] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("origin and destination are required\nUsage: %s <origin-iata> <dest-iata>", cmd.CommandPath()))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			type detailResult struct {
				query string
				detail flightyDetailForRoute
				raw   json.RawMessage
				err   error
			}
			results := make(chan detailResult, 2)
			var wg sync.WaitGroup
			for _, q := range []string{args[0], args[1]} {
				wg.Add(1)
				go func(q string) {
					defer wg.Done()
					raw, _, err := flightyFetchDetailRaw(ctx, flags, q)
					if err != nil {
						results <- detailResult{query: q, err: err}
						return
					}
					var d flightyDetailForRoute
					if json.Unmarshal(raw, &d) != nil {
						results <- detailResult{query: q, err: fmt.Errorf("parsing detail for %s", q)}
						return
					}
					results <- detailResult{query: q, detail: d, raw: raw}
				}(q)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			byQuery := map[string]flightyDetailForRoute{}
			failures := []map[string]string{}
			for res := range results {
				if res.err != nil {
					failures = append(failures, map[string]string{"airport": res.query, "error": res.err.Error()})
					continue
				}
				byQuery[res.query] = res.detail
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 2 detail fetches failed; route view may be partial\n", len(failures))
			}
			origin, originOK := byQuery[args[0]]
			dest, destOK := byQuery[args[1]]

			view := map[string]any{}
			var forward, reverse *flightyDisruptedRoute
			if originOK {
				forward = flightyFindRouteEntry(origin, args[1])
				if forward != nil {
					view["forward"] = forward
				}
			}
			if destOK {
				reverse = flightyFindRouteEntry(dest, args[0])
				if reverse != nil {
					view["reverse"] = reverse
				}
			}
			if originOK {
				view["origin"] = map[string]string{"iata": origin.IATA, "name": origin.Name}
			}
			if destOK {
				view["destination"] = map[string]string{"iata": dest.IATA, "name": dest.Name}
			}
			if len(failures) > 0 {
				view["fetchFailures"] = failures
			}
			if forward == nil && reverse == nil {
				view["note"] = "neither airport currently lists this route among its most disrupted routes — that usually means the route is not heavily disrupted today"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Route %s -> %s\n", upper(args[0]), upper(args[1]))
			if f, ok := view["forward"].(*flightyDisruptedRoute); ok && f != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  from origin's view:   delayed %s, canceled %s (%d flights)\n", f.DelayedPercentage, f.CanceledPercentage, f.Total)
			}
			if r, ok := view["reverse"].(*flightyDisruptedRoute); ok && r != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  from destination's view: delayed %s, canceled %s (%d flights)\n", r.DelayedPercentage, r.CanceledPercentage, r.Total)
			}
			return nil
		},
	}
	return cmd
}

// flightyFindRouteEntry returns the disrupted-route entry on detail whose
// other end is destIATA (checked across departure- and arrival-side arrays),
// or nil when absent.
func flightyFindRouteEntry(detail flightyDetailForRoute, destIATA string) *flightyDisruptedRoute {
	for _, pool := range [][]flightyDisruptedRoute{
		detail.Today.DeparturePerformance.DisruptedRoutes,
		detail.Today.ArrivalPerformance.DisruptedRoutes,
	} {
		for i := range pool {
			if upper(pool[i].Airport.IATA) == upper(destIATA) {
				cp := pool[i]
				return &cp
			}
		}
	}
	return nil
}
