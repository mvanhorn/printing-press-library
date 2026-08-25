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
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/types"
	"github.com/spf13/cobra"
)

type CompareHotelResult struct {
	HotelID      string  `json:"hotel_id"`
	Alias        string  `json:"alias,omitempty"`
	LowestTotal  float64 `json:"lowest_total"`
	Currency     string  `json:"currency"`
	RoomTypeName string  `json:"room_type_name"`
	RatePlanCode string  `json:"rate_plan_code"`
	// FeeInclusive is true when LowestTotal comes from nightly AmtTotal
	// plus service/resort fees. False means the hotel had no nightly
	// totals and fell back to average-rate * nights (room-only).
	FeeInclusive bool `json:"-"`
}

type CompareOutput struct {
	Results       []CompareHotelResult `json:"results"`
	Incomparable  []CompareHotelResult `json:"incomparable,omitempty"`
	FetchFailures []map[string]any     `json:"fetch_failures,omitempty"`
}

func newNovelRatesCompareCmd(flags *rootFlags) *cobra.Command {
	var flagHotels string
	var flagCheckIn string
	var flagCheckOut string

	cmd := &cobra.Command{
		Use:     "compare",
		Short:   "Check several boutique hotels for the same dates in one call, ranked by lowest fee-inclusive total.",
		Example: "  travelclick-pp-cli rates compare --hotels '102306,made-nyc' --check-in 2026-09-15 --check-out 2026-09-18",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--hotels=102306;--check-in=2026-09-15;--check-out=2026-09-18",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rates compare")
			}
			if flagHotels == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hotels is required"))
			}
			if flagCheckIn == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--check-in is required"))
			}
			if flagCheckOut == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--check-out is required"))
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
				result  CompareHotelResult
				hasRate bool
			}

			// Resolve every --hotels token against the local store BEFORE
			// fanning out. See resolveHotelTokensSequential's doc comment:
			// resolving concurrently from inside the fan-out closure crashed
			// this command with a SIGBUS fault.
			resolutions := resolveHotelTokensSequential(ctx, hotelTokens)

			results, errs := cliutil.FanoutRun(ctx, hotelTokens, func(token string) string {
				return token
			}, func(ctx context.Context, token string) (fetchResult, error) {
				resolvedID, alias := resolutions[token].HotelID, resolutions[token].Alias
				path := replacePathParam("/ibe-shop/v1/hotel/{hotel_id}/avail", "hotel_id", resolvedID)

				params := map[string]string{
					"dateIn":            flagCheckIn,
					"dateOut":           flagCheckOut,
					"adults":            "2",
					"infants":           "0",
					"rooms":             "1",
					"currency":          "USD",
					"lang":              "EN_US",
					"partnerIdentifier": "Web4_Desktop",
				}

				data, err := c.GetWithHeaders(ctx, path, params, nil)
				if err != nil {
					return fetchResult{}, err
				}

				var envelope struct {
					RoomStays    []types.AvailRoomStay `json:"roomStays"`
					CurrencyCode string                `json:"currencyCode"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					return fetchResult{}, err
				}

				tIn, _ := time.Parse("2006-01-02", flagCheckIn)
				tOut, _ := time.Parse("2006-01-02", flagCheckOut)
				nights := int(tOut.Sub(tIn).Hours() / 24)
				if nights <= 0 {
					nights = 1
				}

				best, hasRate := computeLowestHotelRate(envelope.RoomStays, resolvedID, alias, nights, envelope.CurrencyCode)
				return fetchResult{
					result:  best,
					hasRate: hasRate,
				}, nil
			})

			var collected []CompareHotelResult
			for _, r := range results {
				if r.Value.hasRate {
					collected = append(collected, r.Value.result)
				}
			}

			finalResults, incomparable := rankHotelCompareResults(collected)

			var fetchFailures []map[string]any
			for _, fe := range errs {
				fetchFailures = append(fetchFailures, map[string]any{
					"hotel": fe.Source,
					"error": fe.Err.Error(),
				})
			}

			if len(errs) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d hotel fetch failures encountered\n", len(errs))
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			}

			output := CompareOutput{
				Results:       finalResults,
				Incomparable:  incomparable,
				FetchFailures: fetchFailures,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}

			if len(finalResults) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No rates found for comparison.")
				return nil
			}

			var rows [][]string
			for _, item := range finalResults {
				rows = append(rows, []string{
					item.HotelID,
					item.Alias,
					item.RoomTypeName,
					item.RatePlanCode,
					fmt.Sprintf("%.2f", item.LowestTotal),
					item.Currency,
				})
			}

			if err := flags.printTable(cmd, []string{"HOTEL_ID", "ALIAS", "ROOM_TYPE", "RATE_PLAN", "TOTAL_COST", "CURRENCY"}, rows); err != nil {
				return err
			}
			if len(incomparable) == 0 {
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Incomparable hotels (fee-exclusive average-rate fallback; not ranked against fee-inclusive totals):")
			var extra [][]string
			for _, item := range incomparable {
				extra = append(extra, []string{
					item.HotelID,
					item.Alias,
					item.RoomTypeName,
					item.RatePlanCode,
					fmt.Sprintf("%.2f", item.LowestTotal),
					item.Currency,
				})
			}
			return flags.printTable(cmd, []string{"HOTEL_ID", "ALIAS", "ROOM_TYPE", "RATE_PLAN", "ROOM_ONLY", "CURRENCY"}, extra)
		},
	}

	cmd.Flags().StringVar(&flagHotels, "hotels", "", "Comma-separated list of hotel IDs or aliases to compare")
	cmd.Flags().StringVar(&flagCheckIn, "check-in", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagCheckOut, "check-out", "", "Check-out date (YYYY-MM-DD)")

	return cmd
}

func compareResultCurrency(payloadCurrency string) string {
	if c := strings.TrimSpace(payloadCurrency); c != "" {
		return c
	}
	return "USD"
}

type hotelRateCandidate struct {
	cost         float64
	roomTypeName string
	ratePlanCode string
}

func collectHotelRateCandidates(stays []types.AvailRoomStay, nights int) (inclusive, exclusive []hotelRateCandidate) {
	for _, stay := range stays {
		for _, rt := range stay.RoomTypes {
			for _, rpr := range rt.AverageRates {
				var cost float64
				var matches int
				for _, nr := range rt.NightlyRates {
					if nr.RatePlanCode == rpr.RatePlanCode {
						cost += nr.AmtTotal + nr.TotalServiceChargeExclusive + nr.TotalResortFeeExclusive
						matches++
					}
				}
				if matches > 0 {
					if cost > 0 {
						inclusive = append(inclusive, hotelRateCandidate{cost, rt.RoomTypeName, rpr.RatePlanCode})
					}
					continue
				}
				// RatePlanRate has no fee fields. Do not invent them, and do
				// not let room-only average*nights compete with fee-inclusive
				// nightly totals from other plans at the same hotel.
				fallback := rpr.Rate * float64(nights)
				if fallback > 0 {
					exclusive = append(exclusive, hotelRateCandidate{fallback, rt.RoomTypeName, rpr.RatePlanCode})
				}
			}
		}
	}
	return inclusive, exclusive
}

func computeLowestHotelRate(stays []types.AvailRoomStay, hotelID, alias string, nights int, currency string) (CompareHotelResult, bool) {
	inclusive, exclusive := collectHotelRateCandidates(stays, nights)
	pool := inclusive
	feeInclusive := true
	if len(pool) == 0 {
		pool = exclusive
		feeInclusive = false
	}

	var best CompareHotelResult
	hasRate := false
	for _, c := range pool {
		if !hasRate || c.cost < best.LowestTotal {
			best = CompareHotelResult{
				HotelID:      hotelID,
				Alias:        alias,
				LowestTotal:  c.cost,
				Currency:     compareResultCurrency(currency),
				RoomTypeName: c.roomTypeName,
				RatePlanCode: c.ratePlanCode,
				FeeInclusive: feeInclusive,
			}
			hasRate = true
		}
	}
	return best, hasRate
}

// rankHotelCompareResults ranks hotels by LowestTotal only when those
// totals are comparable. Fee-exclusive average-rate fallback hotels
// cannot be ranked against fee-inclusive hotels: a room-only 300 would
// otherwise beat a cheaper all-in 500. When every hotel is fallback-only,
// ranking them against each other is apples-to-apples.
func rankHotelCompareResults(hotels []CompareHotelResult) (ranked, incomparable []CompareHotelResult) {
	hasInclusive := false
	for _, h := range hotels {
		if h.FeeInclusive {
			hasInclusive = true
			break
		}
	}
	if hasInclusive {
		for _, h := range hotels {
			if h.FeeInclusive {
				ranked = append(ranked, h)
			} else {
				incomparable = append(incomparable, h)
			}
		}
	} else {
		ranked = append(ranked, hotels...)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].LowestTotal < ranked[j].LowestTotal
	})
	sort.Slice(incomparable, func(i, j int) bool {
		return incomparable[i].LowestTotal < incomparable[j].LowestTotal
	})
	return ranked, incomparable
}

// hotelResolution is the result of resolving one --hotels token (a raw hotel
// ID or a saved alias) to its underlying hotel ID.
type hotelResolution struct {
	HotelID string
	Alias   string
}

// resolveHotelTokensSequential resolves every token against the local store
// using a single open connection, one token at a time, IN THE CALLING
// GOROUTINE. This must run to completion before any parallel fan-out begins.
//
// modernc.org/sqlite (this project's pure-Go, CGO-free driver) is not safe
// for multiple goroutines to open/close connections against the same
// on-disk file concurrently -- doing so previously crashed rates compare,
// rates cheapest-night, and codes check-all with SIGBUS faults because each
// fan-out worker called resolveHotelIDAndAlias (which opens and closes its
// own store handle) at the same time. Resolving aliases up front, before
// cliutil.FanoutRun starts, keeps all SQLite access single-threaded; the
// parallel fan-out closures below only touch the pre-resolved map and the
// HTTP client, never the store.
func resolveHotelTokensSequential(ctx context.Context, tokens []string) map[string]hotelResolution {
	resolved := make(map[string]hotelResolution, len(tokens))

	db, err := openStore(ctx)
	if err != nil || db == nil {
		// No local store available (e.g. first run before any table
		// exists) -- every token is treated as a raw hotel ID.
		for _, token := range tokens {
			t := strings.TrimSpace(token)
			resolved[token] = hotelResolution{HotelID: t}
		}
		return resolved
	}
	defer db.Close()

	for _, token := range tokens {
		t := strings.TrimSpace(token)
		res := hotelResolution{HotelID: t}
		if id, err := db.ResolveHotelID(ctx, t); err == nil && id != t {
			res.HotelID = id
			res.Alias = t
		}
		resolved[token] = res
	}
	return resolved
}

// resolveHotelIDAndAlias resolves a single token against the local store.
// Callers on a hot fan-out path MUST NOT call this concurrently -- use
// resolveHotelTokensSequential up front instead. This single-token form
// remains for callers that only ever resolve one hotel per invocation
// (e.g. analytics price-drift), where there is no concurrency to guard against.
func resolveHotelIDAndAlias(ctx context.Context, token string) (string, string) {
	token = strings.TrimSpace(token)
	db, err := openStore(ctx)
	if err != nil || db == nil {
		return token, ""
	}
	defer db.Close()
	resolved, err := db.ResolveHotelID(ctx, token)
	if err != nil {
		return token, ""
	}
	if resolved != token {
		return resolved, token
	}
	return resolved, ""
}

func openStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("travelclick-pp-cli")
	return store.OpenWithContext(ctx, dbPath)
}
