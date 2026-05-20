// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — KILLER command added by
// /printing-press-amend.
//
// Composes a cash search + an award search for the same itinerary and
// applies a cents-per-point valuation (default: Alaska/Atmos from The
// Points Guy's monthly valuations page, scraped on first use and cached
// for 30 days) to emit an apples-to-apples comparison:
//
//   cash_usd, miles + taxes_usd, effective_cpp, multiple over TPG
//   baseline, TPG-valued dollar cost of paying with points.
//
// Soft-fallback chain: --cpp override → fresh cache → live TPG fetch →
// stale cache → constant fallback (1.4 cpp for Atmos as of 2026-05).
// Never hard-fails the command on a valuation lookup issue; emits a
// stderr warning and surfaces the source in the response envelope.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/alaska-airlines/internal/valuation"
	"github.com/spf13/cobra"
)

func newFlightsValueCompareCmd(flags *rootFlags) *cobra.Command {
	var flagOrigin string
	var flagDest string
	var flagDepart string
	var flagReturn string
	var flagCabin string
	var flagAdults string
	var flagChildren string
	var flagLapInfants string
	var flagMaxStops int
	var flagProgram string
	var flagCPP float64
	var flagNoValuationCache bool
	var flagLocale string

	cmd := &cobra.Command{
		Use:   "value-compare",
		Short: "Compare cash vs award (points) pricing for one itinerary and apply a TPG cents-per-point valuation.",
		Long: `Runs paired cash and award searches against alaskaair.com for the same
itinerary, looks up the program's cents-per-point baseline (default:
Alaska/Atmos from The Points Guy's monthly valuations page, scraped on
first use and cached locally for 30 days), and emits a structured
comparison:

  cash_usd                — total cash fare for the cheapest matching itinerary
  miles + taxes_usd       — cheapest award redemption for the same itinerary
  effective_cpp_cents     — cents-per-point you actually get from the points option
  baseline_cpp_cents      — TPG's published valuation (or your --cpp override)
  multiple                — effective_cpp / baseline_cpp
  tpg_valued_usd          — apples-to-apples cost of the points option at the baseline

Example: is the FCO -> SEA Alaska nonstop a good Atmos redemption?

  alaska-airlines-pp-cli flights value-compare \
    --origin FCO --destination SEA --depart 2026-08-30 \
    --cabin economy --json

The command never hard-fails on a valuation lookup issue — if TPG is
unreachable and no cache is present, it falls back to a constant
baseline and surfaces the source in meta.cpp_baseline_source.`,
		Example:     "  alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --cabin economy --json",
		Annotations: map[string]string{"pp:endpoint": "flights.value_compare", "pp:method": "GET", "pp:path": "/search/results/__data.json (paired)", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagOrigin == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "origin")
			}
			if flagDest == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "destination")
			}
			if flagDepart == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "depart")
			}

			program := valuation.Program(flagProgram)
			if _, ok := valuation.BySlug(program); !ok {
				return fmt.Errorf("unknown --program %q (known: %v)", flagProgram, valuation.Slugs())
			}

			if flags.dryRun {
				return printValueCompareDryRunTo(cmd.OutOrStdout(), flagOrigin, flagDest, flagDepart, flagReturn, flagCabin, program)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			roundTrip := "false"
			if flagReturn != "" {
				roundTrip = "true"
			}

			// Award (miles + taxes) search.
			awardParams := buildAwardSearchParams(awardSearchInput{
				Origin:      flagOrigin,
				Destination: flagDest,
				Depart:      flagDepart,
				Return:      flagReturn,
				Adults:      flagAdults,
				Children:    flagChildren,
				LapInfants:  flagLapInfants,
				RoundTrip:   roundTrip,
				Cabin:       flagCabin,
				Locale:      flagLocale,
			})

			// Cash search — the same endpoint without the award toggle.
			cashParams := buildCashSearchParams(cashSearchInput{
				Origin:      flagOrigin,
				Destination: flagDest,
				Depart:      flagDepart,
				Return:      flagReturn,
				Adults:      flagAdults,
				Children:    flagChildren,
				LapInfants:  flagLapInfants,
				RoundTrip:   roundTrip,
				Locale:      flagLocale,
			})

			// Run cash search, award search, and valuation lookup
			// concurrently — all three are independent. Cash and
			// award hit the same alaskaair.com endpoint but the
			// existing AdaptiveLimiter serializes outbound requests
			// to that host, so concurrency is polite. Valuation
			// goes to a different host (or the local cache).
			path := "/search/results/__data.json"
			var (
				wg       sync.WaitGroup
				cashData, awardData json.RawMessage
				cashProv, awardProv DataProvenance
				cashErr, awardErr   error
				vRes                valuation.Result
				vErr                error
			)
			wg.Add(3)
			go func() {
				defer wg.Done()
				cashData, cashProv, cashErr = resolveRead(cmd.Context(), c, flags, "flights", false, path, cashParams, nil)
			}()
			go func() {
				defer wg.Done()
				awardData, awardProv, awardErr = resolveRead(cmd.Context(), c, flags, "flights", false, path, awardParams, nil)
			}()
			go func() {
				defer wg.Done()
				lookupOpts := valuation.LookupOptions{
					Override:     flagCPP,
					ForceRefresh: flagNoValuationCache,
				}
				vRes, vErr = valuation.Lookup(cmd.Context(), program, lookupOpts)
			}()
			wg.Wait()

			if cashErr != nil {
				return classifyAPIError(cashErr, flags)
			}
			if awardErr != nil {
				return classifyAPIError(awardErr, flags)
			}
			if vErr != nil {
				return vErr
			}
			if vRes.Warning != nil {
				fmt.Fprintf(os.Stderr, "warning: valuation lookup degraded to %s: %v\n", vRes.Source, vRes.Warning)
			}

			// Mirror the existing single-search provenance lines on
			// stderr so an agent sees both calls.
			{
				var n []json.RawMessage
				_ = json.Unmarshal(cashData, &n)
				printProvenance(cmd, len(n), cashProv)
				_ = json.Unmarshal(awardData, &n)
				printProvenance(cmd, len(n), awardProv)
			}

			// Extract fares. For the award side, pass the resolved
			// baseline cpp so the ranking minimizes total
			// out-of-pocket cost (miles*cpp/100 + taxes) rather than
			// miles only — otherwise a 25k+$500 option would beat a
			// 30k+$5 option on the miles criterion, but the latter
			// is dramatically cheaper at any realistic cpp.
			cashFare := extractLowestFare(cashData, fareModeCash, flagCabin, flagMaxStops, 0)
			awardFare := extractLowestFare(awardData, fareModeAward, flagCabin, flagMaxStops, vRes.CPPCents)

			// Build the response envelope. Follows the award-cheapest
			// pattern: custom map[string]any with rich meta + results.
			envelope := buildValueCompareEnvelope(valueCompareInputs{
				Origin:      flagOrigin,
				Destination: flagDest,
				Depart:      flagDepart,
				Return:      flagReturn,
				Cabin:       flagCabin,
				Adults:      flagAdults,
				Children:    flagChildren,
				MaxStops:    flagMaxStops,
				RoundTrip:   roundTrip == "true",
				Program:     program,
				CashFare:    cashFare,
				AwardFare:   awardFare,
				Valuation:   vRes,
			})

			out, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
		},
	}

	cmd.Flags().StringVar(&flagOrigin, "origin", "", "Origin IATA code (e.g. FCO)")
	cmd.Flags().StringVar(&flagDest, "destination", "", "Destination IATA code (e.g. SEA)")
	cmd.Flags().StringVar(&flagDepart, "depart", "", "Outbound date YYYY-MM-DD")
	cmd.Flags().StringVar(&flagReturn, "return", "", "Return date YYYY-MM-DD (omit for one-way)")
	cmd.Flags().StringVar(&flagCabin, "cabin", "economy", "Cabin to compare (economy, premium, business, first); locked to one cabin across both searches")
	cmd.Flags().StringVar(&flagAdults, "adults", "1", "Adult passenger count")
	cmd.Flags().StringVar(&flagChildren, "children", "0", "Child passenger count")
	cmd.Flags().StringVar(&flagLapInfants, "lap-infants", "0", "Lap-infant count")
	cmd.Flags().IntVar(&flagMaxStops, "max-stops", -1, "Maximum number of stops per itinerary; -1 means unset")
	cmd.Flags().StringVar(&flagProgram, "program", string(valuation.ProgramAtmos), "Loyalty program for the valuation lookup (v1: atmos)")
	cmd.Flags().Float64Var(&flagCPP, "cpp", 0, "Override cents-per-point baseline (e.g. --cpp 1.4). When 0, looks up TPG.")
	cmd.Flags().BoolVar(&flagNoValuationCache, "no-valuation-cache", false, "Force re-scrape of the TPG valuation even if a fresh cache entry exists")
	cmd.Flags().StringVar(&flagLocale, "locale", "en-us", "Locale string passed to alaskaair.com")

	return cmd
}

