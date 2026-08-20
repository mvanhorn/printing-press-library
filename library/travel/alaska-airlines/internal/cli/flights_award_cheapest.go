// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-19: award-cheapest planner) — KILLER command added by
// /printing-press-amend. Solves "find me a flight to Japan using points,
// lowest price in August" by iterating every (depart, return) pair in a
// month across one or more destination airports in parallel and returning
// the cheapest round-trip by miles.
//
// Live sniff date: 2026-05-19. See flights_award_search.go for the underlying
// endpoint shape and .printing-press-patches.json entry "award-cheapest-planner".

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// destinationRegions are user-facing shorthands for sets of IATA codes.
// Only "japan" ships in v1 per the user spec; more can be added without
// breaking the existing surface.
var destinationRegions = map[string][]string{
	"japan": {"HND", "NRT", "KIX", "ITM", "NGO", "FUK", "CTS", "OKA"},
}

func newFlightsAwardCheapestCmd(flags *rootFlags) *cobra.Command {
	var flagOrigin string
	var flagDest string
	var flagDestRegion string
	var flagMonth string
	var flagDepartFrom string
	var flagDepartTo string
	var flagReturnFrom string
	var flagReturnTo string
	var flagMinNights int
	var flagMaxNights int
	var flagOneWay bool
	var flagCabin string
	var flagMaxStops int
	var flagAdults string
	var flagChildren string
	var flagTopN int
	var flagConcurrency int
	var flagSavePath string

	cmd := &cobra.Command{
		Use:   "award-cheapest",
		Short: "Find the cheapest round-trip award fare in miles across a month and one or more destinations.",
		Long: `Iterate the (depart, return) date matrix in a calendar month (or arbitrary window) across one or more destination airports in parallel, then return the cheapest round-trip in miles.

Example: find the cheapest round-trip to Japan in August using points:

  alaska-airlines-pp-cli flights award-cheapest \
    --origin SFO --destination-region japan --month 2026-08 \
    --cabin economy --max-stops 1 --json

The (origin, destination, depart, return) tuple space can be large — 8 Japan
airports x 31 depart days x 17 return options (5-21 nights) is up to ~4200
calls. The default --concurrency 4 + the existing AdaptiveLimiter keeps the
fan-out polite. For a typical 1-month round-trip search you should expect
30-90 seconds end-to-end.`,
		Example:     "  alaska-airlines-pp-cli flights award-cheapest --origin SFO --destination-region japan --month 2026-08 --json",
		Annotations: map[string]string{"pp:endpoint": "flights.award_cheapest", "pp:method": "GET", "pp:path": "/search/results/__data.json (fanned)", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagOrigin == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "origin")
			}

			destinations, err := resolveDestinations(flagDest, flagDestRegion)
			if err != nil {
				return err
			}
			if len(destinations) == 0 && !flags.dryRun {
				return fmt.Errorf("no destinations: pass --destination or --destination-region")
			}

			departWindow, returnWindow, err := resolveDateWindows(flagMonth, flagDepartFrom, flagDepartTo, flagReturnFrom, flagReturnTo, flagOneWay)
			if err != nil {
				return err
			}
			if flagMinNights < 0 || flagMaxNights < flagMinNights {
				return fmt.Errorf("invalid --min-nights/--max-nights: min=%d max=%d", flagMinNights, flagMaxNights)
			}

			pairs := enumerateDatePairs(departWindow, returnWindow, flagMinNights, flagMaxNights, flagOneWay)
			if len(pairs) == 0 && !flags.dryRun {
				return fmt.Errorf("no (depart, return) candidates derived from the inputs; widen the window or relax min/max nights")
			}

			// Cross-product against destinations.
			type job struct {
				dest   string
				depart string
				ret    string
				nights int
				oneWay bool
			}
			jobs := make([]job, 0, len(destinations)*len(pairs))
			for _, d := range destinations {
				for _, p := range pairs {
					jobs = append(jobs, job{dest: d, depart: p.depart, ret: p.ret, nights: p.nights, oneWay: flagOneWay})
				}
			}

			if flags.dryRun {
				return printAwardCheapestDryRun(cmd.OutOrStdout(), flagOrigin, destinations, departWindow, returnWindow, flagMinNights, flagMaxNights, flagOneWay, len(jobs))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Concurrency: semaphore-bounded errgroup. Errors from
			// individual jobs are NOT fatal — we collect them in
			// meta.failed_calls and continue, per the plan doc.
			if flagConcurrency < 1 {
				flagConcurrency = 1
			}
			sem := make(chan struct{}, flagConcurrency)
			var mu sync.Mutex
			results := make([]awardCheapestRow, 0, len(jobs))
			var failedCalls int
			var wg sync.WaitGroup
			start := time.Now()

			for _, j := range jobs {
				wg.Add(1)
				sem <- struct{}{}
				go func(j job) {
					defer wg.Done()
					defer func() { <-sem }()

					rt := "true"
					if j.oneWay {
						rt = "false"
					}
					params := buildAwardSearchParams(awardSearchInput{
						Origin:      flagOrigin,
						Destination: j.dest,
						Depart:      j.depart,
						Return:      j.ret,
						Adults:      flagAdults,
						Children:    flagChildren,
						LapInfants:  "0",
						RoundTrip:   rt,
						Cabin:       flagCabin,
						Locale:      "en-us",
					})

					data, _, err := resolveRead(cmd.Context(), c, flags, "flights", false, "/search/results/__data.json", params, nil)
					if err != nil {
						mu.Lock()
						failedCalls++
						mu.Unlock()
						return
					}
					low := extractLowestAwardPrice(data, flagCabin, flagMaxStops)
					if low.Miles == nil {
						// Could not extract a fare from this response —
						// either no inventory or unfamiliar shape. Don't
						// count it as a failed call (the request succeeded),
						// just don't include it in results.
						return
					}
					row := awardCheapestRow{
						Destination: j.dest,
						Depart:      j.depart,
						Return:      j.ret,
						Nights:      j.nights,
						Miles:       *low.Miles,
						CashUSD:     low.CashUSD,
						Carrier:     low.Carrier,
						Cabin:       low.Cabin,
						Stops:       low.Stops,
					}
					mu.Lock()
					results = append(results, row)
					mu.Unlock()
				}(j)
			}
			wg.Wait()
			duration := time.Since(start)

			sort.SliceStable(results, func(i, k int) bool {
				if results[i].Miles != results[k].Miles {
					return results[i].Miles < results[k].Miles
				}
				if results[i].CashUSD != results[k].CashUSD {
					return results[i].CashUSD < results[k].CashUSD
				}
				return results[i].Depart < results[k].Depart
			})

			top := results
			if flagTopN > 0 && len(top) > flagTopN {
				top = top[:flagTopN]
			}

			envelope := map[string]any{
				"meta": map[string]any{
					"source":               "live",
					"origin":               flagOrigin,
					"destinations":         destinations,
					"month":                flagMonth,
					"depart_from":          departWindow.from,
					"depart_to":            departWindow.to,
					"return_from":          returnWindow.from,
					"return_to":            returnWindow.to,
					"min_nights":           flagMinNights,
					"max_nights":           flagMaxNights,
					"one_way":              flagOneWay,
					"cabin":                flagCabin,
					"max_stops":            flagMaxStops,
					"adults":               flagAdults,
					"children":             flagChildren,
					"candidates_evaluated": len(jobs),
					"successful_results":   len(results),
					"failed_calls":         failedCalls,
					"top_n":                flagTopN,
					"duration_seconds":     math.Round(duration.Seconds()*10) / 10,
				},
				"results": top,
			}

			if flagSavePath != "" {
				fullEnvelope := map[string]any{
					"meta":        envelope["meta"],
					"all_results": results,
					"top_results": top,
				}
				if err := writeJSONFile(flagSavePath, fullEnvelope); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save full results to %s: %v\n", flagSavePath, err)
				}
			}

			out, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
		},
	}

	cmd.Flags().StringVar(&flagOrigin, "origin", "", "Origin IATA code (e.g. SFO)")
	cmd.Flags().StringVar(&flagDest, "destination", "", "One or more destination IATA codes, comma-separated (e.g. HND,NRT,KIX)")
	cmd.Flags().StringVar(&flagDestRegion, "destination-region", "", "Preset region expanding to a destination list (e.g. japan -> HND,NRT,KIX,ITM,NGO,FUK,CTS,OKA)")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Calendar month YYYY-MM (e.g. 2026-08). Sets the depart-window to the full month; return-window auto-derives from --min-nights/--max-nights.")
	cmd.Flags().StringVar(&flagDepartFrom, "depart-from", "", "Earliest depart date YYYY-MM-DD (overrides --month)")
	cmd.Flags().StringVar(&flagDepartTo, "depart-to", "", "Latest depart date YYYY-MM-DD (overrides --month)")
	cmd.Flags().StringVar(&flagReturnFrom, "return-from", "", "Earliest return date YYYY-MM-DD (overrides --month/--min-nights derivation)")
	cmd.Flags().StringVar(&flagReturnTo, "return-to", "", "Latest return date YYYY-MM-DD (overrides --month/--max-nights derivation)")
	cmd.Flags().IntVar(&flagMinNights, "min-nights", 5, "Minimum trip length in nights (round-trip only)")
	cmd.Flags().IntVar(&flagMaxNights, "max-nights", 21, "Maximum trip length in nights (round-trip only)")
	cmd.Flags().BoolVar(&flagOneWay, "one-way", false, "One-way search; skip the return-leg iteration")
	cmd.Flags().StringVar(&flagCabin, "cabin", "economy", "Preferred cabin (economy, premium, business, first); empty for any")
	cmd.Flags().IntVar(&flagMaxStops, "max-stops", -1, "Maximum number of stops per itinerary; -1 means unset")
	cmd.Flags().StringVar(&flagAdults, "adults", "1", "Adult passenger count")
	cmd.Flags().StringVar(&flagChildren, "children", "0", "Child passenger count")
	cmd.Flags().IntVar(&flagTopN, "top-n", 5, "Return only the N cheapest results")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "Number of parallel award-search calls (1-16 sensible)")
	cmd.Flags().StringVar(&flagSavePath, "save", "", "Optional file path to persist the full result set (not just top-N) as JSON")

	_ = context.Background // keep context import for future timeout plumbing
	return cmd
}

