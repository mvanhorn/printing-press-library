// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cheapest fully-insured option across every source, including
// the standalone-excess-cover money-saver.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// Standalone third-party excess insurance (e.g. iCarHire, RentalCover) is priced
// as a fixed policy base plus a per-day rate — NOT a flat per-day — so a pure
// per-day estimate badly under-quotes short rentals, where the base dominates.
// These defaults (EUR) are modeled on iCarHire "Excess Europe Daily", single
// trip, Spain, driver age 35, captured 2026-07-23: £24.01 for a 3-night rental
// and £33.15 for a 7-night rental (iCarHire's own base £14.87 + £2.28/policy-day,
// where policy-days = nights + 1). Folding the inclusive extra day into the base
// and converting at ≈€1.16/£ gives base ≈ €20, per-night ≈ €2.65 — which
// reproduces both quotes within cents. Standalone cover is still typically far
// cheaper than a rental company's own full-cover upsell. Both are tunable.
const (
	DefaultExcessCoverBase   = 20.0 // EUR fixed policy base
	DefaultExcessCoverPerDay = 2.65 // EUR per rental night
)

// excessCoverEstimate returns the estimated standalone excess-insurance cost
// (EUR) for a rental of `days` nights: the fixed base plus the per-night rate.
func excessCoverEstimate(days int, base, perDay float64) float64 {
	if days < 0 {
		days = 0
	}
	return base + float64(days)*perDay
}

type bestOption struct {
	Supplier         string  `json:"supplier"`
	Car              string  `json:"car"`
	Source           string  `json:"source"`   // doyouspain | rentalcars | delpaso | centauro | drivalia
	Strategy         string  `json:"strategy"` // "direct" | "aggregator+standalone-cover" | "aggregator-zero-excess"
	BaseTotal        float64 `json:"base_total"`
	ExcessCover      float64 `json:"excess_cover,omitempty"` // standalone cover added, when used
	FullyInsuredTotal float64 `json:"fully_insured_total"`
	Currency         string  `json:"currency"`
	ExcessKnown      bool    `json:"excess_known"`
	// Estimated is true when FullyInsuredTotal includes an assumed standalone
	// excess-cover figure rather than a fully-quoted price. The base rate is
	// real; the cover is an estimate (--excess-cover-per-day). Direct and
	// aggregator-zero-excess options are fully quoted, so this is false.
	Estimated bool `json:"estimated"`
	// MinAge is the supplier's stated minimum driver age for this car (0 =
	// unknown). Used to flag an option a young driver cannot actually rent.
	MinAge int `json:"min_age,omitempty"`
}

