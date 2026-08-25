// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/types"
	"github.com/spf13/cobra"
)

type CheapestNightEntry struct {
	HotelID  string  `json:"hotel_id"`
	Alias    string  `json:"alias,omitempty"`
	Date     string  `json:"date"`
	Rate     float64 `json:"rate"`
	Currency string  `json:"currency"`
}

type CheapestNightOutput struct {
	Best            *CheapestNightEntry  `json:"best"`
	CheapestByHotel []CheapestNightEntry `json:"cheapest_by_hotel"`
	FetchFailures   []map[string]any     `json:"fetch_failures,omitempty"`
}

func newNovelRatesCheapestNightCmd(flags *rootFlags) *cobra.Command {
	var flagHotels string
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:     "cheapest-night",
		Short:   "Scan several hotels' calendars at once and return the single best hotel+date combination.",
		Example: "  travelclick-pp-cli rates cheapest-night --hotels 'made-nyc,102306' --from 2026-09-01 --to 2026-10-31",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--hotels=102306;--from=2026-09-01;--to=2026-09-30",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rates cheapest-night")
			}
			if flagHotels == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hotels is required"))
			}
			if flagFrom == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from is required"))
			}
			if flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required"))
			}

			// Validate --data-source is live / auto
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			tokens := strings.Split(flagHotels, ",")
			var hotelTokens []string
			for _, t := range tokens {
				t = strings.TrimSpace(t)
				if t != "" {
					hotelTokens = append(hotelTokens, t)
				}
			}

			type fetchResult struct {
				best    CheapestNightEntry
				hasRate bool
			}

			// Resolve every --hotels token against the local store BEFORE
			// fanning out -- see resolveHotelTokensSequential's doc comment.
			// Resolving concurrently from inside the fan-out closure crashed
			// this command with a SIGBUS fault.
			resolutions := resolveHotelTokensSequential(ctx, hotelTokens)

			results, errs := cliutil.FanoutRun(ctx, hotelTokens, func(token string) string {
				return token
			}, func(ctx context.Context, token string) (fetchResult, error) {
				resolvedID, alias := resolutions[token].HotelID, resolutions[token].Alias
				path := replacePathParam("/ibe-shop/v1/hotel/{hotel_id}/basicavail/multi-room", "hotel_id", resolvedID)

				hotelCodeInt, _ := strconv.Atoi(resolvedID)
				bodyMap := map[string]any{
					"hotelCode":         hotelCodeInt,
					"dateIn":            flagFrom,
					"dateOut":           flagTo,
					"lang":              "EN_US",
					"currency":          "USD",
					"bookerIdentifier":  "",
					"partnerIdentifier": "Web4_Desktop",
					"multiRoomOccupancy": []any{
						map[string]any{
							"adults":   "2",
							"infant":   "0",
							"children": 0,
						},
					},
				}

				data, _, err := c.PostWithParams(ctx, path, nil, bodyMap)
				if err != nil {
					return fetchResult{}, err
				}

				var envelope struct {
					Dates []types.MultiRoomDate `json:"dates"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					return fetchResult{}, err
				}

				best, hasRate := computeCheapestHotelNight(envelope.Dates, resolvedID, alias)
				return fetchResult{
					best:    best,
					hasRate: hasRate,
				}, nil
			})

			var cheapestByHotel []CheapestNightEntry
			var bestGlobal *CheapestNightEntry

			for _, r := range results {
				if r.Value.hasRate {
					cheapestByHotel = append(cheapestByHotel, r.Value.best)
					if bestGlobal == nil || r.Value.best.Rate < bestGlobal.Rate {
						bestGlobal = &CheapestNightEntry{
							HotelID:  r.Value.best.HotelID,
							Alias:    r.Value.best.Alias,
							Date:     r.Value.best.Date,
							Rate:     r.Value.best.Rate,
							Currency: r.Value.best.Currency,
						}
					}
				}
			}

			// Sort by Rate ascending
			sort.Slice(cheapestByHotel, func(i, j int) bool {
				return cheapestByHotel[i].Rate < cheapestByHotel[j].Rate
			})

			var fetchFailures []map[string]any
			for _, fe := range errs {
				fetchFailures = append(fetchFailures, map[string]any{
					"hotel": fe.Source,
					"error": fe.Err.Error(),
				})
			}

			if len(errs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d hotel calendar fetch failures encountered\n", len(errs))
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			}

			output := CheapestNightOutput{
				Best:            bestGlobal,
				CheapestByHotel: cheapestByHotel,
				FetchFailures:   fetchFailures,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}

			if bestGlobal == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "No available rates found across all hotels in this date range.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Globally Best Rate found at Hotel %s (Alias: %s) on %s: %.2f %s\n\n",
				bestGlobal.HotelID, bestGlobal.Alias, bestGlobal.Date, bestGlobal.Rate, bestGlobal.Currency)

			var rows [][]string
			for _, item := range cheapestByHotel {
				rows = append(rows, []string{
					item.HotelID,
					item.Alias,
					item.Date,
					fmt.Sprintf("%.2f", item.Rate),
					item.Currency,
				})
			}

			return flags.printTable(cmd, []string{"HOTEL_ID", "ALIAS", "BEST_DATE", "RATE", "CURRENCY"}, rows)
		},
	}

	cmd.Flags().StringVar(&flagHotels, "hotels", "", "Comma-separated list of hotel IDs or aliases to scan")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start date of the scan window (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End date of the scan window (YYYY-MM-DD)")

	return cmd
}

func computeCheapestHotelNight(dates []types.MultiRoomDate, hotelID, alias string) (CheapestNightEntry, bool) {
	var best CheapestNightEntry
	hasRate := false

	for _, d := range dates {
		if d.IsAvailable && d.Rate.MinRate > 0 {
			if !hasRate || d.Rate.MinRate < best.Rate {
				best = CheapestNightEntry{
					HotelID:  hotelID,
					Alias:    alias,
					Date:     d.Date,
					Rate:     d.Rate.MinRate,
					Currency: "USD",
				}
				hasRate = true
			}
		}
	}
	return best, hasRate
}