// awardCheapestRow is one row in the result set: a single (destination,
// depart, return) candidate with the lowest miles+cash combo extracted
// from the API response.
type awardCheapestRow struct {
	Destination string  `json:"destination"`
	Depart      string  `json:"depart"`
	Return      string  `json:"return,omitempty"`
	Nights      int     `json:"nights"`
	Miles       int     `json:"miles"`
	CashUSD     float64 `json:"cash_usd"`
	Carrier     string  `json:"carrier,omitempty"`
	Cabin       string  `json:"cabin,omitempty"`
	Stops       int     `json:"stops"`
}

// dateWindow is an inclusive [from, to] range of YYYY-MM-DD strings.
type dateWindow struct {
	from string
	to   string
}

// datePair is one candidate (depart, return) tuple.
type datePair struct {
	depart string
	ret    string
	nights int
}

// resolveDestinations converts --destination + --destination-region into a
// deduped, ordered list of IATA codes. Region wins when both are set —
// the user typically uses one or the other.
func resolveDestinations(destFlag, regionFlag string) ([]string, error) {
	if regionFlag != "" {
		codes, ok := destinationRegions[strings.ToLower(regionFlag)]
		if !ok {
			known := make([]string, 0, len(destinationRegions))
			for k := range destinationRegions {
				known = append(known, k)
			}
			sort.Strings(known)
			return nil, fmt.Errorf("unknown --destination-region %q (known: %s)", regionFlag, strings.Join(known, ", "))
		}
		return append([]string(nil), codes...), nil
	}
	if destFlag == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range strings.Split(destFlag, ",") {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out, nil
}

// resolveDateWindows turns the user's --month or explicit window flags into
// inclusive [from, to] windows for depart and return. For one-way the return
// window is empty.
func resolveDateWindows(month, dFrom, dTo, rFrom, rTo string, oneWay bool) (dateWindow, dateWindow, error) {
	var dep, ret dateWindow

	if month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return dep, ret, fmt.Errorf("invalid --month %q: expected YYYY-MM", month)
		}
		first := t
		last := first.AddDate(0, 1, -1)
		dep.from = first.Format("2006-01-02")
		dep.to = last.Format("2006-01-02")
		if !oneWay {
			// Default return window: same month boundaries; the
			// min/max-nights filter narrows the actual (depart, return)
			// pairs. Extending the return-window past the month is fine
			// for trips that wrap into next month — we extend to end-of-
			// next-month by default.
			retLast := last.AddDate(0, 1, 0)
			ret.from = first.Format("2006-01-02")
			ret.to = retLast.Format("2006-01-02")
		}
	}

	if dFrom != "" {
		dep.from = dFrom
	}
	if dTo != "" {
		dep.to = dTo
	}
	if rFrom != "" {
		ret.from = rFrom
	}
	if rTo != "" {
		ret.to = rTo
	}

	if dep.from == "" || dep.to == "" {
		return dep, ret, fmt.Errorf("depart window not set: pass --month or both --depart-from and --depart-to")
	}
	if _, err := time.Parse("2006-01-02", dep.from); err != nil {
		return dep, ret, fmt.Errorf("invalid --depart-from %q: %w", dep.from, err)
	}
	if _, err := time.Parse("2006-01-02", dep.to); err != nil {
		return dep, ret, fmt.Errorf("invalid --depart-to %q: %w", dep.to, err)
	}
	if !oneWay {
		if ret.from == "" || ret.to == "" {
			return dep, ret, fmt.Errorf("return window not set: pass --month or both --return-from and --return-to (or --one-way)")
		}
		if _, err := time.Parse("2006-01-02", ret.from); err != nil {
			return dep, ret, fmt.Errorf("invalid --return-from %q: %w", ret.from, err)
		}
		if _, err := time.Parse("2006-01-02", ret.to); err != nil {
			return dep, ret, fmt.Errorf("invalid --return-to %q: %w", ret.to, err)
		}
	}

	return dep, ret, nil
}