func newBestCmd(flags *rootFlags) *cobra.Command {
	var pickupTime, dropoffTime string
	var age, limit int
	var coverPerDay, coverBase float64
	var directOnly bool
	var realOnly bool
	var classFilter string
	cmd := &cobra.Command{
		Use:   "best [location-code] <pickup> <dropoff>",
		Short: "Cheapest way to rent fully insured (zero excess) across every source",
		Long: "Rank the cheapest fully-insured (zero-excess) options for Málaga across all sources:\n" +
			"  • direct companies (Delpaso, Centauro, Drivalia, Clickrent, Goldcar, plus CICAR and\n" +
			"    Autoreisen in the Canaries) at their own zero-excess price, and\n" +
			"  • aggregator offers (DoYouSpain, Rentalcars) priced as base rate + a standalone\n" +
			"    third-party excess-insurance estimate (usually far cheaper than the rental firm's\n" +
			"    own full cover — the AutoSlash-style money-saver).\n\n" +
			"Every price shown is a genuine no-excess total, so they compare apples to apples.\n" +
			"Location defaults to Málaga Airport; dates are dd/mm/yyyy or yyyy-mm-dd.",
		Example:     "  rentalcarspain-pp-cli best 20/07/2026 03/08/2026 --limit 8 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location, pickup, dropoff, ok := resolveSearchArgs(args)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("best needs <pickup> <dropoff> (Málaga default) or <location-code> <pickup> <dropoff>"))
			}
			if coverPerDay < 0 || coverBase < 0 {
				return usageErr(fmt.Errorf("--excess-cover-per-day and --excess-cover-base must not be negative"))
			}
			days := rentalDaysBetween(pickup, dropoff)
			loc := resolveLocationInput(location)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := time.Now()

			var opts []bestOption
			var aggErrs map[string]error
			var mu sync.Mutex
			var wg sync.WaitGroup

			// Direct companies — already zero-excess.
			for _, co := range enabledDirectCompanies(flags) {
				wg.Add(1)
				go func(co directCompany) {
					defer wg.Done()
					offers, err := co.quote(ctx, flags, loc, pickup, dropoff, pickupTime, dropoffTime, age, now)
					if err != nil {
						return
					}
					mu.Lock()
					for _, o := range offers {
						if !classFilterMatch(o, classFilter) {
							continue
						}
						opts = append(opts, bestOption{
							Supplier: o.Supplier, Car: o.Car, Source: o.Source, Strategy: "direct",
							BaseTotal: o.Total, FullyInsuredTotal: o.Total, Currency: o.Currency, ExcessKnown: true,
							MinAge: o.MinAge,
						})
					}
					mu.Unlock()
				}(co)
			}

			// Aggregators — base rate + standalone excess cover (unless directOnly).
			if !directOnly {
				wg.Add(1)
				go func() {
					defer wg.Done()
					offers, errs := fetchOffers(ctx, flags, "all", location, pickup, dropoff, pickupTime, dropoffTime, age)
					cover := excessCoverEstimate(days, coverBase, coverPerDay)
					mu.Lock()
					aggErrs = errs
					for _, o := range offers {
						if !classFilterMatch(o, classFilter) {
							continue
						}
						opt := bestOption{
							Supplier: o.Supplier, Car: o.Car, Source: o.Source, Currency: o.Currency, BaseTotal: o.Total, ExcessKnown: o.ExcessKnown,
						}
						if o.ExcessKnown && o.Excess == 0 {
							opt.Strategy = "aggregator-zero-excess"
							opt.FullyInsuredTotal = o.Total
						} else {
							opt.Strategy = "aggregator+standalone-cover"
							opt.ExcessCover = cover
							opt.FullyInsuredTotal = o.Total + cover
						}
						opt.Estimated = strategyIsEstimated(opt.Strategy)
						if realOnly && opt.Estimated {
							continue
						}
						opts = append(opts, opt)
					}
					mu.Unlock()
				}()
			}
			wg.Wait()

			// Surface aggregator health so a thin result set (e.g. DoYouSpain
			// failing its redirect-token step) is visible, not silent.
			for src, e := range aggErrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: aggregator %s unavailable: %s\n", src, truncate(e.Error(), 90))
			}

			if len(opts) == 0 {
				if err := sourceErrorsError(aggErrs); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("no fully-insured options found for these dates"))
			}
			sort.SliceStable(opts, func(i, j int) bool { return opts[i].FullyInsuredTotal < opts[j].FullyInsuredTotal })
			if limit > 0 && len(opts) > limit {
				opts = opts[:limit]
			}

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{
					"location": location, "pickup": pickup, "dropoff": dropoff, "days": days,
					"excess_cover_base": coverBase, "excess_cover_per_day": coverPerDay,
					"count": len(opts), "options": opts,
				})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			mf, mfErr := newMoneyFmt(ctx, flags)
			if mfErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", mfErr)
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "#\tSUPPLIER\tCAR\tFULLY INSURED\tHOW")
			anyEstimated := false
			anyIneligible := false
			for i, o := range opts {
				total := mf.format(o.FullyInsuredTotal)
				if o.Estimated {
					total += " ~est"
					anyEstimated = true
				}
				car := truncate(o.Car, 22)
				if age > 0 && o.MinAge > 0 && age < o.MinAge {
					car += fmt.Sprintf(" [min age %d]", o.MinAge)
					anyIneligible = true
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", i+1, o.Supplier, car, total, bestHow(o))
			}
			tw.Flush()
			if mf.active() {
				fmt.Fprintf(w, "\n%s amounts are indicative (ECB rate %s); you are billed in EUR.\n", mf.currency, mf.rates.Date)
			}
			fmt.Fprintf(w, "\nFully-insured = zero excess. 'direct' is the company's own no-excess price; 'base+cover'\n")
			fmt.Fprintf(w, "adds a standalone excess-insurance estimate of %.2f base + %.2f/day over %d days (%.2f), modeled on\n", coverBase, coverPerDay, days, excessCoverEstimate(days, coverBase, coverPerDay))
			fmt.Fprintf(w, "iCarHire single-trip quotes. Tune with --excess-cover-base / --excess-cover-per-day, or --direct-only.\n")
			if anyEstimated {
				fmt.Fprintf(w, "~est = total includes the assumed cover figure above, not a quoted price (the base rate is real).\n")
				fmt.Fprintf(w, "       Use --real-only to rank on fully-quoted zero-excess prices alone.\n")
			}
			if realOnly {
				fmt.Fprintf(w, "Showing fully-quoted prices only (--real-only): aggregator base+cover estimates are hidden.\n")
			}
			if anyIneligible {
				fmt.Fprintf(w, "[min age N] = this supplier will not rent this car to a driver aged %d; the price is shown for comparison only.\n", age)
			}
			if note := youngDriverNotice(age); note != "" {
				fmt.Fprintf(w, "%s\n", note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "10:00", "Pickup time HH:MM")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "10:00", "Dropoff time HH:MM")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max options to return")
	cmd.Flags().Float64Var(&coverBase, "excess-cover-base", DefaultExcessCoverBase, "Standalone excess-insurance fixed policy base (EUR) added to aggregator base rates")
	cmd.Flags().Float64Var(&coverPerDay, "excess-cover-per-day", DefaultExcessCoverPerDay, "Standalone excess-insurance per-day estimate (EUR) added to aggregator base rates")
	cmd.Flags().BoolVar(&directOnly, "direct-only", false, "Only compare direct-company full-cover prices (skip aggregators)")
	cmd.Flags().BoolVar(&realOnly, "real-only", false, "Only show fully-quoted zero-excess prices; hide rows whose total includes an estimated excess-cover figure")
	cmd.Flags().StringVar(&classFilter, "class", "", "Only rank cars matching these brand/type/size substrings (comma-separated, e.g. bmw,cabrio,suv)")
	return cmd
}

// strategyIsEstimated reports whether a pricing strategy's fully-insured total
// leans on the assumed excess-cover figure rather than a fully-quoted price.
// Only aggregator base rates that lack a stated zero excess are estimated;
// direct quotes and aggregator zero-excess offers are fully quoted.
func strategyIsEstimated(strategy string) bool {
	return strategy == "aggregator+standalone-cover"
}

func bestHow(o bestOption) string {
	switch o.Strategy {
	case "direct":
		return fmt.Sprintf("direct (%s, zero excess)", o.Source)
	case "aggregator-zero-excess":
		return fmt.Sprintf("%s (zero excess)", o.Source)
	default:
		return fmt.Sprintf("%s base %.0f + cover %.0f", o.Source, o.BaseTotal, o.ExcessCover)
	}
}

// rentalDaysBetween returns the whole-day rental length, minimum 1.
func rentalDaysBetween(pickup, dropoff string) int {
	p, err1 := parseDMY(pickup)
	d, err2 := parseDMY(dropoff)
	if err1 != nil || err2 != nil {
		return 1
	}
	days := int(d.Sub(p).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}