// cashSearchInput mirrors awardSearchInput's role for the cash side.
// Same fields minus Cabin (cash search exposes SpecFare via the award
// path; cash mode doesn't need it).
type cashSearchInput struct {
	Origin      string
	Destination string
	Depart      string
	Return      string
	Adults      string
	Children    string
	LapInfants  string
	RoundTrip   string
	Locale      string
}

// buildCashSearchParams composes the query-param map for a cash search.
// Mirrors the manual composition in flights_search.go without any of the
// award-mode toggles (no ShoppingMethod, no UPG, no OT/DT). Kept as a
// pure function so value-compare can call it the same way award-search
// uses buildAwardSearchParams.
func buildCashSearchParams(in cashSearchInput) map[string]string {
	p := map[string]string{}
	if in.Origin != "" {
		p["O"] = in.Origin
	}
	if in.Destination != "" {
		p["D"] = in.Destination
	}
	if in.Depart != "" {
		p["OD"] = in.Depart
	}
	if in.Return != "" {
		p["DD"] = in.Return
	}
	if in.Adults != "" {
		p["A"] = in.Adults
	}
	if in.Children != "" {
		p["C"] = in.Children
	}
	if in.LapInfants != "" {
		p["L"] = in.LapInfants
	}
	if in.RoundTrip != "" {
		p["RT"] = in.RoundTrip
	}
	if in.Locale != "" {
		p["locale"] = in.Locale
	}
	return p
}