// enumerateDatePairs walks the depart window and (for round-trip) attaches
// every valid return date inside [depart+minNights, depart+maxNights] that
// also falls within the return window. One-way calls return a list of
// depart-only pairs (ret = "").
func enumerateDatePairs(depart, ret dateWindow, minNights, maxNights int, oneWay bool) []datePair {
	out := []datePair{}
	depFrom, err1 := time.Parse("2006-01-02", depart.from)
	depTo, err2 := time.Parse("2006-01-02", depart.to)
	if err1 != nil || err2 != nil {
		return out
	}
	if oneWay {
		for d := depFrom; !d.After(depTo); d = d.AddDate(0, 0, 1) {
			out = append(out, datePair{depart: d.Format("2006-01-02"), ret: "", nights: 0})
		}
		return out
	}
	retFrom, err3 := time.Parse("2006-01-02", ret.from)
	retTo, err4 := time.Parse("2006-01-02", ret.to)
	if err3 != nil || err4 != nil {
		return out
	}
	for d := depFrom; !d.After(depTo); d = d.AddDate(0, 0, 1) {
		for n := minNights; n <= maxNights; n++ {
			r := d.AddDate(0, 0, n)
			if r.Before(retFrom) || r.After(retTo) {
				continue
			}
			out = append(out, datePair{depart: d.Format("2006-01-02"), ret: r.Format("2006-01-02"), nights: n})
		}
	}
	return out
}

