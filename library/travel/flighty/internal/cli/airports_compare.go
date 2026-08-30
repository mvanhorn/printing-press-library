// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports compare — side-by-side status, delays, warnings, and flight rules
// for two airports. The web app shows one airport at a time.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

func newNovelAirportsCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <airport-a> <airport-b>",
		Short: "Side-by-side status, delays, warnings, and flight rules for two airports.",
		Long:  "Use this command to compare two airports side by side. Do NOT use it for a single airport's full detail; use 'airports show' instead.",
		Example: "  flighty-pp-cli airports compare sfo oak --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "airport-a=den;airport-b=las"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports compare")
			}
			if len(args) < 2 || args[0] == "" || args[1] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two airports are required\nUsage: %s <airport-a> <airport-b>", cmd.CommandPath()))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			type detailResult struct {
				query  string
				raw    json.RawMessage
				slug   string
				err    error
			}
			results := make(chan detailResult, 2)
			var wg sync.WaitGroup
			for _, q := range []string{args[0], args[1]} {
				wg.Add(1)
				go func(q string) {
					defer wg.Done()
					raw, slug, err := flightyFetchDetailRaw(ctx, flags, q)
					results <- detailResult{query: q, raw: raw, slug: slug, err: err}
				}(q)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			byQuery := map[string]json.RawMessage{}
			failures := []map[string]string{}
			for res := range results {
				if res.err != nil {
					failures = append(failures, map[string]string{"airport": res.query, "error": res.err.Error()})
					continue
				}
				byQuery[strings3Normalize(res.query)] = res.raw
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 2 detail fetches failed\n", len(failures))
			}
			a, aOK := byQuery[strings3Normalize(args[0])]
			b, bOK := byQuery[strings3Normalize(args[1])]
			if !aOK || !bOK {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"error": "one or both airports could not be fetched", "fetchFailures": failures}, flags)
				}
				return fmt.Errorf("one or both airports could not be fetched")
			}

			view := map[string]any{
				"a":                 json.RawMessage(a),
				"b":                 json.RawMessage(b),
				"comparisonFields": []string{"airportWeather.flightRulesTitle", "airportWeather.conditionTitle", "airportWeather.rawMetar", "today.departurePerformance", "today.arrivalPerformance"},
			}
			if len(failures) > 0 {
				view["fetchFailures"] = failures
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			flightyPrintCompareHuman(cmd, args[0], string(a), args[1], string(b))
			return nil
		},
	}
	return cmd
}

func strings3Normalize(s string) string { return s }

// flightyPrintCompareHuman renders the side-by-side summary lines.
func flightyPrintCompareHuman(cmd *cobra.Command, qA, a, qB, b string) {
	rowFor := func(raw json.RawMessage) (string, string, string, string) {
		var d struct {
			IATA           string `json:"iata"`
			Name           string `json:"name"`
			AirportWeather struct {
				FlightRulesTitle string `json:"flightRulesTitle"`
				ConditionTitle   string `json:"conditionTitle"`
				Temperature      float64 `json:"temperature"`
			} `json:"airportWeather"`
			Today struct {
				DeparturePerformance struct {
					OnTime struct {
						Percentage string `json:"percentage"`
					} `json:"onTime"`
				} `json:"departurePerformance"`
				ArrivalPerformance struct {
					OnTime struct {
						Percentage string `json:"percentage"`
					} `json:"onTime"`
				} `json:"arrivalPerformance"`
			} `json:"today"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			return "?", "?", "?", "?"
		}
		return d.IATA, d.AirportWeather.FlightRulesTitle, d.AirportWeather.ConditionTitle, d.Today.DeparturePerformance.OnTime.Percentage
	}
	iataA, rulesA, condA, depA := rowFor(json.RawMessage(a))
	iataB, rulesB, condB, depB := rowFor(json.RawMessage(b))
	fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-16s %-16s\n", "", iataA+" ("+qA+")", iataB+" ("+qB+")")
	fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-16s %-16s\n", "flight rules", rulesA, rulesB)
	fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-16s %-16s\n", "weather", condA, condB)
	fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-16s %-16s\n", "dep on-time", depA, depB)
}
