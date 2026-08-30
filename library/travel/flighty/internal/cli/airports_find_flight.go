// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports find-flight — one flight by number across arrivals AND departures
// boards: status, original vs actual time, gate, belt, terminal.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

func newNovelAirportsFindFlightCmd(flags *rootFlags) *cobra.Command {
	var flagAirline string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "find-flight <airport> <flight-number>",
		Short: "Find one flight by number across arrivals and departures boards — status, original vs actual time, gate, belt, terminal.",
		Long:  "Use this command to look up one flight by number across arrivals and departures. Do NOT use it to browse the full board; use 'airports departures' or 'airports arrivals' instead. The flight number may include the airline code (UA5072) or not (5072).",
		Example: "  flighty-pp-cli airports find-flight den UA5072 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "airport=den;flight-number=2381"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports find-flight")
			}
			if len(args) < 2 || args[0] == "" || args[1] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("airport and flight-number are required\nUsage: %s <airport> <flight-number>", cmd.CommandPath()))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			wantAirline := strings.ToUpper(strings.TrimSpace(flagAirline))
			number, queryAirline := flightySplitFlightNumber(args[1])
			if wantAirline == "" && queryAirline != "" {
				wantAirline = queryAirline
			}

			// Fan out to both boards in parallel; partial failure is surfaced,
			// not silently dropped.
			type boardResult struct {
				kind    string
				flights []flightyBoardFlight
				err     error
			}
			results := make(chan boardResult, 2)
			var wg sync.WaitGroup
			for _, kind := range []string{"arrivals", "departures"} {
				wg.Add(1)
				go func(kind string) {
					defer wg.Done()
					flights, err := flightyFetchBoard(ctx, flags, kind, args[0])
					results <- boardResult{kind: kind, flights: flights, err: err}
				}(kind)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			matches := []map[string]any{}
			boardErrors := []map[string]string{}
			for res := range results {
				if res.err != nil {
					boardErrors = append(boardErrors, map[string]string{"board": res.kind, "error": res.err.Error()})
					continue
				}
				for _, f := range res.flights {
					if number != "" && strings.TrimPrefix(f.FlightNumber, "0") != strings.TrimPrefix(number, "0") {
						continue
					}
					if wantAirline != "" && !strings.EqualFold(f.Airline.IATA, wantAirline) {
						continue
					}
					entry := map[string]any{
						"board":            res.kind,
						"flightNumber":     f.FlightNumber,
						"airline":          f.Airline,
						"city":             f.City,
						"status":           f.StatusText(),
						"originalTime":     f.OriginalTime,
						"newTime":          f.NewTime,
						"secondaryCorner":  f.SecondaryCorner,
					}
					if res.kind == "arrivals" {
						entry["origin"] = f.Departure.IATA
						entry["gateInfo"] = f.Arrival
					} else {
						entry["destination"] = f.Arrival.IATA
						entry["gateInfo"] = f.Departure
					}
					matches = append(matches, entry)
				}
			}
			if flagLimit > 0 && len(matches) > flagLimit {
				matches = matches[:flagLimit]
			}
			view := map[string]any{
				"airport":      args[0],
				"flightNumber": args[1],
				"matches":      matches,
			}
			if len(boardErrors) > 0 {
				view["fetchFailures"] = boardErrors
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 2 board fetches failed; results cover the remaining %d board(s)\n", len(boardErrors), 2-len(boardErrors))
			}
			if len(matches) == 0 {
				view["note"] = "no flight matched on either board; boards show the current operational window only"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching flight on the arrivals or departures board.")
				return nil
			}
			for _, m := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "%-9s %-8s %-10s %-18s %s\n",
					m["board"], m["flightNumber"], statusOf(m), m["city"], m["secondaryCorner"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAirline, "airline", "", "Restrict to one airline IATA code (e.g. UA)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum matches to return")
	return cmd
}

// statusOf is a tiny human-view helper for the table branch.
func statusOf(m map[string]any) string {
	if s, ok := m["status"].(string); ok {
		if s == "" {
			return "On Time"
		}
		return s
	}
	return ""
}

// flightySplitFlightNumber splits "UA5072" into ("5072", "UA"); plain
// numbers pass through unchanged.
func flightySplitFlightNumber(input string) (number, airline string) {
	s := strings.ToUpper(strings.TrimSpace(input))
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		i++
	}
	if i > 0 && i < len(s) {
		return s[i:], s[:i]
	}
	return s, ""
}