// lowestAwardPrice is a minimal extraction shape — the SvelteKit response
// is index-encoded so we walk it tolerantly and pick the lowest miles
// total we can find. Missing fields stay nil/empty so the result is still
// useful even if the response shape changes upstream.
type lowestAwardPrice struct {
	Miles   *int
	CashUSD float64
	Carrier string
	Cabin   string
	Stops   int
}

// extractLowestAwardPrice hydrates the SvelteKit __data.json response
// and returns the cheapest miles+taxes combo. Delegates to
// extractLowestFare (see flights_fare_extract.go) for the underlying
// hydration and walk; this wrapper preserves the legacy type so
// award-cheapest's call sites stay unchanged.
//
// When maxStops >= 0, itineraries with more stops are skipped. Returns
// lowestAwardPrice with Miles=nil when no usable miles total is found.
func extractLowestAwardPrice(data json.RawMessage, cabinFilter string, maxStops int) lowestAwardPrice {
	// award-cheapest ranks across hundreds of itineraries before any
	// valuation lookup runs, so cpp is not known here. Pass 0 to keep
	// the legacy "minimum miles" ranking; value-compare passes a real
	// cpp via extractLowestFare directly so its top pick reflects
	// total out-of-pocket cost rather than miles only.
	//
	// cabinFilter is enforced at extraction time so --cabin actually
	// constrains the cheapest pick. The API also gets the cabin as a
	// best-effort SpecFare query param, but that filter is not always
	// honored — without this enforcement, a user asking for economy
	// can receive a business-class row as the cheapest result.
	fare := extractLowestFare(data, fareModeAward, cabinFilter, maxStops, 0)
	if fare.Miles == nil {
		return lowestAwardPrice{}
	}
	return lowestAwardPrice{
		Miles:   fare.Miles,
		CashUSD: fare.CashUSD,
		Carrier: fare.Carrier,
		Cabin:   fare.Cabin,
		Stops:   fare.Stops,
	}
}

// Note: the legacy walkAwardJSON / readIntField / readFloatField /
// readStringField helpers were removed in the 2026-05-20 value-compare
// amend. They emitted on JSON keys (milesAmount, cashAmount) that the
// live SvelteKit response does not produce. extractLowestAwardPrice now
// delegates to extractLowestFare in flights_fare_extract.go, which
// walks the actual rows[].solutions.<CABIN> shape after hydrating the
// SvelteKit positional encoding.

// printAwardCheapestDryRun emits a structured preview of the work that would
// be done. No network calls.
func printAwardCheapestDryRun(w io.Writer, origin string, destinations []string, dep, ret dateWindow, minNights, maxNights int, oneWay bool, jobs int) error {
	preview := map[string]any{
		"dry_run":      true,
		"origin":       origin,
		"destinations": destinations,
		"depart_from":  dep.from,
		"depart_to":    dep.to,
		"return_from":  ret.from,
		"return_to":    ret.to,
		"min_nights":   minNights,
		"max_nights":   maxNights,
		"one_way":      oneWay,
		"jobs_to_run":  jobs,
	}
	out, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}

// writeJSONFile atomically writes obj as pretty-printed JSON to path.
// Best-effort temp-rename pattern. Used by --save.
func writeJSONFile(path string, obj any) error {
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