// valueCompareInputs is the set of values buildValueCompareEnvelope
// turns into the response envelope. Extracted so the envelope shape is
// unit-testable without firing live searches.
type valueCompareInputs struct {
	Origin      string
	Destination string
	Depart      string
	Return      string
	Cabin       string
	Adults      string
	Children    string
	MaxStops    int
	RoundTrip   bool
	Program     valuation.Program
	CashFare    lowestFare
	AwardFare   lowestFare
	Valuation   valuation.Result
}

// buildValueCompareEnvelope produces the structured response. Layout
// mirrors award-cheapest: a meta block carrying request + valuation
// context, plus a results block carrying cash, award, and the computed
// comparison. When either side is absent, the corresponding result
// block is nil and a meta.note carries the reason.
func buildValueCompareEnvelope(in valueCompareInputs) map[string]any {
	meta := map[string]any{
		"source":                  "live",
		"origin":                  in.Origin,
		"destination":             in.Destination,
		"depart":                  in.Depart,
		"return":                  in.Return,
		"cabin":                   in.Cabin,
		"adults":                  in.Adults,
		"children":                in.Children,
		"max_stops":               in.MaxStops,
		"round_trip":              in.RoundTrip,
		"program":                 string(in.Program),
		"cpp_baseline_cents":      in.Valuation.CPPCents,
		"cpp_baseline_source":     in.Valuation.Source,
		"cpp_baseline_fetched_at": in.Valuation.FetchedAt.UTC().Format(time.RFC3339),
		"tpg_url":                 valuation.TPGValuationsURL,
	}
	if in.Valuation.Warning != nil {
		meta["valuation_warning"] = in.Valuation.Warning.Error()
	}

	results := map[string]any{}
	var notes []string

	if in.CashFare.CashUSD > 0 {
		results["cash"] = map[string]any{
			"price_usd": in.CashFare.CashUSD,
			"carrier":   in.CashFare.Carrier,
			"cabin":     in.CashFare.Cabin,
			"stops":     in.CashFare.Stops,
		}
	} else {
		results["cash"] = nil
		notes = append(notes, "no cash itinerary found matching the cabin/max-stops filter")
	}

	if in.AwardFare.Miles != nil {
		results["award"] = map[string]any{
			"miles":     *in.AwardFare.Miles,
			"taxes_usd": in.AwardFare.CashUSD,
			"carrier":   in.AwardFare.Carrier,
			"cabin":     in.AwardFare.Cabin,
			"stops":     in.AwardFare.Stops,
		}
	} else {
		results["award"] = nil
		notes = append(notes, "no award inventory found matching the cabin/max-stops filter")
	}

	if in.CashFare.CashUSD > 0 && in.AwardFare.Miles != nil {
		comp := valuation.Compare(in.CashFare.CashUSD, *in.AwardFare.Miles, in.AwardFare.CashUSD, in.Valuation.CPPCents)
		results["comparison"] = map[string]any{
			"effective_cpp_cents": comp.EffectiveCPPCents,
			"baseline_cpp_cents":  comp.BaselineCPPCents,
			"multiple":            comp.Multiple,
			"tpg_valued_usd":      comp.TPGValuedUSD,
			"cash_saved_usd":      comp.CashSavedUSD,
		}
	} else {
		results["comparison"] = nil
	}

	if len(notes) > 0 {
		meta["notes"] = notes
	}

	return map[string]any{
		"meta":    meta,
		"results": results,
	}
}

// printValueCompareDryRunTo emits a structured preview without firing
// network calls. Used by --dry-run.
func printValueCompareDryRunTo(w interface {
	Write(p []byte) (n int, err error)
}, origin, dest, depart, ret, cabin string, program valuation.Program) error {
	preview := map[string]any{
		"dry_run":     true,
		"origin":      origin,
		"destination": dest,
		"depart":      depart,
		"return":      ret,
		"cabin":       cabin,
		"program":     string(program),
		"calls_to_make": []string{
			"GET /search/results/__data.json (cash)",
			"GET /search/results/__data.json (award)",
			"Lookup " + string(program) + " cents-per-point (cache→TPG→fallback)",
		},
	}
	out, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(out, '\n'))
	return err
}
