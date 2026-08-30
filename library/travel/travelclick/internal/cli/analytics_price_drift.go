// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/store"
	"github.com/spf13/cobra"
)

type PriceDriftTimelineEntry struct {
	CapturedAt   string  `json:"captured_at"`
	NightlyRate  float64 `json:"nightly_rate"`
	RoomTypeName string  `json:"room_type_name"`
	RatePlanCode string  `json:"rate_plan_code"`
}

type PriceDriftOutput struct {
	HotelID      string                    `json:"hotel_id"`
	Alias        string                    `json:"alias,omitempty"`
	EarliestRate float64                   `json:"earliest_rate"`
	LatestRate   float64                   `json:"latest_rate"`
	Drift        float64                   `json:"drift"`
	Currency     string                    `json:"currency"`
	Timeline     []PriceDriftTimelineEntry `json:"timeline"`
}

func newNovelAnalyticsPriceDriftCmd(flags *rootFlags) *cobra.Command {
	var flagHotel string

	cmd := &cobra.Command{
		Use:     "price-drift",
		Short:   "Track how a hotel's rates move over time from your own saved search history.",
		Example: "  travelclick-pp-cli analytics price-drift --hotel 102306",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--hotel=102306",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "analytics price-drift")
			}
			if flagHotel == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hotel is required"))
			}

			// Validate --data-source is local
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			resolvedID, alias := resolveHotelIDAndAlias(cmd.Context(), flagHotel)

			db, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			snapshots, err := db.QueryRateSnapshots(cmd.Context(), resolvedID)
			if err != nil {
				return err
			}

			// No snapshots yet is a valid, empty ANSWER to "what's the price
			// drift for this hotel" -- not a failure. This store only ever
			// holds what the user has explicitly captured with
			// `rates search --save`, so an empty timeline on a fresh install
			// (or before the first --save) is expected, not an error
			// condition. Exit 0 with an empty timeline, matching how a
			// zero-match read command behaves elsewhere in this CLI; the
			// stderr hint stays for humans, but machine callers must be able
			// to tell "checked, nothing there yet" apart from "the command
			// itself failed" by exit code.
			if len(snapshots) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no rate snapshots yet for hotel %s -- run rates search %s ... --save first\n", resolvedID, resolvedID)

				emptyOutput := PriceDriftOutput{
					HotelID:  resolvedID,
					Alias:    alias,
					Timeline: []PriceDriftTimelineEntry{},
				}
				return printJSONFiltered(cmd.OutOrStdout(), emptyOutput, flags)
			}

			series := selectPriceDriftSeries(snapshots)
			target := series[len(series)-1]

			var timeline []PriceDriftTimelineEntry
			for _, sn := range series {
				timeline = append(timeline, PriceDriftTimelineEntry{
					CapturedAt:   sn.CapturedAt,
					NightlyRate:  sn.NightlyRate,
					RoomTypeName: sn.RoomTypeName,
					RatePlanCode: sn.RatePlanCode,
				})
			}

			earliest := series[0].NightlyRate
			latest := target.NightlyRate
			drift := latest - earliest

			output := PriceDriftOutput{
				HotelID:      resolvedID,
				Alias:        alias,
				EarliestRate: earliest,
				LatestRate:   latest,
				Drift:        drift,
				Currency:     target.Currency,
				Timeline:     timeline,
			}

			if len(series) == 1 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: only one snapshot found for currency %s; drift is 0.00\n", target.Currency)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Price Drift Analysis for Hotel %s (Alias: %s, Currency: %s):\n", resolvedID, alias, target.Currency)
			fmt.Fprintf(cmd.OutOrStdout(), "  Earliest Rate: %.2f\n", earliest)
			fmt.Fprintf(cmd.OutOrStdout(), "  Latest Rate:   %.2f\n", latest)
			fmt.Fprintf(cmd.OutOrStdout(), "  Price Drift:   %+.2f\n\n", drift)

			var rows [][]string
			for _, item := range timeline {
				rows = append(rows, []string{
					item.CapturedAt,
					fmt.Sprintf("%.2f", item.NightlyRate),
					item.RoomTypeName,
					item.RatePlanCode,
				})
			}

			return flags.printTable(cmd, []string{"CAPTURED_AT", "RATE", "ROOM_TYPE", "RATE_PLAN"}, rows)
		},
	}

	cmd.Flags().StringVar(&flagHotel, "hotel", "", "Hotel ID or alias to analyze")

	return cmd
}

type priceDriftSeriesKey struct {
	roomTypeCode string
	ratePlanCode string
	checkIn      string
	checkOut     string
	currency     string
}

func snapshotSeriesKey(sn store.RateSnapshot) priceDriftSeriesKey {
	return priceDriftSeriesKey{
		roomTypeCode: sn.RoomTypeCode,
		ratePlanCode: sn.RatePlanCode,
		checkIn:      sn.CheckIn,
		checkOut:     sn.CheckOut,
		currency:     sn.Currency,
	}
}

// selectPriceDriftSeries groups snapshots by product+dates+currency and
// picks one series so drift never mixes room types, rate plans, stay
// dates, or currencies. Among series present at the latest capture
// timestamp, prefer the longest history, then cheapest latest nightly
// rate, then lexicographic room, plan, check-in, check-out, and currency.
func selectPriceDriftSeries(snapshots []store.RateSnapshot) []store.RateSnapshot {
	if len(snapshots) == 0 {
		return nil
	}

	groups := make(map[priceDriftSeriesKey][]store.RateSnapshot, len(snapshots))
	latestAt := snapshots[0].CapturedAt
	for _, sn := range snapshots {
		k := snapshotSeriesKey(sn)
		groups[k] = append(groups[k], sn)
		if sn.CapturedAt > latestAt {
			latestAt = sn.CapturedAt
		}
	}

	var cands []priceDriftSeriesCandidate
	for k, series := range groups {
		sort.Slice(series, func(i, j int) bool {
			if series[i].CapturedAt != series[j].CapturedAt {
				return series[i].CapturedAt < series[j].CapturedAt
			}
			return series[i].ID < series[j].ID
		})
		var latestRate float64
		seenLatest := false
		for _, sn := range series {
			if sn.CapturedAt == latestAt {
				seenLatest = true
				latestRate = sn.NightlyRate
			}
		}
		if !seenLatest {
			continue
		}
		cands = append(cands, priceDriftSeriesCandidate{key: k, series: series, latestRate: latestRate})
	}
	if len(cands) == 0 {
		return snapshots
	}

	best := cands[0]
	for _, c := range cands[1:] {
		if preferPriceDriftCandidate(c, best) {
			best = c
		}
	}
	return best.series
}

type priceDriftSeriesCandidate struct {
	key        priceDriftSeriesKey
	series     []store.RateSnapshot
	latestRate float64
}

func preferPriceDriftCandidate(a, b priceDriftSeriesCandidate) bool {
	if len(a.series) != len(b.series) {
		return len(a.series) > len(b.series)
	}
	if a.latestRate != b.latestRate {
		return a.latestRate < b.latestRate
	}
	if a.key.roomTypeCode != b.key.roomTypeCode {
		return a.key.roomTypeCode < b.key.roomTypeCode
	}
	if a.key.ratePlanCode != b.key.ratePlanCode {
		return a.key.ratePlanCode < b.key.ratePlanCode
	}
	if a.key.checkIn != b.key.checkIn {
		return a.key.checkIn < b.key.checkIn
	}
	if a.key.checkOut != b.key.checkOut {
		return a.key.checkOut < b.key.checkOut
	}
	return a.key.currency < b.key.currency
}
